# VPS CI/CD

This repo deploys as one artifact:

- `admin/.output` from the Nuxt build.
- `backend/api`, `backend/worker` and `backend/admincli` Linux binaries.
- `contracts/openapi/openapi.yaml` and the built `packages/content-client` package.
- `ecosystem.config.cjs` for PM2.
- `release.json` with release ID, commit SHA, build metadata and SHA-256 checksums.

The VPS keeps releases under `/srv/seoblog/releases`, points `/srv/seoblog/current` at the active release, and keeps persistent data in `/srv/seoblog/shared`.

## One-time VPS setup

Run the setup from this repo on your machine. Use the same SSH user that owns or should own the PM2 processes.

```bash
./scripts/setup-vps-cicd.sh --host YOUR_VPS_HOST --user YOUR_SSH_USER --repo Envsoftio/Blog_Master --set-github-secrets
```

If `3000` or `8080` are already used by your other PM2 projects, choose different local ports:

```bash
./scripts/setup-vps-cicd.sh --host YOUR_VPS_HOST --user YOUR_SSH_USER --admin-port 3010 --api-port 8090 --repo Envsoftio/Blog_Master --set-github-secrets
```

To write a directly exposed HTTPS Nginx site, provision a certificate on the VPS first and pass its remote paths:

```bash
./scripts/setup-vps-cicd.sh \
  --host YOUR_VPS_HOST \
  --user YOUR_SSH_USER \
  --domain cms.example.com \
  --configure-nginx \
  --tls-cert /etc/letsencrypt/live/cms.example.com/fullchain.pem \
  --tls-key /etc/letsencrypt/live/cms.example.com/privkey.pem
```

This writes an HTTP-to-HTTPS redirect and serves the application on port 443. If HTTPS terminates at a trusted upstream proxy such as a load balancer or CDN and the origin connection is protected, use `--configure-nginx --external-tls` instead. The setup script refuses an HTTP-only production site unless that upstream TLS mode is selected explicitly.

The script creates a dedicated deploy SSH key at `.deploy/seoblog_github_actions_ed25519`, adds the public key to the VPS user's `authorized_keys`, prepares the release folders, uploads `/srv/seoblog/deploy.sh`, and optionally sets GitHub Actions secrets through `gh`.

## GitHub secrets

If you do not use `--set-github-secrets`, add these secrets manually:

- `VPS_HOST`
- `VPS_USER`
- `VPS_SSH_KEY`
- `VPS_SSH_PORT`, optional, defaults to `22`
- `VPS_DEPLOY_PATH`, optional, defaults to `/srv/seoblog`
- `VPS_GOARCH`, optional, `amd64` or `arm64`, defaults to `amd64`
- `VPS_SSH_KNOWN_HOSTS`, optional

## Runtime config

Edit `/srv/seoblog/shared/.env` on the VPS for production values. The setup script creates a safe default:

```bash
SEOBLOG_ENV=production
SEOBLOG_HTTP_ADDR=127.0.0.1:8080
SEOBLOG_DB_PATH=/srv/seoblog/shared/seoblog.db
SEOBLOG_DEV_AUTH=false
SEOBLOG_TRUSTED_PROXIES=127.0.0.1
SEOBLOG_ADMIN_PUBLIC_URL=https://admin.example.com
SEOBLOG_SMTP_ADDR=smtp.zeptomail.com:587
SEOBLOG_SMTP_USERNAME=emailapikey
SEOBLOG_SMTP_PASSWORD=
SEOBLOG_SMTP_REQUIRE_STARTTLS=true
SEOBLOG_SMTP_FROM=noreply@proctorplus.io
SEOBLOG_SMTP_FROM_NAME='Example Team'
SEOBLOG_WEBHOOK_ENCRYPTION_KEY=xqhcQ/knhyV37B0W4qeA73cLHgFyMwPojXHnW0xVv/Y=
SEOBLOG_AI_PROVIDER=openai-compatible
SEOBLOG_AI_BASE_URL=https://provider.example.com/v1
SEOBLOG_AI_API_KEY=
SEOBLOG_AI_MODEL=
SEOBLOG_AI_TIMEOUT=90s
SEOBLOG_AI_MAX_INPUT_BYTES=262144
SEOBLOG_AI_MAX_OUTPUT_TOKENS=4096
SEOBLOG_DEPLOY_BACKUP_COMMAND=/srv/seoblog/backup/create-recovery-point.sh pre-release
SEOBLOG_DEPLOY_BACKUP_VERIFY_COMMAND=
SEOBLOG_DEPLOY_REQUIRE_BACKUP=true
SEOBLOG_DEPLOY_SKIP_BACKUP=false
SEOBLOG_WORKER_METRICS_ADDR=127.0.0.1:9092
SEOBLOG_DEPLOY_DRAIN_COMMAND=
SEOBLOG_DEPLOY_CONTENT_SMOKE_COMMAND=
NITRO_HOST=127.0.0.1
NITRO_PORT=3000
NUXT_API_BASE_URL=http://127.0.0.1:8080
SEOBLOG_RELEASE_ROOT=/srv/seoblog/current
```

