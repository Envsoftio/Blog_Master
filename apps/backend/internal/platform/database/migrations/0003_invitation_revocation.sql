ALTER TABLE invitations ADD COLUMN revoked_at TEXT
-- statement
CREATE INDEX idx_invitations_pending_identity
ON invitations(project_id, email_normalized, accepted_at, revoked_at, expires_at)
