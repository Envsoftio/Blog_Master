#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"

fail() { printf '[backup-test] ERROR: %s\n' "$*" >&2; exit 1; }

for script in \
  "$repo_root/infra/backup/create-recovery-point.sh" \
  "$repo_root/infra/backup/restore-primary.sh" \
  "$repo_root/infra/deploy/deploy-release.sh"; do
  bash -n "$script"
done

if "$repo_root/infra/backup/create-recovery-point.sh" unsupported > /dev/null 2>&1; then
  fail "recovery-point script accepted an unsupported kind"
fi

grep -q '^retention:$' "$repo_root/infra/backup/litestream.yml" || fail "remote deletion policy is missing"
grep -q '^  enabled: false$' "$repo_root/infra/backup/litestream.yml" || fail "continuous-chain remote deletion must be disabled"
grep -q '^socket:$' "$repo_root/infra/backup/litestream.yml" || fail "Litestream control socket is required"
grep -q 'object-lock-retain-until-date' "$repo_root/infra/backup/create-recovery-point.sh" || fail "snapshot Object Lock is required"
grep -q "get-object.*downloaded_db" "$repo_root/infra/backup/create-recovery-point.sh" || fail "new snapshots must be downloaded for verification"
grep -q 'PRAGMA quick_check' "$repo_root/infra/backup/create-recovery-point.sh" || fail "isolated SQLite verification is required"
grep -q 'SEOBLOG_DEPLOY_REQUIRE_BACKUP=true' "$repo_root/infra/deploy/deploy-release.sh" || fail "production backup gate must default to enabled"

