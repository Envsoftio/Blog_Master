#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="${APP_NAME:-seoblog}"
DEPLOY_PATH="${DEPLOY_PATH:-/srv/${APP_NAME}}"
RELEASES_DIR="${DEPLOY_PATH}/releases"
SHARED_DIR="${DEPLOY_PATH}/shared"
CURRENT_LINK="${DEPLOY_PATH}/current"
KEEP_RELEASES="${KEEP_RELEASES:-5}"
ARCHIVE="${1:-}"

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

rollback() {
  if [ -n "${PREVIOUS_RELEASE:-}" ] && [ -e "$PREVIOUS_RELEASE" ]; then
    log "rolling back to $PREVIOUS_RELEASE"
    ln -sfn "$PREVIOUS_RELEASE" "$CURRENT_LINK"
    if [ -f "$CURRENT_LINK/ecosystem.config.cjs" ]; then
      pm2 startOrReload "$CURRENT_LINK/ecosystem.config.cjs" --env production --update-env || true
    fi
  fi
}

health_url() {
  local addr="${SEOBLOG_HTTP_ADDR:-127.0.0.1:8080}"
  if [[ "$addr" == :* ]]; then
    printf 'http://127.0.0.1%s/healthz' "$addr"
  else
    printf 'http://%s/healthz' "$addr"
  fi
}

cleanup_old_releases() {
  local -a releases=()
  local release
  while IFS= read -r -d '' release; do
    releases+=("$release")
  done < <(find "$RELEASES_DIR" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)

  local count="${#releases[@]}"
  if (( count <= KEEP_RELEASES )); then
    return
  fi

  local remove_count=$((count - KEEP_RELEASES))
  local current_release=""
  local previous_release=""
  local candidate=""
  local removed=0
  current_release="$(readlink -f "$CURRENT_LINK" 2>/dev/null || true)"
  if [ -n "${PREVIOUS_RELEASE:-}" ]; then
    previous_release="$(readlink -f "$PREVIOUS_RELEASE" 2>/dev/null || true)"
  fi

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

[ -n "$ARCHIVE" ] || fail "usage: DEPLOY_PATH=/srv/seoblog $0 /tmp/seoblog-release.tar.gz"
[ -f "$ARCHIVE" ] || fail "release archive not found: $ARCHIVE"
command -v tar >/dev/null 2>&1 || fail "tar is required on the VPS"
command -v pm2 >/dev/null 2>&1 || fail "pm2 is required on the VPS"

mkdir -p "$RELEASES_DIR" "$SHARED_DIR"

RELEASE_ID="$(date -u +%Y%m%d%H%M%S)-${RELEASE_SHA:-manual}"
RELEASE_ID="$(printf '%s' "$RELEASE_ID" | tr -cs 'A-Za-z0-9._-' '-')"
RELEASE_DIR="${RELEASES_DIR}/${RELEASE_ID}"
if [ -e "$RELEASE_DIR" ]; then
  RELEASE_DIR="${RELEASES_DIR}/${RELEASE_ID}-$(date -u +%s)"
fi

PREVIOUS_RELEASE=""
if [ -L "$CURRENT_LINK" ]; then
  PREVIOUS_RELEASE="$(readlink "$CURRENT_LINK" || true)"
fi

log "extracting release to $RELEASE_DIR"
mkdir -p "$RELEASE_DIR"
tar -xzf "$ARCHIVE" -C "$RELEASE_DIR"

[ -f "$RELEASE_DIR/ecosystem.config.cjs" ] || fail "release is missing ecosystem.config.cjs"
[ -x "$RELEASE_DIR/backend/api" ] || chmod +x "$RELEASE_DIR/backend/api"
[ -x "$RELEASE_DIR/backend/worker" ] || chmod +x "$RELEASE_DIR/backend/worker"
[ -x "$RELEASE_DIR/backend/admincli" ] || chmod +x "$RELEASE_DIR/backend/admincli"
touch "$RELEASE_DIR"

if [ ! -f "$SHARED_DIR/.env" ]; then
  log "creating default shared env at $SHARED_DIR/.env"
  umask 077
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
SEOBLOG_SMTP_FROM_NAME=Example Team
NITRO_HOST=127.0.0.1
NITRO_PORT=3000
NUXT_API_BASE_URL=http://127.0.0.1:8080
SEOBLOG_RELEASE_ROOT=${CURRENT_LINK}
ENV
fi

ln -sfn "$SHARED_DIR/.env" "$RELEASE_DIR/.env"

set -a
# shellcheck disable=SC1091
. "$SHARED_DIR/.env"
set +a

log "switching current release"
ln -sfn "$RELEASE_DIR" "$CURRENT_LINK"

log "reloading pm2"
if ! pm2 startOrReload "$CURRENT_LINK/ecosystem.config.cjs" --env production --update-env; then
  rollback
  fail "pm2 reload failed"
fi

if command -v curl >/dev/null 2>&1; then
  url="$(health_url)"
  log "checking $url"
  healthy=0
  for _ in {1..12}; do
    if curl -fsS "$url" >/dev/null; then
      healthy=1
      break
    fi
    sleep 2
  done

  if [ "$healthy" -ne 1 ]; then
    rollback
    fail "health check failed: $url"
  fi
else
  log "curl not found; skipping health check"
fi

pm2 save >/dev/null
cleanup_old_releases
log "deployed $RELEASE_DIR"
