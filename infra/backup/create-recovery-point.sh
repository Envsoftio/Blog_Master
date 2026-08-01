#!/usr/bin/env bash
set -Eeuo pipefail

KIND="${1:-daily}"
case "$KIND" in
  daily|monthly|pre-release) ;;
  *) printf '[backup] ERROR: kind must be daily, monthly or pre-release\n' >&2; exit 2 ;;
esac

log() { printf '[backup] %s\n' "$*"; }
fail() { printf '[backup] ERROR: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }
require_value() { [ -n "${!1:-}" ] || fail "$1 is required"; }

backup_env_file="${SEOBLOG_BACKUP_ENV_FILE:-$(dirname "${SEOBLOG_DB_PATH:-/srv/seoblog/shared/seoblog.db}")/backup.env}"
if [ -r "$backup_env_file" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$backup_env_file"
  set +a
fi

for command_name in aws date flock litestream node openssl sha256sum sqlite3; do
  require_command "$command_name"
done
for variable_name in \
  SEOBLOG_DB_PATH SEOBLOG_LITESTREAM_CONFIG SEOBLOG_LITESTREAM_SOCKET \
  SEOBLOG_BACKUP_ENDPOINT SEOBLOG_BACKUP_REGION SEOBLOG_BACKUP_BUCKET \
  SEOBLOG_BACKUP_SNAPSHOT_PREFIX SEOBLOG_BACKUP_EVIDENCE_DIR \
  SEOBLOG_BACKUP_SNAPSHOT_KEY_ID SEOBLOG_BACKUP_SNAPSHOT_APPLICATION_KEY \
  SEOBLOG_BACKUP_RESTORE_KEY_ID SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY; do
  require_value "$variable_name"
done

[[ "$SEOBLOG_BACKUP_ENDPOINT" == https://* ]] || fail "SEOBLOG_BACKUP_ENDPOINT must use HTTPS"
[[ "$SEOBLOG_BACKUP_BUCKET" =~ ^[A-Za-z0-9][A-Za-z0-9.-]{4,61}[A-Za-z0-9]$ ]] || fail "invalid backup bucket name"
[ -s "$SEOBLOG_DB_PATH" ] || fail "SQLite database is missing or empty: $SEOBLOG_DB_PATH"
[ -r "$SEOBLOG_LITESTREAM_CONFIG" ] || fail "Litestream config is not readable: $SEOBLOG_LITESTREAM_CONFIG"

case "$KIND" in
  daily) retention_days="${SEOBLOG_BACKUP_DAILY_RETENTION_DAYS:-31}" ;;
  monthly) retention_days="${SEOBLOG_BACKUP_MONTHLY_RETENTION_DAYS:-}" ;;
  pre-release) retention_days="${SEOBLOG_BACKUP_PRE_RELEASE_RETENTION_DAYS:-31}" ;;
esac
[[ "$retention_days" =~ ^[0-9]+$ ]] && [ "$retention_days" -ge 30 ] || fail "snapshot retention must be at least 30 days"

lock_mode="${SEOBLOG_BACKUP_OBJECT_LOCK_MODE:-GOVERNANCE}"
case "$lock_mode" in GOVERNANCE|COMPLIANCE) ;; *) fail "Object Lock mode must be GOVERNANCE or COMPLIANCE" ;; esac

mkdir -p "$SEOBLOG_BACKUP_EVIDENCE_DIR"
chmod 700 "$SEOBLOG_BACKUP_EVIDENCE_DIR"
exec 9>"$SEOBLOG_BACKUP_EVIDENCE_DIR/recovery-point.lock"
flock -n 9 || fail "another recovery-point job is running"

temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/seoblog-backup.XXXXXX")"
cleanup() { rm -rf -- "$temp_dir"; }
trap cleanup EXIT

restored_db="$temp_dir/restored.db"
downloaded_db="$temp_dir/downloaded.db"
sync_json="$temp_dir/sync.json"

log "forcing the continuous replica through the latest committed transaction"
litestream sync -socket "$SEOBLOG_LITESTREAM_SOCKET" -wait -timeout 120 -json "$SEOBLOG_DB_PATH" >"$sync_json"

log "restoring the continuous replica into an isolated database"
AWS_ACCESS_KEY_ID="$SEOBLOG_BACKUP_RESTORE_KEY_ID" \
AWS_SECRET_ACCESS_KEY="$SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY" \
  litestream restore -config "$SEOBLOG_LITESTREAM_CONFIG" -o "$restored_db" "$SEOBLOG_DB_PATH"

check_database() {
  local database="$1" quick foreign_keys
  quick="$(sqlite3 -batch -bail "$database" 'PRAGMA quick_check;')"
  [ "$quick" = "ok" ] || fail "PRAGMA quick_check failed for $database: $quick"
  foreign_keys="$(sqlite3 -batch -bail "$database" 'PRAGMA foreign_key_check;')"
  [ -z "$foreign_keys" ] || fail "foreign-key violations found in $database"
}
check_database "$restored_db"