for unit in "$repo_root"/infra/systemd/*; do
  if grep -Eq '^ExecStart=.*pm2' "$unit"; then
    fail "infrastructure services must not be launched through PM2: $unit"
  fi
done

test_dir="$(mktemp -d "${TMPDIR:-/tmp}/seoblog-backup-test.XXXXXX")"
test_dir="$(cd "$test_dir" && pwd -P)"
cleanup() { rm -rf -- "$test_dir"; }
trap cleanup EXIT
mkdir -p "$test_dir/bin" "$test_dir/evidence"
sqlite3 "$test_dir/source.db" 'PRAGMA foreign_keys=ON; CREATE TABLE verification (id INTEGER PRIMARY KEY); INSERT INTO verification VALUES (1);'

cat > "$test_dir/bin/aws" <<'FAKEAWS'
#!/usr/bin/env bash
set -euo pipefail
action=""
body=""
destination=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "--body" ]; then body="$argument"; fi
  case "$argument" in put-object|get-object|head-object|get-object-retention) action="$argument" ;; esac
  previous="$argument"
done
case "$action" in
  put-object) cp "$body" "$SEOBLOG_TEST_REMOTE"; printf '{}\n' ;;
  get-object) destination="${@: -1}"; cp "$SEOBLOG_TEST_REMOTE" "$destination"; printf '{}\n' ;;
  head-object) printf '{"ServerSideEncryption":"AES256","Metadata":{"sha256":"%s"}}\n' "$(shasum -a 256 "$SEOBLOG_TEST_REMOTE" | awk '{print $1}')" ;;
  get-object-retention) printf '{"Retention":{"Mode":"GOVERNANCE","RetainUntilDate":"2099-01-01T00:00:00Z"}}\n' ;;
  *) exit 2 ;;
esac
FAKEAWS
cat > "$test_dir/bin/flock" <<'FAKEFLOCK'
#!/usr/bin/env bash
exit 0
FAKEFLOCK
cat > "$test_dir/bin/litestream" <<'FAKELITESTREAM'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  sync) printf '{"db_path":"%s","txid":"1","replica_txid":"1","duration_ms":1}\n' "$SEOBLOG_DB_PATH" ;;
  restore)
    output=""
    previous=""
    for argument in "$@"; do
      if [ "$previous" = "-o" ]; then output="$argument"; fi
      previous="$argument"
    done
    cp "$SEOBLOG_TEST_SNAPSHOT" "$output"
    ;;
esac
FAKELITESTREAM
cat > "$test_dir/bin/date" <<'FAKEDATE'
#!/usr/bin/env bash
set -euo pipefail
if [[ " $* " == *" -d "* ]]; then
  printf '2099-01-01T00:00:00Z\n'
else
  /bin/date "$@"
fi
FAKEDATE
cat > "$test_dir/bin/sha256sum" <<'FAKESHA'
#!/usr/bin/env bash
set -euo pipefail
shasum -a 256 "$1"
FAKESHA
chmod +x "$test_dir/bin/aws" "$test_dir/bin/date" "$test_dir/bin/flock" "$test_dir/bin/litestream" "$test_dir/bin/sha256sum"

PATH="$test_dir/bin:$PATH" \
SEOBLOG_DB_PATH="$test_dir/source.db" \
SEOBLOG_LITESTREAM_CONFIG="$repo_root/infra/backup/litestream.yml" \
SEOBLOG_LITESTREAM_SOCKET="$test_dir/litestream.sock" \
SEOBLOG_BACKUP_ENDPOINT="https://s3.example.invalid" \
SEOBLOG_BACKUP_REGION="test-region" \
SEOBLOG_BACKUP_BUCKET="valid-test-bucket" \
SEOBLOG_BACKUP_SNAPSHOT_PREFIX="snapshots/test" \
SEOBLOG_BACKUP_EVIDENCE_DIR="$test_dir/evidence" \
SEOBLOG_BACKUP_SNAPSHOT_KEY_ID="snapshot-writer" \
SEOBLOG_BACKUP_SNAPSHOT_APPLICATION_KEY="not-a-secret" \
SEOBLOG_BACKUP_RESTORE_KEY_ID="restore-reader" \
SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY="not-a-secret" \
SEOBLOG_TEST_SNAPSHOT="$test_dir/source.db" \
SEOBLOG_TEST_REMOTE="$test_dir/remote.db" \
  "$repo_root/infra/backup/create-recovery-point.sh" daily >/dev/null

[ -s "$test_dir/remote.db" ] || fail "verified recovery point was not uploaded"
grep -q 'downloadedChecksum' "$test_dir/evidence/recovery-points.jsonl" || fail "recovery-point evidence was not recorded"

PATH="$test_dir/bin:$PATH" \
SEOBLOG_DB_PATH="$test_dir/production.db" \
SEOBLOG_BACKUP_ENDPOINT="https://s3.example.invalid" \
SEOBLOG_BACKUP_REGION="test-region" \
SEOBLOG_BACKUP_BUCKET="valid-test-bucket" \
SEOBLOG_BACKUP_RESTORE_KEY_ID="restore-reader" \
SEOBLOG_BACKUP_RESTORE_APPLICATION_KEY="not-a-secret" \
SEOBLOG_BACKUP_EVIDENCE_DIR="$test_dir/evidence" \
SEOBLOG_RESTORE_SERVICES_STOPPED="true" \
SEOBLOG_RESTORE_CONFIRM="$test_dir/production.db" \
SEOBLOG_TEST_SNAPSHOT="$test_dir/source.db" \
SEOBLOG_TEST_REMOTE="$test_dir/remote.db" \
  "$repo_root/infra/backup/restore-primary.sh" \
    --snapshot-key snapshots/test.db \
    --target "$test_dir/production.db" \
    --authorized-by test-operator \
    --change-id TEST-RESTORE >/dev/null

[ "$(sqlite3 "$test_dir/production.db" 'SELECT COUNT(*) FROM verification;')" = "1" ] || fail "guarded snapshot restore did not install the validated database"
grep -q 'TEST-RESTORE' "$test_dir/evidence/restores.jsonl" || fail "restore evidence was not recorded"

printf '[backup-test] backup and restore automation checks passed\n'
