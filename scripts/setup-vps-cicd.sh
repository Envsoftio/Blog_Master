#!/usr/bin/env bash
set -euo pipefail

APP_NAME="seoblog"
REMOTE_HOST=""
REMOTE_USER=""
SSH_PORT="22"
DEPLOY_PATH="/srv/seoblog"
ADMIN_PORT="3000"
API_PORT="8080"
DOMAIN=""
CONFIGURE_NGINX="0"
TLS_CERT_PATH=""
TLS_KEY_PATH=""
EXTERNAL_TLS_TERMINATION="0"
SET_GITHUB_SECRETS="0"
GH_REPO=""
KEY_PATH=""

usage() {
  cat <<'USAGE'
Usage:
  ./scripts/setup-vps-cicd.sh --host <vps-ip-or-domain> --user <ssh-user> [options]

Options:
  --port <port>                SSH port. Default: 22
  --deploy-path <path>         VPS release root. Default: /srv/seoblog
  --admin-port <port>          Local Nuxt/PM2 port. Default: 3000
  --api-port <port>            Local Go API/PM2 port. Default: 8080
  --domain <domain>            Public domain for optional Nginx config
  --configure-nginx            Create/reload an Nginx site for --domain
  --tls-cert <remote-path>     TLS certificate chain on the VPS
  --tls-key <remote-path>      TLS private key on the VPS
  --external-tls               TLS terminates at a trusted upstream proxy
  --repo <owner/repo>          GitHub repo for setting secrets
  --set-github-secrets         Use gh CLI to set GitHub Actions secrets
  --key-path <path>            Local deploy key path. Default: .deploy/seoblog_github_actions_ed25519
  -h, --help                   Show this help

Example:
  ./scripts/setup-vps-cicd.sh --host 203.0.113.10 --user deploy --repo Envsoftio/Blog_Master --set-github-secrets

With custom ports and Nginx:
  ./scripts/setup-vps-cicd.sh --host 203.0.113.10 --user deploy --admin-port 3010 --api-port 8090 --domain cms.example.com --configure-nginx --tls-cert /etc/letsencrypt/live/cms.example.com/fullchain.pem --tls-key /etc/letsencrypt/live/cms.example.com/privkey.pem
USAGE
}

log() {
  printf '[setup] %s\n' "$*"
}

die() {
  printf '[setup] ERROR: %s\n' "$*" >&2
  exit 1
}

quote() {
  printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
}

detect_repo() {
  local url
  url="$(git config --get remote.origin.url 2>/dev/null || true)"
  case "$url" in
    git@github.com:*.git)
      printf '%s\n' "${url#git@github.com:}" | sed 's/\.git$//'
      ;;
    git@github.com:*)
      printf '%s\n' "${url#git@github.com:}"
      ;;
    https://github.com/*.git)
      printf '%s\n' "${url#https://github.com/}" | sed 's/\.git$//'
      ;;
    https://github.com/*)
      printf '%s\n' "${url#https://github.com/}"
      ;;
  esac
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --host)
      REMOTE_HOST="${2:-}"
      shift 2
      ;;
    --user)
      REMOTE_USER="${2:-}"
      shift 2
      ;;
    --port)
      SSH_PORT="${2:-}"
      shift 2
      ;;
    --deploy-path)
      DEPLOY_PATH="${2:-}"
      shift 2
      ;;
    --admin-port)
      ADMIN_PORT="${2:-}"
      shift 2
      ;;
    --api-port)
      API_PORT="${2:-}"
      shift 2
      ;;
    --domain)
      DOMAIN="${2:-}"
      shift 2
      ;;
    --configure-nginx)
      CONFIGURE_NGINX="1"
      shift
      ;;
    --tls-cert)
      TLS_CERT_PATH="${2:-}"
      shift 2
      ;;
    --tls-key)
      TLS_KEY_PATH="${2:-}"
      shift 2
      ;;
    --external-tls)
      EXTERNAL_TLS_TERMINATION="1"
      shift
      ;;
    --repo)
      GH_REPO="${2:-}"
      shift 2
      ;;
    --set-github-secrets)
      SET_GITHUB_SECRETS="1"
      shift
      ;;
    --key-path)
      KEY_PATH="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

