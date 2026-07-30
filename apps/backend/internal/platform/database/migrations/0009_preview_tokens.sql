CREATE TABLE preview_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    created_by TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, content_id, revision_id) REFERENCES content_revisions(project_id, content_id, id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE(project_id, id)
)
-- statement
CREATE INDEX idx_preview_tokens_hash_active
ON preview_tokens(token_hash, expires_at, revoked_at)
-- statement
CREATE INDEX idx_preview_tokens_project_revision
ON preview_tokens(project_id, revision_id, expires_at)
