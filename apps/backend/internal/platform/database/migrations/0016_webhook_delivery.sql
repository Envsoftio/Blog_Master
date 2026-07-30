ALTER TABLE webhook_endpoints ADD COLUMN secret_ciphertext TEXT;
-- statement
UPDATE webhook_endpoints
SET events_json = '[]'
WHERE json_valid(events_json) = 0
   OR json_type(events_json) != 'array';
-- statement
UPDATE webhook_endpoints
SET events_json = '[]'
WHERE EXISTS (
    SELECT 1
    FROM json_each(webhook_endpoints.events_json)
    WHERE json_each.type != 'text'
       OR json_each.value NOT IN (
          'content.published',
          'content.updated',
          'content.unpublished',
          'content.restored',
          'content.slug_changed',
          'content.archived'
       )
);
-- statement
CREATE TRIGGER webhook_endpoints_events_valid_insert
BEFORE INSERT ON webhook_endpoints
WHEN CASE
    WHEN json_valid(NEW.events_json) = 0 THEN 1
    WHEN json_type(NEW.events_json) != 'array' THEN 1
    WHEN EXISTS (
        SELECT 1
        FROM json_each(NEW.events_json)
        WHERE json_each.type != 'text'
           OR json_each.value NOT IN (
              'content.published',
              'content.updated',
              'content.unpublished',
              'content.restored',
              'content.slug_changed',
              'content.archived'
           )
    ) THEN 1
    ELSE 0
END
BEGIN
    SELECT RAISE(ABORT, 'webhook endpoint events are invalid');
END;
-- statement
CREATE TRIGGER webhook_endpoints_events_valid_update
BEFORE UPDATE OF events_json ON webhook_endpoints
WHEN CASE
    WHEN json_valid(NEW.events_json) = 0 THEN 1
    WHEN json_type(NEW.events_json) != 'array' THEN 1
    WHEN EXISTS (
        SELECT 1
        FROM json_each(NEW.events_json)
        WHERE json_each.type != 'text'
           OR json_each.value NOT IN (
              'content.published',
              'content.updated',
              'content.unpublished',
              'content.restored',
              'content.slug_changed',
              'content.archived'
           )
    ) THEN 1
    ELSE 0
END
BEGIN
    SELECT RAISE(ABORT, 'webhook endpoint events are invalid');
END;
-- statement
ALTER TABLE outbox_events ADD COLUMN webhook_fanned_out_at TEXT;
-- statement
UPDATE outbox_events
SET webhook_fanned_out_at = CURRENT_TIMESTAMP;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN max_attempts INTEGER NOT NULL DEFAULT 5;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN next_attempt_at TEXT NOT NULL DEFAULT '';
-- statement
UPDATE webhook_attempts
SET next_attempt_at = attempted_at
WHERE next_attempt_at = '';
-- statement
ALTER TABLE webhook_attempts ADD COLUMN locked_by TEXT;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN locked_until TEXT;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN response_duration_ms INTEGER;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN last_error_safe_message TEXT;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN completed_at TEXT;
-- statement
ALTER TABLE webhook_attempts ADD COLUMN replay_of_attempt_id TEXT;
-- statement
UPDATE webhook_attempts
SET replay_of_attempt_id = (
    SELECT source.id
    FROM webhook_attempts source
    WHERE source.project_id = webhook_attempts.project_id
      AND source.endpoint_id = webhook_attempts.endpoint_id
      AND source.outbox_event_id = webhook_attempts.outbox_event_id
      AND source.rowid < webhook_attempts.rowid
    ORDER BY source.rowid
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1
    FROM webhook_attempts source
    WHERE source.project_id = webhook_attempts.project_id
      AND source.endpoint_id = webhook_attempts.endpoint_id
      AND source.outbox_event_id = webhook_attempts.outbox_event_id
      AND source.rowid < webhook_attempts.rowid
);
-- statement
CREATE TRIGGER webhook_attempts_status_valid_insert
BEFORE INSERT ON webhook_attempts
WHEN NEW.status NOT IN ('queued','processing','retrying','succeeded','failed','dead_letter','suppressed')
BEGIN
    SELECT RAISE(ABORT, 'webhook attempt status is invalid');
END;
-- statement
CREATE TRIGGER webhook_attempts_status_valid_update
BEFORE UPDATE OF status ON webhook_attempts
WHEN NEW.status NOT IN ('queued','processing','retrying','succeeded','failed','dead_letter','suppressed')
BEGIN
    SELECT RAISE(ABORT, 'webhook attempt status is invalid');
END;
-- statement
CREATE TRIGGER webhook_attempts_counters_valid_insert
BEFORE INSERT ON webhook_attempts
WHEN NEW.attempt_count < 0 OR NEW.max_attempts <= 0 OR NEW.attempt_count > NEW.max_attempts
BEGIN
    SELECT RAISE(ABORT, 'webhook attempt counters are invalid');
END;
-- statement
CREATE TRIGGER webhook_attempts_counters_valid_update
BEFORE UPDATE OF attempt_count, max_attempts ON webhook_attempts
WHEN NEW.attempt_count < 0 OR NEW.max_attempts <= 0 OR NEW.attempt_count > NEW.max_attempts
BEGIN
    SELECT RAISE(ABORT, 'webhook attempt counters are invalid');
END;
-- statement
CREATE TRIGGER webhook_attempts_replay_scope_insert
BEFORE INSERT ON webhook_attempts
WHEN NEW.replay_of_attempt_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM webhook_attempts source
    WHERE source.id = NEW.replay_of_attempt_id
      AND source.project_id = NEW.project_id
      AND source.endpoint_id = NEW.endpoint_id
      AND source.outbox_event_id = NEW.outbox_event_id
  )
BEGIN
    SELECT RAISE(ABORT, 'webhook replay scope is invalid');
END;
-- statement
CREATE UNIQUE INDEX idx_webhook_attempts_original_delivery
ON webhook_attempts(project_id, endpoint_id, outbox_event_id)
WHERE replay_of_attempt_id IS NULL;
-- statement
UPDATE webhook_attempts
SET status = 'suppressed',
    completed_at = CURRENT_TIMESTAMP
WHERE replay_of_attempt_id IS NOT NULL
  AND status IN ('queued','processing','retrying')
  AND EXISTS (
    SELECT 1
    FROM webhook_attempts earlier
    WHERE earlier.replay_of_attempt_id = webhook_attempts.replay_of_attempt_id
      AND earlier.status IN ('queued','processing','retrying')
      AND earlier.rowid < webhook_attempts.rowid
  );
-- statement
CREATE UNIQUE INDEX idx_webhook_attempts_active_replay
ON webhook_attempts(replay_of_attempt_id)
WHERE replay_of_attempt_id IS NOT NULL
  AND status IN ('queued','processing','retrying');
-- statement
CREATE INDEX idx_webhook_attempts_claim
ON webhook_attempts(status, next_attempt_at, locked_until, id);
-- statement
CREATE INDEX idx_outbox_webhook_fanout
ON outbox_events(webhook_fanned_out_at, available_at, created_at, id);
