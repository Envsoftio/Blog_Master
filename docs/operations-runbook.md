# Production incident and recovery runbook

This is the operator-facing response contract for SEO Blog. Commands assume the production layout under `/srv/seoblog`, the dedicated `seoblog` application user and an incident/change system outside this repository. Replace names only when provisioning recorded the alternative.

## Common incident protocol

1. Acknowledge the alert, declare severity, name the incident commander and record the incident/change ID and UTC start time.
2. Preserve published landing/CDN availability. Pause only the affected egress or mutation path; do not destroy SQLite, Redis, releases, logs, backup evidence or pending jobs.
3. Capture the active release (`readlink -f /srv/seoblog/current`), `pm2 jlist`, relevant metrics, sanitized logs and the oldest affected record IDs. Never paste secrets or confidential content into the incident channel.
4. Stabilize first, diagnose second. Prefer disabling an optional provider or stopping the worker over making unreviewed database changes.
5. Use two-person approval for restoration, credential rotation, DNS/canonical changes or destructive provider actions. Record the exact commands and outcomes.
6. After recovery, verify `/healthz`, `/readyz`, admin SSR, a representative Content API read, worker/outbox state, metrics ingestion and alert resolution. Record impact, root cause, RPO/RTO where relevant and follow-up owners.

## API, admin or worker outage

- Check `pm2 status`, `pm2 logs <name> --lines 200 --nostream`, loopback health endpoints and `seoblog_pm2_process_*` metrics. Confirm the PM2 command is running as the dedicated application user.
- If one process stopped without a bad release or data fault, use the current declaration: `pm2 startOrRestart /srv/seoblog/current/ecosystem.config.cjs --env production --update-env --only "seoblog-admin,seoblog-api,seoblog-worker"` and then `pm2 save`.
- If it restart-loops, stop only that named process, preserve logs and inspect configuration, memory, disk, SQLite and provider errors before restarting. Do not use `pm2 delete all`.
- Keep the worker stopped if delivery side effects are uncertain. Admin and Content API reads can be recovered independently while pending work is reviewed.

## Lost session or leaked credential

- For a lost/compromised human session, disable login for the account or revoke its active sessions through an unaffected owner account. Reset the password and inspect audit events for the user and affected projects.
- For a project API key or preview credential, issue a replacement, update the intended consumer, verify it, then revoke the exposed credential. Do not log or copy the replacement into the incident record.
- For a webhook secret, revoke and recreate the endpoint, update the receiver atomically and replay only reviewed failed events. Rotate `SEOBLOG_WEBHOOK_ENCRYPTION_KEY` only through a separately tested migration because existing encrypted secrets depend on it.
- Search sanitized logs and audit data for use after the suspected exposure time. Escalate cross-project or publication access as a security incident.

## Harmful publication or emergency rollback

- Identify the exact published revision and preserve its audit/revision evidence.
- Use the normal article unpublish or exact-revision rollback workflow; do not edit publication tables directly.
- Confirm the Content API ETag/body, discovery/feed data and redirect behavior. Then verify landing/CDN removal or corrected rendering.
- If webhooks fail, follow the landing procedure below and communicate the partial propagation state. A previous application release is not a content rollback mechanism.

## Stale CDN or failed landing revalidation

- Confirm the authoritative Content API exposes the expected revision and inspect the outbox age, webhook attempt status, receiver response code/category and delivery signature clock.
- Pause repeated delivery if the receiver is causing harm. Fix receiver availability/authentication/idempotency, then use the admin replay action for only reviewed failed/dead-letter attempts.
- Purge or revalidate the exact affected landing paths through the landing operator's approved workflow. Do not purge an entire CDN zone unless separately authorized.
- Verify canonical HTML, JSON-LD, feed/discovery data and the visible landing page before resolving.

## Redis outage or cache poisoning

- Verify the API is serving authoritative SQLite fallbacks and inspect `seoblog_content_cache_events_total{outcome="fallback"}`, `redis_up`, latency, memory and evictions.
- For an outage, restart or fail over Redis under its host/managed-service procedure. Do not restore Redis from backup; it is disposable cache state.
- For suspected poisoning, stop Redis or flush only the dedicated SEO Blog keyspace after approval. The generation-scoped keys will refill from SQLite.
- Compare representative cached/uncached bodies and ETags before returning Redis to service.

## SQLite busy, corruption, disk-full or host loss

