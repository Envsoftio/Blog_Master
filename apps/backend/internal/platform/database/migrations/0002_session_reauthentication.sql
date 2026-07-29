ALTER TABLE sessions ADD COLUMN reauthenticated_at TEXT
-- statement
UPDATE sessions
SET reauthenticated_at = created_at
WHERE reauthenticated_at IS NULL