[ -n "$REMOTE_HOST" ] || die "--host is required"
[ -n "$REMOTE_USER" ] || die "--user is required"
[ -n "$DEPLOY_PATH" ] || die "--deploy-path cannot be empty"
[ "$CONFIGURE_NGINX" = "0" ] || [ -n "$DOMAIN" ] || die "--configure-nginx requires --domain"
if [ "$CONFIGURE_NGINX" = "1" ]; then
  if [ "$EXTERNAL_TLS_TERMINATION" = "1" ]; then
    [ -z "$TLS_CERT_PATH" ] && [ -z "$TLS_KEY_PATH" ] || die "--external-tls cannot be combined with --tls-cert or --tls-key"
  else
    [ -n "$TLS_CERT_PATH" ] && [ -n "$TLS_KEY_PATH" ] || die "--configure-nginx requires --tls-cert and --tls-key unless --external-tls is set"
  fi
elif [ -n "$TLS_CERT_PATH" ] || [ -n "$TLS_KEY_PATH" ] || [ "$EXTERNAL_TLS_TERMINATION" = "1" ]; then
  die "TLS options require --configure-nginx"
fi

for cmd in ssh scp ssh-keygen sed; do
  command -v "$cmd" >/dev/null 2>&1 || die "$cmd is required locally"
done

if [ -z "$KEY_PATH" ]; then
  KEY_PATH=".deploy/${APP_NAME}_github_actions_ed25519"
fi

mkdir -p "$(dirname "$KEY_PATH")"
if [ ! -f "$KEY_PATH" ]; then
  log "generating deploy key at $KEY_PATH"
  ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "github-actions-${APP_NAME}" >/dev/null
else
  log "using existing deploy key at $KEY_PATH"
fi
chmod 600 "$KEY_PATH"

SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"
SSH_OPTS=(-p "$SSH_PORT")
SCP_OPTS=(-P "$SSH_PORT")

log "checking SSH access to $SSH_TARGET"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "printf 'connected to %s\n' \"\$(hostname)\""

PUBLIC_KEY="$(tr -d '\n' < "${KEY_PATH}.pub")"
log "installing GitHub Actions public key for $SSH_TARGET"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "PUB_KEY=$(quote "$PUBLIC_KEY") bash -s" <<'REMOTE'
set -euo pipefail
mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"
touch "$HOME/.ssh/authorized_keys"
chmod 600 "$HOME/.ssh/authorized_keys"
if ! grep -qxF "$PUB_KEY" "$HOME/.ssh/authorized_keys"; then
  printf '%s\n' "$PUB_KEY" >> "$HOME/.ssh/authorized_keys"
fi
REMOTE

REMOTE_ARCH="$(ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "uname -m" | tr -d '\r')"
case "$REMOTE_ARCH" in
  x86_64|amd64)
    VPS_GOARCH="amd64"
    ;;
  aarch64|arm64)
    VPS_GOARCH="arm64"
    ;;
  *)
    VPS_GOARCH="amd64"
    log "unknown remote architecture '$REMOTE_ARCH'; defaulting GitHub build target to amd64"
    ;;
esac

log "uploading VPS deploy script"
scp "${SCP_OPTS[@]}" "infra/deploy/deploy-release.sh" "$SSH_TARGET:/tmp/${APP_NAME}-deploy-release.sh"

REMOTE_SETUP_SCRIPT="/tmp/${APP_NAME}-setup-vps.sh"
log "uploading VPS setup helper"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "cat > $(quote "$REMOTE_SETUP_SCRIPT")" <<'REMOTE'
set -euo pipefail

export PATH="$HOME/.local/bin:$HOME/.npm-global/bin:/usr/local/bin:/usr/bin:/bin:$PATH"
if [ -s "$HOME/.nvm/nvm.sh" ]; then
  # shellcheck disable=SC1090
  . "$HOME/.nvm/nvm.sh"
  nvm use --silent default >/dev/null 2>&1 || true
fi

log() {
  printf '[vps] %s\n' "$*"
}

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required to prepare $DEPLOY_PATH" >&2
    exit 1
  fi
  SUDO="sudo"
fi

missing_runtime=0
for cmd in node pm2 tar; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "$cmd is required on the VPS" >&2
    missing_runtime=1
  fi
done
[ "$missing_runtime" -eq 0 ] || exit 1

if ! node -e "const [major] = process.versions.node.split('.').map(Number); process.exit(major >= 22 ? 0 : 1)"; then
  echo "Node.js 22+ is required on the VPS for the Nuxt server output" >&2
  exit 1