sha256="$(sha256sum "$restored_db" | awk '{print $1}')"
content_md5="$(openssl dgst -md5 -binary "$restored_db" | base64 -w 0)"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
retain_until="$(date -u -d "+${retention_days} days" +%Y-%m-%dT%H:%M:%SZ)"
release_id="${RELEASE_ID:-none}"
safe_release="$(printf '%s' "$release_id" | tr -cs 'A-Za-z0-9._-' '-')"
object_key="${SEOBLOG_BACKUP_SNAPSHOT_PREFIX%/}/${KIND}/seoblog-${timestamp}-${safe_release}-${sha256:0:12}.db"

aws_snapshot=(env AWS_ACCESS_KEY_ID="$SEOBLOG_BACKUP_SNAPSHOT_KEY_ID" AWS_SECRET_ACCESS_KEY="$SEOBLOG_BACKUP_SNAPSHOT_APPLICATION_KEY" aws --endpoint-url "$SEOBLOG_BACKUP_ENDPOINT" --region "$SEOBLOG_BACKUP_REGION")
aws_restore=(env AWS_ACCESS_KEY_ID="$SEOBLOG_BACKUP_RESTORE_KEY_ID" AWS_SECRET_ACCESS_KEY="$SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY" aws --endpoint-url "$SEOBLOG_BACKUP_ENDPOINT" --region "$SEOBLOG_BACKUP_REGION")

log "uploading locked ${KIND} snapshot s3://${SEOBLOG_BACKUP_BUCKET}/${object_key}"
"${aws_snapshot[@]}" s3api put-object \
  --bucket "$SEOBLOG_BACKUP_BUCKET" \
  --key "$object_key" \
  --body "$restored_db" \
  --content-md5 "$content_md5" \
  --server-side-encryption AES256 \
  --object-lock-mode "$lock_mode" \
  --object-lock-retain-until-date "$retain_until" \
  --metadata "sha256=${sha256},kind=${KIND},release-id=${safe_release}" >/dev/null

log "downloading and validating the newly locked snapshot"
"${aws_restore[@]}" s3api get-object --bucket "$SEOBLOG_BACKUP_BUCKET" --key "$object_key" "$downloaded_db" >/dev/null
downloaded_sha256="$(sha256sum "$downloaded_db" | awk '{print $1}')"
[ "$downloaded_sha256" = "$sha256" ] || fail "downloaded snapshot checksum does not match"
check_database "$downloaded_db"
head_json="$("${aws_restore[@]}" s3api head-object --bucket "$SEOBLOG_BACKUP_BUCKET" --key "$object_key")"
retention_json="$("${aws_restore[@]}" s3api get-object-retention --bucket "$SEOBLOG_BACKUP_BUCKET" --key "$object_key")"
node - "$head_json" "$retention_json" "$sha256" "$lock_mode" "$retain_until" <<'NODE'
const [headJSON, retentionJSON, sha256, lockMode, retainUntil] = process.argv.slice(2)
const head = JSON.parse(headJSON)
const retention = JSON.parse(retentionJSON).Retention || {}
if (head.ServerSideEncryption !== 'AES256') throw new Error('snapshot is missing SSE-B2/AES256 evidence')
if (!head.Metadata || head.Metadata.sha256 !== sha256) throw new Error('snapshot metadata checksum does not match')
if (retention.Mode !== lockMode) throw new Error('snapshot Object Lock mode does not match')
if (Date.parse(retention.RetainUntilDate) !== Date.parse(retainUntil)) throw new Error('snapshot Object Lock date does not match')
NODE

evidence_file="$SEOBLOG_BACKUP_EVIDENCE_DIR/recovery-points.jsonl"
node - "$evidence_file" "$KIND" "$object_key" "$sha256" "$retain_until" "$lock_mode" "$release_id" "$sync_json" <<'NODE'
const fs = require('node:fs')
const [file, kind, objectKey, sha256, retainUntil, lockMode, releaseId, syncFile] = process.argv.slice(2)
const sync = JSON.parse(fs.readFileSync(syncFile, 'utf8'))
const record = {
  verifiedAt: new Date().toISOString(), kind, objectKey, sha256,
  retainUntil, lockMode, releaseId, sync,
  checks: { downloadedChecksum: true, quickCheck: 'ok', foreignKeyCheck: 'ok' }
}
fs.appendFileSync(file, `${JSON.stringify(record)}\n`, { mode: 0o600 })
NODE
chmod 600 "$evidence_file"
log "verified recovery point: s3://${SEOBLOG_BACKUP_BUCKET}/${object_key}"
