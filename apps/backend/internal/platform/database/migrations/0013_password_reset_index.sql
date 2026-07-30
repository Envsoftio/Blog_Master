CREATE INDEX idx_password_resets_user_pending
ON password_resets(user_id, used_at, expires_at)