fi

$SUDO mkdir -p "$DEPLOY_PATH/releases" "$DEPLOY_PATH/shared" "$DEPLOY_PATH/logs"
$SUDO chown -R "$REMOTE_DEPLOY_USER" "$DEPLOY_PATH"
chmod 755 "$DEPLOY_PATH" "$DEPLOY_PATH/releases" "$DEPLOY_PATH/shared" "$DEPLOY_PATH/logs"
install -m 755 "/tmp/${APP_NAME}-deploy-release.sh" "$DEPLOY_PATH/deploy.sh"

SHARED_ENV="$DEPLOY_PATH/shared/.env"
if [ ! -f "$SHARED_ENV" ]; then
  log "creating $SHARED_ENV"
  umask 077
  WEBHOOK_ENCRYPTION_KEY="$(node -e "console.log(require('node:crypto').randomBytes(32).toString('base64'))")"
  cat > "$SHARED_ENV" <<ENV
SEOBLOG_ENV=production
SEOBLOG_HTTP_ADDR=127.0.0.1:${API_PORT}
SEOBLOG_DB_PATH=${DEPLOY_PATH}/shared/seoblog.db
SEOBLOG_DEV_AUTH=false
SEOBLOG_TRUSTED_PROXIES=127.0.0.1
SEOBLOG_ADMIN_PUBLIC_URL=
SEOBLOG_SMTP_ADDR=smtp.zeptomail.com:587
SEOBLOG_SMTP_USERNAME=emailapikey
SEOBLOG_SMTP_PASSWORD=
SEOBLOG_SMTP_REQUIRE_STARTTLS=true
SEOBLOG_SMTP_FROM=noreply@proctorplus.io
SEOBLOG_SMTP_FROM_NAME='Example Team'
SEOBLOG_WEBHOOK_ENCRYPTION_KEY=${WEBHOOK_ENCRYPTION_KEY}
SEOBLOG_WEBHOOK_ALLOWED_HOSTS=
SEOBLOG_AI_PROVIDER=openai-compatible
SEOBLOG_AI_BASE_URL=
SEOBLOG_AI_API_KEY=
SEOBLOG_AI_MODEL=
SEOBLOG_AI_TIMEOUT=90s
SEOBLOG_AI_MAX_INPUT_BYTES=262144
SEOBLOG_AI_MAX_OUTPUT_TOKENS=4096
SEOBLOG_DEPLOY_BACKUP_COMMAND=
SEOBLOG_DEPLOY_BACKUP_VERIFY_COMMAND=
SEOBLOG_DEPLOY_REQUIRE_BACKUP=false
SEOBLOG_DEPLOY_SKIP_BACKUP=false
SEOBLOG_DEPLOY_DRAIN_COMMAND=
SEOBLOG_DEPLOY_CONTENT_SMOKE_COMMAND=
NITRO_HOST=127.0.0.1
NITRO_PORT=${ADMIN_PORT}
NUXT_API_BASE_URL=http://127.0.0.1:${API_PORT}
SEOBLOG_RELEASE_ROOT=${DEPLOY_PATH}/current
ENV
else
  log "$SHARED_ENV already exists; leaving it unchanged"
fi