Set the public admin URL and ZeptoMail API-key password before enabling invitation or password-recovery email. Generate `SEOBLOG_WEBHOOK_ENCRYPTION_KEY` as 32 random bytes encoded with standard Base64 and provide the same value to the API and worker. Staging must also set `SEOBLOG_WEBHOOK_ALLOWED_HOSTS` to its non-production receiver hosts; an empty staging allowlist blocks all delivery. Production SMTP requires STARTTLS and the API never returns reset tokens in an API response. The signing and receiver contract is documented in [webhooks.md](webhooks.md).

The API metrics route uses the loopback API listener, and the worker metrics listener must remain loopback-only. Install the pinned collector and runtime evidence units with `--configure-observability`, then follow [observability operations](observability.md) to provision protected managed-service credentials, dashboard/alert definitions and preflight tests before enabling the units.

AI execution is disabled unless the base URL, API key, and model are all set. The worker calls an OpenAI-compatible chat-completions endpoint, enforces input/output bounds, and persists generated JSON as a review-only proposal with its run provenance. Use HTTPS for remote providers; plain HTTP is accepted only for a loopback development endpoint.

## Release safety

`task deploy:prod RELEASE=<immutable-release-id>` calls the VPS deploy script with `/tmp/seoblog-release-<release-id>.tar.gz` unless `ARCHIVE=<path>` is supplied. The archive must contain `release.json`, and the deploy script verifies every listed checksum before installing it.

For an existing SQLite database, the default `SEOBLOG_DEPLOY_BACKUP_COMMAND` creates an isolated SQLite `.db` backup, uploads it as an immutable B2 snapshot, downloads it and verifies SQLite before returning success. `SEOBLOG_DEPLOY_REQUIRE_BACKUP=true` is the default and makes this a hard production gate; the deploy script rejects the skip flag while the gate is required. Provision credentials, timers and restore access using [the backup and recovery runbook](backup-recovery.md) before the next deployment of an existing database.

The deploy script stops the worker before migrations, runs the new release's `backend/admincli migrate` before switching `/srv/seoblog/current`, restarts only `seoblog-admin`, `seoblog-api` and `seoblog-worker`, checks API readiness, API health and Nuxt SSR, then records the result in `/srv/seoblog/shared/deployments.jsonl`.

## First owner

The application has no public bootstrap route. After the first deployment to a new database, create the first owner through the packaged CLI:

```bash
cd /srv/seoblog/current
read -rsp 'Bootstrap password: ' SEOBLOG_BOOTSTRAP_PASSWORD
echo
export SEOBLOG_BOOTSTRAP_PASSWORD
./backend/admincli bootstrap-owner -email owner@example.com
unset SEOBLOG_BOOTSTRAP_PASSWORD
```

Every push to `main` runs tests, builds the checksummed release artifact, uploads it to the VPS, verifies the manifest, runs the guarded deploy flow and checks the API plus Nuxt SSR before recording success.
