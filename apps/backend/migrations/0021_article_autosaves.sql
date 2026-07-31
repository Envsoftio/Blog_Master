CREATE TABLE article_autosaves (
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    base_revision_id TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
    draft_json TEXT NOT NULL CHECK(json_valid(draft_json) AND json_type(draft_json) = 'object'),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(project_id, content_id, user_id),
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, content_id, base_revision_id) REFERENCES content_revisions(project_id, content_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
-- statement
CREATE INDEX idx_article_autosaves_user_updated
ON article_autosaves(user_id, updated_at DESC);
