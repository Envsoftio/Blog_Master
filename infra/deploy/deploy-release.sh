#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="${APP_NAME:-seoblog}"
DEPLOY_PATH="${DEPLOY_PATH:-/srv/${APP_NAME}}"
RELEASES_DIR="${DEPLOY_PATH}/releases"
SHARED_DIR="${DEPLOY_PATH}/shared"
CURRENT_LINK="${DEPLOY_PATH}/current"
PREVIOUS_LINK="${DEPLOY_PATH}/previous"
DEPLOY_LOCK="${DEPLOY_PATH}/deploy.lock"
KEEP_RELEASES="${KEEP_RELEASES:-5}"
ARCHIVE="${1:-}"
EXPECTED_RELEASE_ID="${RELEASE:-}"
PM2_PROCESSES="${SEOBLOG_PM2_PROCESSES:-seoblog-admin,seoblog-api,seoblog-worker}"
MIN_FREE_KB="${SEOBLOG_DEPLOY_MIN_FREE_KB:-1048576}"
LOCK_DIR=""
STAGING_DIR=""
RELEASE_DIR=""
FINAL_RELEASE_DIR=""
RELEASE_ID=""
COMMIT_SHA=""
ACTIVATED="0"
WORKER_STOPPED="0"

export PATH="$HOME/.local/bin:$HOME/.npm-global/bin:/usr/local/bin:/usr/bin:/bin:$PATH"
if [ -s "$HOME/.nvm/nvm.sh" ]; then
  # shellcheck disable=SC1090
  . "$HOME/.nvm/nvm.sh"
  nvm use --silent default >/dev/null 2>&1 || true
fi

log() {
  printf '[deploy] %s\n' "$*"
}

fail() {
  printf '[deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

cleanup_staging() {
  if [ -n "$STAGING_DIR" ] && [ -d "$STAGING_DIR" ]; then
    rm -rf -- "$STAGING_DIR"
  fi
}

pm2_restart_current() {
  if [ -f "$CURRENT_LINK/ecosystem.config.cjs" ]; then
    pm2 startOrRestart "$CURRENT_LINK/ecosystem.config.cjs" --env production --update-env --only "$PM2_PROCESSES"
  fi
}

rollback() {
  if [ "$ACTIVATED" = "1" ] && [ -n "${PREVIOUS_RELEASE:-}" ] && [ -d "$PREVIOUS_RELEASE" ]; then
    log "rolling back current symlink to $PREVIOUS_RELEASE"
    ln -sfn "$PREVIOUS_RELEASE" "$CURRENT_LINK"
  fi
  if [ "$WORKER_STOPPED" = "1" ] || [ "$ACTIVATED" = "1" ]; then
    log "restoring PM2 process state"
    pm2_restart_current || true
  fi
}

on_exit() {
  status=$?
  cleanup_staging
  if [ "$status" -ne 0 ]; then
    rollback
    if [ -n "$RELEASE_ID" ] && [ -d "$SHARED_DIR" ]; then
      record_release_result "failed" || true
    fi
  fi
  cleanup_lock
  exit "$status"
}
trap on_exit EXIT

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required on the production host"
}

node_path_resolve() {
  node -e "const path = require('node:path'); console.log(path.resolve(process.argv[1]));" "$1"
}

trim_string() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

cleanup_lock() {
  if [ -n "$LOCK_DIR" ] && [ -d "$LOCK_DIR" ]; then
    rmdir "$LOCK_DIR" 2>/dev/null || true
  fi
}

acquire_deploy_lock() {
  if command -v flock >/dev/null 2>&1; then
    exec 9>"$DEPLOY_LOCK"
    flock -n 9 || fail "another deployment is already running"
    return
  fi

  LOCK_DIR="${DEPLOY_LOCK}.dir"
  if ! mkdir "$LOCK_DIR" 2>/dev/null; then
    fail "another deployment is already running"
  fi
}

health_url() {
  local path="${1:-/readyz}"
  local addr="${SEOBLOG_HTTP_ADDR:-127.0.0.1:8080}"
  if [[ "$addr" == :* ]]; then
    printf 'http://127.0.0.1%s%s' "$addr" "$path"
  else
    printf 'http://%s%s' "$addr" "$path"
  fi
}

admin_url() {
  local host="${NITRO_HOST:-127.0.0.1}"
  local port="${NITRO_PORT:-3000}"
  printf 'http://%s:%s/' "$host" "$port"
}

