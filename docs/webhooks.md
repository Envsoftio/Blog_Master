# Webhook Delivery Contract

Each active endpoint receives a JSON `POST` for its selected event types. Delivery starts from the durable SQLite outbox, and each endpoint has an independent leased delivery record with bounded retries.

## Request

The body has this shape:

```json
{
  "id": "event_...",
  "type": "content.published",
  "projectId": "project_...",
  "aggregateType": "content",
  "aggregateId": "article_...",
  "data": {},
  "createdAt": "2026-07-30 12:00:00"
}
```

Headers:

- `X-SEOBlog-Event-ID`: stable outbox event ID.
- `X-SEOBlog-Event-Type`: event type.
- `X-SEOBlog-Timestamp`: Unix timestamp generated for this delivery attempt.
- `X-SEOBlog-Signature`: `v1=` followed by a lowercase hexadecimal HMAC-SHA256.
- `Idempotency-Key`: stable outbox event ID.

The HMAC key is the signing secret shown once when the endpoint is created. The signed bytes are:

```text
timestamp + "." + event_id + "." + raw_request_body
```

Receivers should compare signatures in constant time, reject timestamps outside their accepted clock-skew window, and retain processed event IDs so duplicate deliveries are harmless. A manual replay intentionally retains the original event ID.

## Delivery Safety

Destinations must use HTTPS. The worker resolves and rejects private, loopback, link-local, multicast, metadata-service, and special-purpose addresses before every request and connection. Only `307` and `308` redirects are followed, with the same checks applied to every redirect.

Set `SEOBLOG_WEBHOOK_ALLOWED_HOSTS` to a comma-separated exact or `*.suffix` allowlist to restrict destinations. Staging blocks every webhook when this allowlist is empty.

Receiver timeouts, `408`, `425`, `429`, and `5xx` responses are retried with bounded backoff. Other `4xx` responses and destination-safety failures are terminal. Failed and dead-letter deliveries can be replayed from the project Integrations page.

Endpoints created before encrypted signing-secret storage was introduced must be revoked and recreated because their original one-time secret cannot be recovered. Migration intentionally marks historical outbox rows as already considered for webhook fan-out; delivery begins with events committed after the migration.