if [ "$CONFIGURE_NGINX" = "1" ]; then
  command -v nginx >/dev/null 2>&1 || { echo "nginx is required for --configure-nginx" >&2; exit 1; }
  SAFE_DOMAIN="$(printf '%s' "$DOMAIN" | tr -cs 'A-Za-z0-9._-' '-')"
  NGINX_BASENAME="${APP_NAME}-${SAFE_DOMAIN}.conf"
  if [ -d /etc/nginx/sites-available ] && [ -d /etc/nginx/sites-enabled ]; then
    NGINX_CONF="/etc/nginx/sites-available/${NGINX_BASENAME}"
    NGINX_LINK="/etc/nginx/sites-enabled/${NGINX_BASENAME}"
  else
    NGINX_CONF="/etc/nginx/conf.d/${NGINX_BASENAME}"
    NGINX_LINK=""
  fi

  EXISTING_DOMAIN_CONFIGS="$($SUDO nginx -T 2>/dev/null | awk -v domain="$DOMAIN" -v target="$NGINX_CONF" -v link="$NGINX_LINK" '
    /^# configuration file / {
      file = $4
      sub(/:$/, "", file)
    }
    $1 == "server_name" {
      for (i = 2; i <= NF; i++) {
        value = $i
        gsub(/;/, "", value)
        if (value == domain && file != target && file != link) {
          print file
        }
      }
    }
  ' | sort -u || true)"
  if [ -n "$EXISTING_DOMAIN_CONFIGS" ]; then
    echo "server_name ${DOMAIN} already exists outside ${NGINX_CONF}:" >&2
    printf '%s\n' "$EXISTING_DOMAIN_CONFIGS" >&2
    echo "Refusing to write a duplicate Nginx server block." >&2
    exit 1
  fi

  if [ "$EXTERNAL_TLS_TERMINATION" = "1" ]; then
    HTTP_REDIRECT_BLOCK=""
    APP_LISTEN="listen 80;"
    TLS_DIRECTIVES=""
    FORWARDED_PROTO="https"
  else
    $SUDO test -r "$TLS_CERT_PATH" || { echo "TLS certificate is not readable: $TLS_CERT_PATH" >&2; exit 1; }
    $SUDO test -r "$TLS_KEY_PATH" || { echo "TLS private key is not readable: $TLS_KEY_PATH" >&2; exit 1; }
    HTTP_REDIRECT_BLOCK="$(cat <<REDIRECT
server {
  listen 80;
  server_name ${DOMAIN};
  return 301 https://\$host\$request_uri;
}
REDIRECT
)"
    APP_LISTEN="listen 443 ssl http2;"
    TLS_DIRECTIVES="$(cat <<TLS
  ssl_certificate ${TLS_CERT_PATH};
  ssl_certificate_key ${TLS_KEY_PATH};
  ssl_protocols TLSv1.2 TLSv1.3;
  ssl_session_cache shared:SEOBLOG_SSL:10m;
  ssl_session_timeout 1d;
TLS
)"
    FORWARDED_PROTO="\$scheme"
  fi

  NGINX_TMP="${NGINX_CONF}.tmp.$$"
  NGINX_BACKUP=""
  LINK_BACKUP_TARGET=""
  LINK_EXISTED="0"
  if [ -e "$NGINX_CONF" ]; then
    NGINX_BACKUP="${NGINX_CONF}.bak.$(date -u +%Y%m%d%H%M%S)"
    $SUDO cp -a "$NGINX_CONF" "$NGINX_BACKUP"
  fi
  if [ -n "$NGINX_LINK" ] && [ -L "$NGINX_LINK" ]; then
    LINK_EXISTED="1"
    LINK_BACKUP_TARGET="$(readlink "$NGINX_LINK")"
  elif [ -n "$NGINX_LINK" ] && [ -e "$NGINX_LINK" ]; then
    echo "Nginx enabled path exists but is not a symlink: $NGINX_LINK" >&2
    exit 1
  fi

  restore_nginx_config() {
    if [ -n "$NGINX_BACKUP" ] && [ -f "$NGINX_BACKUP" ]; then
      $SUDO cp -a "$NGINX_BACKUP" "$NGINX_CONF"
    else
      $SUDO rm -f "$NGINX_CONF"
    fi
    if [ -n "$NGINX_LINK" ]; then
      if [ "$LINK_EXISTED" = "1" ]; then
        $SUDO ln -sfn "$LINK_BACKUP_TARGET" "$NGINX_LINK"
      else
        $SUDO rm -f "$NGINX_LINK"
      fi
    fi
  }

  log "writing Nginx config for ${DOMAIN} at ${NGINX_CONF}"
  $SUDO tee "$NGINX_TMP" >/dev/null <<NGINX