check_url() {
  local url="$1"
  if ! command -v curl >/dev/null 2>&1; then
    log "curl is not installed; skipping health check for $url"
    return
  fi
  log "checking $url"
  local healthy=0
  for _ in {1..18}; do
    if curl -fsS "$url" >/dev/null; then
      healthy=1
      break
    fi
    sleep 2
  done
  [ "$healthy" -eq 1 ] || fail "health check failed: $url"
}

create_default_shared_env() {
  if [ -f "$SHARED_DIR/.env" ]; then
    return
  fi

  log "creating default shared env at $SHARED_DIR/.env"
  umask 077
  local webhook_encryption_key
  webhook_encryption_key="$(node -e "console.log(require('node:crypto').randomBytes(32).toString('base64'))")"
  cat > "$SHARED_DIR/.env" <<ENV
SEOBLOG_ENV=production
SEOBLOG_HTTP_ADDR=127.0.0.1:8080
SEOBLOG_DB_PATH=${SHARED_DIR}/seoblog.db
SEOBLOG_DEV_AUTH=false
SEOBLOG_TRUSTED_PROXIES=127.0.0.1
SEOBLOG_ADMIN_PUBLIC_URL=
SEOBLOG_SMTP_ADDR=smtp.zeptomail.com:587
SEOBLOG_SMTP_USERNAME=emailapikey
SEOBLOG_SMTP_PASSWORD=
SEOBLOG_SMTP_REQUIRE_STARTTLS=true
SEOBLOG_SMTP_FROM=noreply@proctorplus.io
SEOBLOG_SMTP_FROM_NAME='Example Team'
SEOBLOG_WEBHOOK_ENCRYPTION_KEY=${webhook_encryption_key}
SEOBLOG_WEBHOOK_ALLOWED_HOSTS=
SEOBLOG_DEPLOY_BACKUP_COMMAND=${DEPLOY_PATH}/backup/create-recovery-point.sh pre-release
SEOBLOG_DEPLOY_BACKUP_VERIFY_COMMAND=
SEOBLOG_DEPLOY_REQUIRE_BACKUP=true
SEOBLOG_DEPLOY_SKIP_BACKUP=false
SEOBLOG_DEPLOY_DRAIN_COMMAND=
SEOBLOG_DEPLOY_CONTENT_SMOKE_COMMAND=
NITRO_HOST=127.0.0.1
NITRO_PORT=3000
NUXT_API_BASE_URL=http://127.0.0.1:8080
SEOBLOG_RELEASE_ROOT=${CURRENT_LINK}
ENV
}

