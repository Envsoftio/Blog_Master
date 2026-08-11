# Observability operations

This runbook installs the bounded production-observability slice: structured Go service/request logs, private Prometheus-compatible application metrics, durable delivery gauges, host/Redis collection, PM2/backup/release evidence, a managed dashboard and alert rules. It deliberately does not expose a public metrics route or deploy a self-hosted monitoring stack.

## Telemetry boundaries

- The API serves `/metrics` on the same loopback-bound address as `/healthz`. Production Nginx has no `/metrics` location, so the route is not public.
- The worker serves metrics only at `SEOBLOG_WORKER_METRICS_ADDR`, which rejects non-loopback binds and defaults to `127.0.0.1:9092`.
- Application labels are bounded to service, environment, normalized route, HTTP status, cache outcome and worker component/outcome. Project, content, actor, endpoint and job identifiers are excluded from metrics.
- Request logs may contain safe project, user and API-key IDs for correlation. They never include request bodies, query strings, cookies, authorization headers or tokens.
- Durable gauges are collected from SQLite at scrape time. A scrape failure is observable as `up == 0`; it does not affect API readiness or worker delivery.

## Production installation

Install Grafana Alloy `v1.18.0` from its verified package before provisioning. The setup script validates the pinned version and installs configuration without enabling collection:

```bash
./scripts/setup-vps-cicd.sh \
  --host HOST \
  --user seoblog \
  --configure-observability
```

Edit `/srv/seoblog/shared/observability.env`, keep it mode `0600`, and replace every `.invalid` endpoint and blank credential. Use credentials that can only write metrics or logs to this environment. If Redis uses authentication, set its password here as well; do not place it in `alloy.config`.

The service user needs permission to read its own PM2 log directory. To ingest the selected system journal units, also grant the minimum distribution-specific journal group (`systemd-journal` or `adm`) and re-login/restart the service. Do not grant Alloy access to unrelated secret files.

Validate before enabling:

```bash
curl --fail --silent http://127.0.0.1:8080/metrics | head
curl --fail --silent http://127.0.0.1:9092/metrics | head
sudo systemctl start seoblog-observability-export.service
sudo cat /srv/seoblog/shared/metrics/seoblog.prom
sudo -u seoblog bash -c 'set -a; . /srv/seoblog/shared/observability.env; set +a; alloy validate /srv/seoblog/observability/alloy.config'
```

Import [the alert rules](../infra/observability/alerts.yml) into the managed Prometheus-compatible ruler and [the dashboard](../infra/observability/grafana-dashboard.json) into Grafana. Replace each relative `runbook_url` during import with the repository/runbook URL available to the on-call operator. Route `severity=critical` to the paging policy and `severity=warning` to the staffed operational channel. Test one synthetic warning and one page before production.

Then enable collection:

```bash
sudo systemctl enable --now seoblog-observability-export.timer seoblog-alloy.service
sudo systemctl status seoblog-alloy.service seoblog-observability-export.timer
sudo journalctl -u seoblog-alloy.service --since '-10 minutes'
```

The setup uses Alloy's embedded Unix and Redis exporters. Application and worker targets remain loopback-only. Alloy remote-write credentials stay in the protected environment file, and its local WAL is held under the systemd-managed `/var/lib/seoblog-alloy` state directory.

## Metric contract

| Area | Primary metrics |
|---|---|
| API | `seoblog_http_requests_total`, `seoblog_http_request_duration_seconds` |
| Cache | `seoblog_content_cache_events_total`, embedded `redis_*` metrics |
| SQLite | connection/wait counters and primary/WAL sizes under `seoblog_sqlite_*` |
| Delivery | pending/oldest outbox gauges, webhook status/retry/latency metrics |
| Worker | cycles, processed items and duration by scheduler/webhook/email/media/AI |
| Providers | durable media, AI and email-notification states plus estimated AI cost |
| Runtime | Go uptime/memory/goroutines; host CPU/memory/filesystem/systemd metrics |
| PM2 | required process health, restarts, memory and saved-list membership |
| Recovery | last verified recovery-point time/result and last deployment result |

Duplicate durable gauges are emitted by API and worker with different `service` labels. Dashboard and alert queries use `max` when they need the authoritative shared value.

## Alert maintenance

Thresholds in source control are production defaults, not silent provider-side edits. Review them after 30 days of representative traffic. A threshold change requires a pull request, `task observability:check`, rule validation with `promtool`, import into staging, and a recorded synthetic alert. Keep the two-minute publication-outbox warning aligned with `NFR-PUB-006`; only the critical paging threshold may be tuned through the agreed incident policy.

At least monthly, verify all targets report `up == 1`, the dashboard has no unexplained gaps, PM2 saved-list metrics are `1`, and the latest backup timestamp agrees with `backup-evidence/recovery-points.jsonl`. At least quarterly, include alerts in the clean-host recovery drill.

Use [the incident runbook](operations-runbook.md) for response and recovery.

## Source references

- [Grafana Alloy Unix exporter](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.unix/)
- [Grafana Alloy Redis exporter](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.redis/)
- [Grafana Alloy journal source](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.journal/)
- [Prometheus rule validation](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