- Stop new worker claims first. For contention, capture connection/wait metrics and long operations; do not increase connection count or busy timeout during the incident without review.
- For disk pressure, preserve database/WAL/backup evidence and remove only known recoverable artifacts such as expired release bundles or rotated logs. Never delete the live DB, WAL or verified snapshot evidence.
- Run integrity checks only on an isolated restored/copy-safe database. Do not copy the live `.db` file alone or run repair commands against the primary.
- For corruption or host loss, follow [backup and recovery](backup-recovery.md). Keep webhook/email egress paused until restored pending work is reviewed.

## Failed migration or release

- Read `/srv/seoblog/shared/deployments.jsonl`, the deploy output, current/previous symlinks and migration status.
- Before activation, leave the current release untouched and correct the candidate/configuration.
- After activation, the deploy trap should restore the previous compatible symlink and the three named PM2 processes. Verify that state and run health checks; never improvise a down migration.
- If the schema is incompatible with the previous release, choose a tested forward fix or authorized database restoration. Use the pre-release recovery-point evidence and record the decision.

## PM2 daemon, process list, restart loop or reboot

- Run `pm2 status` and `pm2 prettylist` as the dedicated application user; check `$PM2_HOME`, `dump.pm2`, startup unit ownership and `seoblog_pm2_saved_process_present`.
- Restore from the current ecosystem declaration with the exact `--only` list, verify all three processes, then `pm2 save` as that same user.
- For a restart loop, stop only the affected named process and diagnose logs, exit code, memory, configuration and dependencies before restart.
- After reboot, verify Nginx and Redis under systemd, the PM2 startup service under the correct user, all loopback endpoints and a worker cycle.

## Graceful-shutdown timeout

- Stop deployments and new worker claims. Capture active jobs/leases and the process log around `SIGTERM`.
- Allow the declared API/admin/worker timeout. If forced termination is necessary, record it and leave the worker stopped until expired leases and idempotency are reviewed.
- Fix the blocking handler/provider timeout and test bounded shutdown in staging before another release.

## Nginx, TLS or domain incident

- Run `nginx -t` before every reload. If validation fails, retain/restore the last known config and do not reload.
- Check the loopback upstream directly to distinguish Nginx/TLS from application failure. Keep Redis, SQLite and PM2 ports private.
- For certificate/domain failure, preserve the last verified canonical/domain configuration, stop the migration, repair ownership/TLS/DNS, and verify old-domain redirects cannot loop.
- Reload gracefully only after validation; verify UI, admin API and Content API routing through the public hostname.

## Backup or primary-host recovery

- Page immediately when backup freshness exceeds RPO or latest verification is false. Check both backup timers and the last evidence line before retrying the verified job.
- Do not claim recovery readiness from upload success alone; the downloaded snapshot checksum and SQLite checks must pass.
- For restoration or host replacement, follow [backup and recovery](backup-recovery.md), including authorization, isolated restore, egress pause, pending-delivery review and recorded RPO/RTO.

## Outbox or dead-letter backlog

- Compare `seoblog_outbox_pending_events`, oldest age, worker cycle errors and webhook/email status gauges. Stop the worker if duplicate or harmful side effects are possible.
- Resolve the underlying database, endpoint, provider or clock issue. Inspect leases and next-attempt times; do not bulk-update queue tables.
- Resume the worker with a bounded batch, watch latency/failure rate and replay only admin-visible eligible records. Confirm newest publication converges correctly even if processing order changed.

## AI provider or cost incident

- Disable AI execution by removing the provider configuration and restarting only the worker if spend, privacy or output safety is uncertain. Manual editing/publication remains available.
- Preserve job/run provenance and inspect provider category, model, token and estimated-cost metrics. Never include unredacted prompts in incident chat/logs.
- For provider outage, leave retryable jobs bounded or cancel them through the product. For unexpected spend, revoke/limit the provider credential and obtain budget approval before re-enabling.
- Test the pinned prompt/model in staging before rollback or replacement.

## Transactional email outage

- Confirm SMTP/provider status, TLS, sender DNS and dead-letter notification state without testing account existence publicly.
- Pause retries if the provider rejects credentials or sender identity. Rotate the SMTP credential through protected configuration, restart API/worker, and send a controlled test.
- Use the safe resend flow after recovery; do not manually expose invitation/reset tokens. Provider bounce handling remains subject to the provider's operational console until native bounce ingestion exists.

## Completion checklist

- Customer/editor impact ended and authoritative content is correct.
- Required services, metrics targets, backup freshness and PM2 saved state are healthy.
- Paused worker/webhook/email paths were deliberately resumed or have named owners.
- Temporary credentials/configuration were rotated or removed.
- Incident timeline, evidence, approvals, actual RPO/RTO and corrective actions were recorded.
- A regression, alert or rehearsal update is filed when detection or recovery was insufficient.