load_shared_env() {
  local raw_line line key value quote
  while IFS= read -r raw_line || [ -n "$raw_line" ]; do
    line="$(trim_string "$raw_line")"
    if [ -z "$line" ] || [[ "$line" == \#* ]]; then
      continue
    fi
    if [[ "$line" == export[[:space:]]* ]]; then
      line="$(trim_string "${line#export}")"
    fi
    if [[ "$line" != *=* ]]; then
      continue
    fi
    key="$(trim_string "${line%%=*}")"
    value="$(trim_string "${line#*=}")"
    if [[ ! "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      continue
    fi
    if [ "${#value}" -ge 2 ]; then
      quote="${value:0:1}"
      if { [ "$quote" = "'" ] || [ "$quote" = '"' ]; } && [ "${value: -1}" = "$quote" ]; then
        value="${value:1:${#value}-2}"
        if [ "$quote" = '"' ]; then
          value="${value//\\n/$'\n'}"
          value="${value//\\\"/\"}"
          value="${value//\\\\/\\}"
        fi
      fi
    fi
    export "$key=$value"
  done < "$SHARED_DIR/.env"
}

validate_release_root() {
  [ -f "$RELEASE_DIR/ecosystem.config.cjs" ] || fail "release is missing ecosystem.config.cjs"
  [ -x "$RELEASE_DIR/backend/api" ] || chmod +x "$RELEASE_DIR/backend/api"
  [ -x "$RELEASE_DIR/backend/worker" ] || chmod +x "$RELEASE_DIR/backend/worker"
  [ -x "$RELEASE_DIR/backend/admincli" ] || chmod +x "$RELEASE_DIR/backend/admincli"
}

validate_database_path() {
  [ -n "${SEOBLOG_DB_PATH:-}" ] || fail "SEOBLOG_DB_PATH is required"
  local db_path releases_path current_path
  db_path="$(node_path_resolve "$SEOBLOG_DB_PATH")"
  releases_path="$(node_path_resolve "$RELEASES_DIR")"
  current_path="$(node_path_resolve "$CURRENT_LINK")"
  case "$db_path" in
    "$releases_path"/*|"$current_path"/*)
      fail "SEOBLOG_DB_PATH must live outside release directories: $SEOBLOG_DB_PATH"
      ;;
  esac
  mkdir -p "$(dirname "$SEOBLOG_DB_PATH")"
}

validate_disk_space() {
  local available_kb
  available_kb="$(df -Pk "$DEPLOY_PATH" | awk 'NR == 2 {print $4}')"
  [ -n "$available_kb" ] || fail "could not determine free disk space for $DEPLOY_PATH"
  if [ "$available_kb" -lt "$MIN_FREE_KB" ]; then
    fail "free disk space is below ${MIN_FREE_KB} KB"
  fi
}

run_worker_drain() {
  if [ -n "${SEOBLOG_DEPLOY_DRAIN_COMMAND:-}" ]; then
    log "running worker drain command"
    RELEASE_ID="$RELEASE_ID" COMMIT_SHA="$COMMIT_SHA" bash -c "$SEOBLOG_DEPLOY_DRAIN_COMMAND"
    WORKER_STOPPED="1"
    return
  fi

  log "stopping worker before migration"
  pm2 stop seoblog-worker >/dev/null 2>&1 || true
  WORKER_STOPPED="1"
}

create_recovery_point() {
  if [ ! -s "$SEOBLOG_DB_PATH" ]; then
    log "no existing SQLite database found; skipping pre-migration recovery point"
    return
  fi
  if [ "${SEOBLOG_DEPLOY_SKIP_BACKUP:-false}" = "true" ]; then
    if [ "${SEOBLOG_DEPLOY_REQUIRE_BACKUP:-true}" = "true" ]; then
      fail "production backup gate cannot be skipped while SEOBLOG_DEPLOY_REQUIRE_BACKUP=true"
    fi
    log "pre-migration recovery point skipped by SEOBLOG_DEPLOY_SKIP_BACKUP=true"
    return
  fi
  if [ -z "${SEOBLOG_DEPLOY_BACKUP_COMMAND:-}" ]; then
    if [ "${SEOBLOG_DEPLOY_REQUIRE_BACKUP:-true}" = "true" ]; then
      fail "SEOBLOG_DEPLOY_BACKUP_COMMAND is required for an existing production database"
    fi
    log "SEOBLOG_DEPLOY_BACKUP_COMMAND is not set; continuing for backward-compatible deploy"
    return
  fi

  log "creating pre-migration recovery point"
  RELEASE_ID="$RELEASE_ID" COMMIT_SHA="$COMMIT_SHA" SEOBLOG_DB_PATH="$SEOBLOG_DB_PATH" bash -c "$SEOBLOG_DEPLOY_BACKUP_COMMAND"
  if [ -n "${SEOBLOG_DEPLOY_BACKUP_VERIFY_COMMAND:-}" ]; then
    log "verifying pre-migration recovery point"
    RELEASE_ID="$RELEASE_ID" COMMIT_SHA="$COMMIT_SHA" SEOBLOG_DB_PATH="$SEOBLOG_DB_PATH" bash -c "$SEOBLOG_DEPLOY_BACKUP_VERIFY_COMMAND"
  fi
}

run_migrations() {
  log "running database migrations before activation"
  (
    cd "$RELEASE_DIR"
    SEOBLOG_RELEASE_ROOT="$RELEASE_DIR" ./backend/admincli migrate
  )
}

record_release_result() {
  local status="$1"
  local output="$SHARED_DIR/deployments.jsonl"
  node - "$output" "$status" "$RELEASE_ID" "$COMMIT_SHA" <<'NODE'
const fs = require('node:fs')
const [output, status, releaseId, commitSha] = process.argv.slice(2)
const record = {
  recordedAt: new Date().toISOString(),
  status,
  releaseId,
  commitSha
}
fs.appendFileSync(output, `${JSON.stringify(record)}\n`, { mode: 0o600 })
NODE
}

cleanup_old_releases() {
  local -a releases=()
  local release
  while IFS= read -r -d '' release; do
    releases+=("$release")
  done < <(find "$RELEASES_DIR" -mindepth 1 -maxdepth 1 -type d ! -name '.incoming.*' -print0 | sort -z)

  local count="${#releases[@]}"
  if (( count <= KEEP_RELEASES )); then
    return
  fi

  local remove_count=$((count - KEEP_RELEASES))
  local current_release previous_release candidate removed
  current_release="$(readlink -f "$CURRENT_LINK" 2>/dev/null || true)"
  previous_release="$(readlink -f "$PREVIOUS_LINK" 2>/dev/null || true)"
  removed=0

  for candidate in "${releases[@]}"; do
    if [ "$removed" -ge "$remove_count" ]; then
      break
    fi
    candidate="$(readlink -f "$candidate" 2>/dev/null || true)"
    if [ -z "$candidate" ] || [ "$candidate" = "$current_release" ] || [ "$candidate" = "$previous_release" ]; then
      continue
    fi
    log "removing old release $candidate"
    rm -rf -- "$candidate"
    removed=$((removed + 1))
  done
}

[ -n "$ARCHIVE" ] || fail "usage: RELEASE=<release-id> DEPLOY_PATH=/srv/seoblog $0 /tmp/seoblog-release.tar.gz"
[ -f "$ARCHIVE" ] || fail "release archive not found: $ARCHIVE"
require_command tar
require_command pm2
require_command node

mkdir -p "$RELEASES_DIR" "$SHARED_DIR"

acquire_deploy_lock

PREVIOUS_RELEASE=""
if [ -L "$CURRENT_LINK" ]; then
  PREVIOUS_RELEASE="$(readlink -f "$CURRENT_LINK" 2>/dev/null || true)"
fi

log "extracting release archive for verification"
STAGING_DIR="$(mktemp -d "${RELEASES_DIR}/.incoming.XXXXXX")"
tar -xzf "$ARCHIVE" -C "$STAGING_DIR"

[ -f "$STAGING_DIR/scripts/release-manifest.mjs" ] || fail "release is missing scripts/release-manifest.mjs"
node "$STAGING_DIR/scripts/release-manifest.mjs" verify "$STAGING_DIR" "$EXPECTED_RELEASE_ID"
RELEASE_ID="$(node "$STAGING_DIR/scripts/release-manifest.mjs" field "$STAGING_DIR" releaseId)"
COMMIT_SHA="$(node "$STAGING_DIR/scripts/release-manifest.mjs" field "$STAGING_DIR" commitSha)"
FINAL_RELEASE_DIR="${RELEASES_DIR}/${RELEASE_ID}"
[ ! -e "$FINAL_RELEASE_DIR" ] || fail "release already exists: $FINAL_RELEASE_DIR"

RELEASE_DIR="$STAGING_DIR"
validate_release_root

create_default_shared_env
ln -sfn "$SHARED_DIR/.env" "$RELEASE_DIR/.env"
load_shared_env
validate_database_path
validate_disk_space

run_worker_drain
create_recovery_point
run_migrations

log "installing verified release $RELEASE_ID"
mv "$STAGING_DIR" "$FINAL_RELEASE_DIR"
STAGING_DIR=""
RELEASE_DIR="$FINAL_RELEASE_DIR"

if [ -n "$PREVIOUS_RELEASE" ] && [ -d "$PREVIOUS_RELEASE" ]; then
  ln -sfn "$PREVIOUS_RELEASE" "$PREVIOUS_LINK"
fi

log "activating release $RELEASE_ID"
ln -sfn "$RELEASE_DIR" "$CURRENT_LINK"
ACTIVATED="1"

log "restarting exact PM2 applications: $PM2_PROCESSES"
pm2_restart_current

check_url "$(health_url /readyz)"
check_url "$(health_url /healthz)"
check_url "$(admin_url)"

if [ -n "${SEOBLOG_DEPLOY_CONTENT_SMOKE_COMMAND:-}" ]; then
  log "running Content API smoke command"
  RELEASE_ID="$RELEASE_ID" COMMIT_SHA="$COMMIT_SHA" bash -c "$SEOBLOG_DEPLOY_CONTENT_SMOKE_COMMAND"
fi

pm2 save >/dev/null
cleanup_old_releases
record_release_result "succeeded"
log "deployed $RELEASE_DIR"
