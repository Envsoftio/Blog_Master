#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
fail() { printf '[observability-test] ERROR: %s\n' "$*" >&2; exit 1; }

node --check "$repo_root/infra/observability/export-runtime-metrics.mjs"
node -e "JSON.parse(require('node:fs').readFileSync(process.argv[1], 'utf8'))" "$repo_root/infra/observability/grafana-dashboard.json"

grep -q '127.0.0.1:8080' "$repo_root/infra/observability/alloy.config" || fail "API metrics scrape must remain loopback-only"
grep -q '127.0.0.1:9092' "$repo_root/infra/observability/alloy.config" || fail "worker metrics scrape must remain loopback-only"
grep -q 'sys.env("SEOBLOG_PROMETHEUS_PASSWORD")' "$repo_root/infra/observability/alloy.config" || fail "managed metrics credentials must come from protected environment"
grep -q 'SEOBlogPublicationOutboxDelayed' "$repo_root/infra/observability/alerts.yml" || fail "two-minute outbox alert is required"
grep -q 'seoblog_backup_last_verified_timestamp_seconds' "$repo_root/infra/observability/alerts.yml" || fail "backup freshness alert is required"
grep -q 'seoblog_pm2_saved_process_present' "$repo_root/infra/observability/alerts.yml" || fail "PM2 saved-list alert is required"
grep -q -- '--configure-observability' "$repo_root/scripts/setup-vps-cicd.sh" || fail "VPS observability setup integration is required"
grep -q 'v1.18.0' "$repo_root/scripts/setup-vps-cicd.sh" || fail "Grafana Alloy must be pinned"
while IFS= read -r alert_name; do
  [[ "$alert_name" =~ ^[A-Za-z_:][A-Za-z0-9_:]*$ ]] || fail "invalid Prometheus alert name: $alert_name"
done < <(sed -n 's/^[[:space:]]*- alert: //p' "$repo_root/infra/observability/alerts.yml")

test_dir="$(mktemp -d "${TMPDIR:-/tmp}/seoblog-observability-test.XXXXXX")"
cleanup() { rm -rf -- "$test_dir"; }
trap cleanup EXIT
mkdir -p "$test_dir/bin" "$test_dir/deploy/shared/backup-evidence" "$test_dir/home/.pm2"
cat > "$test_dir/bin/pm2" <<'FAKEPM2'
#!/usr/bin/env bash
printf '[{"name":"seoblog-admin","pm2_env":{"status":"online","restart_time":1},"monit":{"memory":1024}},{"name":"seoblog-api","pm2_env":{"status":"online","restart_time":2},"monit":{"memory":2048}},{"name":"seoblog-worker","pm2_env":{"status":"online","restart_time":3},"monit":{"memory":4096}}]\n'
FAKEPM2
chmod +x "$test_dir/bin/pm2"
printf '[{"name":"seoblog-admin"},{"name":"seoblog-api"},{"name":"seoblog-worker"}]\n' > "$test_dir/home/.pm2/dump.pm2"
printf '%s\n' '{"verifiedAt":"2026-08-01T00:00:00.000Z","checks":{"downloadedChecksum":true,"quickCheck":"ok","foreignKeyCheck":"ok"}}' > "$test_dir/deploy/shared/backup-evidence/recovery-points.jsonl"
printf '%s\n' '{"recordedAt":"2026-08-01T00:00:00.000Z","status":"succeeded"}' > "$test_dir/deploy/shared/deployments.jsonl"

PATH="$test_dir/bin:$PATH" SEOBLOG_PM2_HOME="$test_dir/home/.pm2" SEOBLOG_DEPLOY_PATH="$test_dir/deploy" SEOBLOG_METRICS_TEXTFILE="$test_dir/seoblog.prom" node "$repo_root/infra/observability/export-runtime-metrics.mjs"
grep -q 'seoblog_pm2_process_up{name="seoblog-api"} 1' "$test_dir/seoblog.prom" || fail "runtime exporter did not report PM2 state"
grep -q 'seoblog_backup_last_verification_success 1' "$test_dir/seoblog.prom" || fail "runtime exporter did not report backup verification"
grep -q 'seoblog_deployment_last_status{status="succeeded"} 1' "$test_dir/seoblog.prom" || fail "runtime exporter did not report release state"

if command -v promtool >/dev/null 2>&1; then
  promtool check rules "$repo_root/infra/observability/alerts.yml"
fi
if command -v alloy >/dev/null 2>&1; then
  alloy validate "$repo_root/infra/observability/alloy.config"
fi

printf '[observability-test] observability contracts passed\n'
