# Backup and recovery

This runbook covers the production SQLite database. Redis is disposable cache state and is rebuilt after recovery. Media recovery follows the independent B2 media bucket's version-history policy.

## Recovery contract

- Recovery point objective: five minutes. Litestream continuously ships committed SQLite changes to `continuous/<environment>/seoblog`.
- Initial recovery time objective: four hours. The target becomes one hour after two successful clean-host drills.
- A verified immutable snapshot is created daily and before every migration-bearing deployment. Monthly snapshots have a separately reviewed long retention.
- Continuous LTX objects and immutable database snapshots use different prefixes. Litestream has no remote-delete permission; reviewed B2 lifecycle rules own continuous-chain retention.
- The normal API, worker and media credentials have no access to either backup prefix.

The operations owner or incident commander may authorize a production restore. Every restore requires an incident/change ID and the authorizer's identity. Two people must review a compliance-mode Object Lock change, lifecycle reduction, or primary replacement.

## Provisioning

Create a private, environment-specific B2 backup bucket before uploading any object. Enable default SSE-B2 encryption and Object Lock, then test the retention mode and duration in a non-production bucket. Object Lock configuration is difficult to reverse; do not infer these values from this repository.

Create three bucket- or prefix-scoped application keys:

- continuous writer: list/write for only the continuous prefix, with no delete capability;
- snapshot writer: write plus retention-setting capability for only the snapshot prefix;
- restore reader: list/read plus retention-read capability for both prefixes, with no write or delete capability.

Pin Litestream v0.5.11, AWS CLI v2 and SQLite CLI on the host. Install the automation without starting it:

```bash
./scripts/setup-vps-cicd.sh --host HOST --user USER --configure-backups
```

Edit `/srv/seoblog/shared/backup.env`, preserving mode `0600`. Use the exact regional B2 endpoint and set `SEOBLOG_BACKUP_MONTHLY_RETENTION_DAYS` only to the approved legal/business retention period; it intentionally has no default because locked retention cannot be shortened casually. The checked-in configuration deliberately leaves remote retention enforcement to B2 and exposes Litestream metrics only on loopback.

Validate access and create the first verified recovery point:

```bash
sudo -u seoblog bash -c 'set -a; . /srv/seoblog/shared/backup.env; set +a; litestream databases -config /srv/seoblog/backup/litestream.yml'
sudo systemctl enable --now seoblog-litestream.service
sudo systemctl start seoblog-backup-verify.service
sudo systemctl enable --now seoblog-backup-verify.timer seoblog-backup-monthly.timer
sudo systemctl status seoblog-litestream.service seoblog-backup-verify.timer
sudo tail -n 1 /srv/seoblog/shared/backup-evidence/recovery-points.jsonl
```

Do not place secrets directly on a shell command line in production. The direct shell example above is only a configuration check; prefer `systemctl start` after reviewing the protected EnvironmentFile. Confirm the downloaded checksum, `quick_check`, foreign-key check and Object Lock evidence in the JSONL record. Then check that `/srv/seoblog/shared/.env` contains:

```bash
SEOBLOG_DEPLOY_BACKUP_COMMAND=/srv/seoblog/backup/create-recovery-point.sh pre-release
SEOBLOG_DEPLOY_REQUIRE_BACKUP=true
SEOBLOG_DEPLOY_SKIP_BACKUP=false
```

The deployment stops before migrations if that command fails. A required backup cannot be bypassed with the skip flag.

## Daily evidence and alerts

The daily timer forces Litestream to synchronously reach the current transaction, restores to an isolated file with the read-only credential, validates SQLite, uploads a separately locked snapshot, downloads it with the restore credential and validates it again. Evidence is appended to `backup-evidence/recovery-points.jsonl` without credentials.

Alert when the timer fails, Litestream is inactive, the last verified record is older than five minutes beyond its scheduled window, the replica sync transaction lags the primary, or disk space threatens SQLite/Litestream operation. Observability wiring is tracked in the next implementation slice.

## Isolated restore test

At least quarterly, provision a clean host with no database and separate restore credentials. Keep public DNS/CDN pointed at the healthy landing deployment, disable webhook and email egress, and do not start the worker. Restore into an isolated path:

```bash
export SEOBLOG_RESTORE_SERVICES_STOPPED=true
export SEOBLOG_RESTORE_CONFIRM=/srv/seoblog/shared/isolated-drill.db
/srv/seoblog/backup/restore-primary.sh \
  --continuous \
  --target /srv/seoblog/shared/isolated-drill.db \
  --authorized-by 'Operations owner' \
  --change-id DRILL-YYYY-QN
```

Run migrations from the intended release, start API/admin/worker against isolated dependencies, and verify readiness, sign-in, article reads, media references, scheduled jobs and pending outbox/webhook counts. Do not deliver restored outbox events until their idempotency and business validity are reviewed. Record start/end time, latest restored transaction time, RPO/RTO, release ID and exceptions. Destroy the isolated host only after evidence is retained.

## Primary recovery

1. Declare the incident, name an authorizer and record the change ID. Keep landing CDN content online where possible; put the CMS origin in maintenance mode and pause webhook/email egress.
2. Stop `seoblog-api`, `seoblog-worker` and `seoblog-litestream`. Confirm no process has the SQLite file open.
3. Select either the latest/point-in-time continuous restore or an immutable snapshot key. First restore to an isolated target and inspect it.
4. Set the two explicit guards and replace the primary. Existing database/WAL/SHM files are moved to a timestamped, recoverable `pre-restore-*` directory; they are never deleted by the script.

```bash
export SEOBLOG_RESTORE_SERVICES_STOPPED=true
export SEOBLOG_RESTORE_CONFIRM=/srv/seoblog/shared/seoblog.db
/srv/seoblog/backup/restore-primary.sh \
  --continuous --timestamp 2026-08-01T12:00:00Z \
  --target /srv/seoblog/shared/seoblog.db \
  --authorized-by 'Incident commander' \
  --change-id INC-1234 \
  --replace
```

5. Apply only forward-compatible migrations from the selected release. Start Litestream first, then API/admin, and keep the worker/webhook/email paths paused while reviewing restored outbox and scheduled work.
6. Verify `/readyz`, `/healthz`, admin SSR, representative Content API reads, audit history, media availability and queue counts. Resume outbound delivery deliberately, then record actual RPO/RTO and the final decision in `backup-evidence/restores.jsonl` and the incident system.

Never copy a live `.db` file directly, run a destructive down migration, overwrite the primary without the recoverable move, expose backup credentials to application processes, or shorten locked retention during an incident.
