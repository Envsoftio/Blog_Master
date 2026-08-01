CREATE TABLE workspace_memberships (
    workspace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('workspace_owner','workspace_operator')),
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','removed')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    removed_at TEXT,
    PRIMARY KEY(workspace_id, user_id),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
);
-- statement
INSERT OR IGNORE INTO workspace_memberships(workspace_id, user_id, role, status)
SELECT DISTINCT project.workspace_id, membership.user_id, 'workspace_owner', 'active'
FROM projects project
JOIN project_memberships membership ON membership.project_id = project.id
WHERE membership.role = 'project_owner' AND membership.status = 'active';
-- statement
CREATE INDEX idx_workspace_memberships_user
ON workspace_memberships(user_id, status, workspace_id);
