#!/usr/bin/env bash
set -Eeuo pipefail

SNAPSHOT_KEY=""
TARGET="${SEOBLOG_DB_PATH:-}"
AUTHORIZED_BY=""
CHANGE_ID=""
REPLACE="0"

usage() {
  printf '%s\n' 'Usage: restore-primary.sh --snapshot-key KEY --target PATH --authorized-by NAME --change-id ID [--replace]'
}
fail() { printf '[restore] ERROR: %s\n' "$*" >&2; exit 1; }
require_value() { [ -n "${!1:-}" ] || fail "$1 is required"; }

backup_env_file="${SEOBLOG_BACKUP_ENV_FILE:-$(dirname "${SEOBLOG_DB_PATH:-/srv/seoblog/shared/seoblog.db}")/backup.env}"
if [ -r "$backup_env_file" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$backup_env_file"
  set +a
fi

while [ "$#" -gt 0 ]; do
  case "$1" in
    --snapshot-key) SNAPSHOT_KEY="${2:-}"; shift 2 ;;
    --target) TARGET="${2:-}"; shift 2 ;;
    --authorized-by) AUTHORIZED_BY="${2:-}"; shift 2 ;;
    --change-id) CHANGE_ID="${2:-}"; shift 2 ;;
    --replace) REPLACE="1"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; fail "unknown argument: $1" ;;
  esac
done

[ -n "$SNAPSHOT_KEY" ] || fail "select --snapshot-key"
for variable_name in TARGET AUTHORIZED_BY CHANGE_ID SEOBLOG_BACKUP_ENDPOINT SEOBLOG_BACKUP_REGION \
  SEOBLOG_BACKUP_BUCKET SEOBLOG_BACKUP_RESTORE_KEY_ID SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY \
  SEOBLOG_BACKUP_EVIDENCE_DIR SEOBLOG_DB_PATH; do require_value "$variable_name"; done
[ "${SEOBLOG_RESTORE_SERVICES_STOPPED:-false}" = "true" ] || fail "set SEOBLOG_RESTORE_SERVICES_STOPPED=true only after API and worker are stopped"
[[ "$SEOBLOG_BACKUP_ENDPOINT" == https://* ]] || fail "backup endpoint must use HTTPS"

for command_name in aws flock node sqlite3; do command -v "$command_name" >/dev/null 2>&1 || fail "$command_name is required"; done

target_dir="$(dirname "$TARGET")"
mkdir -p "$target_dir" "$SEOBLOG_BACKUP_EVIDENCE_DIR"
chmod 700 "$SEOBLOG_BACKUP_EVIDENCE_DIR"
target_dir="$(cd "$target_dir" && pwd -P)"
TARGET="$target_dir/$(basename "$TARGET")"
case "$TARGET" in /|/etc/*|/usr/*|/bin/*|/sbin/*) fail "unsafe restore target: $TARGET" ;; esac
[ "${SEOBLOG_RESTORE_CONFIRM:-}" = "$TARGET" ] || fail "SEOBLOG_RESTORE_CONFIRM must exactly equal the resolved target path"

exec 9>"$SEOBLOG_BACKUP_EVIDENCE_DIR/primary-restore.lock"
flock -n 9 || fail "another primary restore is running"
temp_db="$(mktemp "$target_dir/.seoblog-restore.XXXXXX")"
rm -f -- "$temp_db"
cleanup() { rm -f -- "$temp_db"; }
trap cleanup EXIT

export AWS_ACCESS_KEY_ID="$SEOBLOG_BACKUP_RESTORE_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY"
[[ "$SNAPSHOT_KEY" != /* && "$SNAPSHOT_KEY" != *..* ]] || fail "unsafe snapshot key"
aws --endpoint-url "$SEOBLOG_BACKUP_ENDPOINT" --region "$SEOBLOG_BACKUP_REGION" \
  s3api get-object --bucket "$SEOBLOG_BACKUP_BUCKET" --key "$SNAPSHOT_KEY" "$temp_db" >/dev/null
source_description="snapshot:${SNAPSHOT_KEY}"

[ -s "$temp_db" ] || fail "restore source produced an empty database"
quick="$(sqlite3 -batch -bail "$temp_db" 'PRAGMA quick_check;')"
[ "$quick" = "ok" ] || fail "restored database failed PRAGMA quick_check: $quick"
foreign_keys="$(sqlite3 -batch -bail "$temp_db" 'PRAGMA foreign_key_check;')"
[ -z "$foreign_keys" ] || fail "restored database has foreign-key violations"

existing_primary="0"
for existing in "$TARGET" "$TARGET-wal" "$TARGET-shm"; do
  [ ! -e "$existing" ] || existing_primary="1"
done
if [ "$existing_primary" = "1" ] && [ "$REPLACE" != "1" ]; then
  fail "target database or WAL sidecar exists; rerun with --replace after reviewing the validated restore"
fi

replaced_path=""
if [ "$existing_primary" = "1" ]; then
  recovery_dir="$target_dir/pre-restore-$(date -u +%Y%m%dT%H%M%SZ)"
  mkdir -m 700 "$recovery_dir"
  for existing in "$TARGET" "$TARGET-wal" "$TARGET-shm"; do
    [ ! -e "$existing" ] || mv -- "$existing" "$recovery_dir/"
  done
  replaced_path="$recovery_dir"
fi
chmod 600 "$temp_db"
mv -- "$temp_db" "$TARGET"

evidence_file="$SEOBLOG_BACKUP_EVIDENCE_DIR/restores.jsonl"
node - "$evidence_file" "$source_description" "$TARGET" "$AUTHORIZED_BY" "$CHANGE_ID" "$replaced_path" <<'NODE'
const fs = require('node:fs')
const [file, source, target, authorizedBy, changeId, replacedPath] = process.argv.slice(2)
const record = { restoredAt: new Date().toISOString(), source, target, authorizedBy, changeId,
  replacedPath: replacedPath || null, checks: { quickCheck: 'ok', foreignKeyCheck: 'ok' } }
fs.appendFileSync(file, `${JSON.stringify(record)}\n`, { mode: 0o600 })
NODE
chmod 600 "$evidence_file"
printf '[restore] restored %s from %s; services remain stopped pending operator validation\n' "$TARGET" "$source_description"