${HTTP_REDIRECT_BLOCK}
server {
  ${APP_LISTEN}
  server_name ${DOMAIN};

${TLS_DIRECTIVES}
  client_max_body_size 2m;
  proxy_connect_timeout 5s;
  proxy_read_timeout 60s;
  proxy_send_timeout 60s;

  location /api/ {
    proxy_pass http://127.0.0.1:${API_PORT};
    proxy_set_header Host \$host;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto ${FORWARDED_PROTO};
  }

  location /content/ {
    proxy_pass http://127.0.0.1:${API_PORT};
    proxy_set_header Host \$host;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto ${FORWARDED_PROTO};
  }

  location /healthz {
    proxy_pass http://127.0.0.1:${API_PORT};
  }

  location /readyz {
    proxy_pass http://127.0.0.1:${API_PORT};
  }

  location /_nuxt/ {
    proxy_pass http://127.0.0.1:${ADMIN_PORT};
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
    proxy_set_header Upgrade \$http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto ${FORWARDED_PROTO};
  }

  location / {
    proxy_pass http://127.0.0.1:${ADMIN_PORT};
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
    proxy_set_header Upgrade \$http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto ${FORWARDED_PROTO};
  }
}
NGINX

  $SUDO mv "$NGINX_TMP" "$NGINX_CONF"
  if [ -n "$NGINX_LINK" ]; then
    $SUDO ln -sfn "$NGINX_CONF" "$NGINX_LINK"
  fi
  if ! $SUDO nginx -t; then
    echo "Nginx validation failed; restoring previous config." >&2
    restore_nginx_config
    $SUDO nginx -t || true
    exit 1
  fi
  if ! command -v systemctl >/dev/null 2>&1 || ! $SUDO systemctl reload nginx; then
    $SUDO nginx -s reload
  fi
fi

log "bootstrap complete"
REMOTE
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "chmod 700 $(quote "$REMOTE_SETUP_SCRIPT")"

log "preparing $DEPLOY_PATH on the VPS"
ssh -tt "${SSH_OPTS[@]}" "$SSH_TARGET" \
  "APP_NAME=$(quote "$APP_NAME") DEPLOY_PATH=$(quote "$DEPLOY_PATH") REMOTE_DEPLOY_USER=$(quote "$REMOTE_USER") ADMIN_PORT=$(quote "$ADMIN_PORT") API_PORT=$(quote "$API_PORT") DOMAIN=$(quote "$DOMAIN") CONFIGURE_NGINX=$(quote "$CONFIGURE_NGINX") TLS_CERT_PATH=$(quote "$TLS_CERT_PATH") TLS_KEY_PATH=$(quote "$TLS_KEY_PATH") EXTERNAL_TLS_TERMINATION=$(quote "$EXTERNAL_TLS_TERMINATION") bash $(quote "$REMOTE_SETUP_SCRIPT"); status=\$?; rm -f $(quote "$REMOTE_SETUP_SCRIPT"); exit \$status"

KNOWN_HOSTS="$(ssh-keyscan -p "$SSH_PORT" "$REMOTE_HOST" 2>/dev/null || true)"

if [ "$SET_GITHUB_SECRETS" = "1" ]; then
  command -v gh >/dev/null 2>&1 || die "gh CLI is required for --set-github-secrets"
  if [ -z "$GH_REPO" ]; then
    GH_REPO="$(detect_repo || true)"
  fi
  [ -n "$GH_REPO" ] || die "could not detect GitHub repo; pass --repo owner/repo"

  log "setting GitHub Actions secrets on $GH_REPO"
  gh auth status >/dev/null
  gh secret set VPS_HOST --repo "$GH_REPO" --body "$REMOTE_HOST"
  gh secret set VPS_USER --repo "$GH_REPO" --body "$REMOTE_USER"
  gh secret set VPS_SSH_PORT --repo "$GH_REPO" --body "$SSH_PORT"
  gh secret set VPS_DEPLOY_PATH --repo "$GH_REPO" --body "$DEPLOY_PATH"
  gh secret set VPS_GOARCH --repo "$GH_REPO" --body "$VPS_GOARCH"
  gh secret set VPS_SSH_KEY --repo "$GH_REPO" < "$KEY_PATH"
  if [ -n "$KNOWN_HOSTS" ]; then
    gh secret set VPS_SSH_KNOWN_HOSTS --repo "$GH_REPO" --body "$KNOWN_HOSTS"
  fi
else
  cat <<SUMMARY

VPS setup is complete.

Add these GitHub Actions secrets to the repository:
  VPS_HOST=$REMOTE_HOST
  VPS_USER=$REMOTE_USER
  VPS_SSH_PORT=$SSH_PORT
  VPS_DEPLOY_PATH=$DEPLOY_PATH
  VPS_GOARCH=$VPS_GOARCH
  VPS_SSH_KEY=<contents of $KEY_PATH>

Optional:
  VPS_SSH_KNOWN_HOSTS=<output of: ssh-keyscan -p $SSH_PORT $REMOTE_HOST>

Or rerun this script with --set-github-secrets --repo owner/repo if you have gh authenticated.
SUMMARY
fi

log "done"
