-- Members are a product-wide directory. Preserve the strongest existing role and
-- make every known member available in every project.
INSERT OR IGNORE INTO project_memberships(
    project_id, user_id, role, status, invited_by, invited_at, joined_at, updated_at, removed_at
)
SELECT
    project.id,
    canonical.user_id,
    canonical.role,
    canonical.status,
    canonical.invited_by,
    canonical.invited_at,
    canonical.joined_at,
    CURRENT_TIMESTAMP,
    CASE WHEN canonical.status = 'removed' THEN canonical.removed_at END
FROM projects project
CROSS JOIN (
    SELECT
        membership.user_id,
        CASE MIN(CASE membership.role
            WHEN 'project_owner' THEN 1
            WHEN 'project_admin' THEN 2
            WHEN 'editor' THEN 3
            WHEN 'reviewer' THEN 4
            ELSE 5
        END)
            WHEN 1 THEN 'project_owner'
            WHEN 2 THEN 'project_admin'
            WHEN 3 THEN 'editor'
            WHEN 4 THEN 'reviewer'
            ELSE 'writer'
        END AS role,
        CASE
            WHEN SUM(CASE WHEN membership.status = 'active' THEN 1 ELSE 0 END) > 0 THEN 'active'
            WHEN SUM(CASE WHEN membership.status = 'invited' THEN 1 ELSE 0 END) > 0 THEN 'invited'
            ELSE 'removed'
        END AS status,
        MAX(membership.invited_by) AS invited_by,
        MIN(membership.invited_at) AS invited_at,
        MIN(membership.joined_at) AS joined_at,
        MAX(membership.removed_at) AS removed_at
    FROM project_memberships membership
    GROUP BY membership.user_id
) canonical
-- statement
CREATE TEMP TABLE global_member_directory_snapshot AS
SELECT
    membership.user_id,
    CASE MIN(CASE membership.role
        WHEN 'project_owner' THEN 1
        WHEN 'project_admin' THEN 2
        WHEN 'editor' THEN 3
        WHEN 'reviewer' THEN 4
        ELSE 5
    END)
        WHEN 1 THEN 'project_owner'
        WHEN 2 THEN 'project_admin'
        WHEN 3 THEN 'editor'
        WHEN 4 THEN 'reviewer'
        ELSE 'writer'
    END AS role,
    CASE
        WHEN SUM(CASE WHEN membership.status = 'active' THEN 1 ELSE 0 END) > 0 THEN 'active'
        WHEN SUM(CASE WHEN membership.status = 'invited' THEN 1 ELSE 0 END) > 0 THEN 'invited'
        ELSE 'removed'
    END AS status
FROM project_memberships membership
GROUP BY membership.user_id
-- statement
UPDATE project_memberships
SET role = (SELECT snapshot.role FROM global_member_directory_snapshot snapshot WHERE snapshot.user_id = project_memberships.user_id),
    status = (SELECT snapshot.status FROM global_member_directory_snapshot snapshot WHERE snapshot.user_id = project_memberships.user_id),
    joined_at = CASE
        WHEN (SELECT snapshot.status FROM global_member_directory_snapshot snapshot WHERE snapshot.user_id = project_memberships.user_id) = 'active'
        THEN COALESCE(joined_at, CURRENT_TIMESTAMP)
        ELSE joined_at
    END,
    removed_at = CASE
        WHEN (SELECT snapshot.status FROM global_member_directory_snapshot snapshot WHERE snapshot.user_id = project_memberships.user_id) = 'removed'
        THEN COALESCE(removed_at, CURRENT_TIMESTAMP)
        ELSE NULL
    END,
    updated_at = CURRENT_TIMESTAMP
-- statement
DROP TABLE global_member_directory_snapshot
-- statement
-- Resolve legacy per-project author collisions before enforcing one shared directory.
UPDATE authors
SET slug = slug || '-' || substr(id, MAX(length(id) - 5, 1))
WHERE EXISTS (
    SELECT 1
    FROM authors earlier
    WHERE earlier.slug = authors.slug
      AND (earlier.created_at < authors.created_at OR (earlier.created_at = authors.created_at AND earlier.id < authors.id))
)
-- statement
UPDATE authors
SET login_user_id = NULL
WHERE login_user_id IS NOT NULL
  AND EXISTS (
    SELECT 1
    FROM authors earlier
    WHERE earlier.login_user_id = authors.login_user_id
      AND (earlier.created_at < authors.created_at OR (earlier.created_at = authors.created_at AND earlier.id < authors.id))
  )
-- statement
DROP INDEX IF EXISTS one_author_login_per_project
-- statement
CREATE UNIQUE INDEX one_author_login_global
ON authors(login_user_id)
WHERE login_user_id IS NOT NULL
-- statement
CREATE UNIQUE INDEX authors_slug_global ON authors(slug)
-- statement
DROP TRIGGER IF EXISTS authors_login_member_insert_guard
-- statement
DROP TRIGGER IF EXISTS authors_login_member_update_guard
-- statement
CREATE TRIGGER authors_login_member_insert_guard
BEFORE INSERT ON authors
WHEN NEW.login_user_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1
    FROM project_memberships membership
    JOIN users user ON user.id = membership.user_id
    WHERE membership.user_id = NEW.login_user_id
      AND membership.status IN ('active','invited')
      AND user.status IN ('active','invited')
 )
BEGIN
    SELECT RAISE(ABORT, 'author login user must be an invited or active member');
END
-- statement
CREATE TRIGGER authors_login_member_update_guard
BEFORE UPDATE OF login_user_id ON authors
WHEN NEW.login_user_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1
    FROM project_memberships membership
    JOIN users user ON user.id = membership.user_id
    WHERE membership.user_id = NEW.login_user_id
      AND membership.status IN ('active','invited')
      AND user.status IN ('active','invited')
 )
BEGIN
    SELECT RAISE(ABORT, 'author login user must be an invited or active member');
END
-- statement
DROP TRIGGER IF EXISTS authors_photo_asset_insert_guard
-- statement
DROP TRIGGER IF EXISTS authors_photo_asset_update_guard
-- statement
DROP TRIGGER IF EXISTS assets_author_photo_delete_guard
-- statement
DROP TRIGGER IF EXISTS assets_author_photo_identity_update_guard
-- statement
CREATE TRIGGER authors_photo_asset_insert_guard
BEFORE INSERT ON authors
WHEN NEW.photo_asset_id IS NOT NULL
 AND NOT EXISTS (SELECT 1 FROM assets WHERE id = NEW.photo_asset_id)
BEGIN
    SELECT RAISE(ABORT, 'author photo asset must exist');
END
-- statement
CREATE TRIGGER authors_photo_asset_update_guard
BEFORE UPDATE OF photo_asset_id ON authors
WHEN NEW.photo_asset_id IS NOT NULL
 AND NOT EXISTS (SELECT 1 FROM assets WHERE id = NEW.photo_asset_id)
BEGIN
    SELECT RAISE(ABORT, 'author photo asset must exist');
END
-- statement
CREATE TRIGGER assets_author_photo_delete_guard
BEFORE DELETE ON assets
WHEN EXISTS (SELECT 1 FROM authors WHERE photo_asset_id = OLD.id)
BEGIN
    SELECT RAISE(ABORT, 'asset is used as an author photo');
END
-- statement
CREATE TRIGGER assets_author_photo_identity_update_guard
BEFORE UPDATE OF id ON assets
WHEN NEW.id <> OLD.id
 AND EXISTS (SELECT 1 FROM authors WHERE photo_asset_id = OLD.id)
BEGIN
    SELECT RAISE(ABORT, 'asset is used as an author photo');
END
-- statement
-- Contributors remain project content, but their author reference is now global.
CREATE TABLE revision_contributors_global_author (
    project_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    author_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('primary_author','co_author','editor','expert_reviewer','photographer','other')),
    position INTEGER NOT NULL DEFAULT 0,
    public_snapshot_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY(project_id, revision_id, author_id, role),
    FOREIGN KEY (project_id, revision_id) REFERENCES content_revisions(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (author_id) REFERENCES authors(id) ON DELETE RESTRICT
)
-- statement
INSERT INTO revision_contributors_global_author(
    project_id, revision_id, author_id, role, position, public_snapshot_json
)
SELECT project_id, revision_id, author_id, role, position, public_snapshot_json
FROM revision_contributors
-- statement
DROP TABLE revision_contributors
-- statement
ALTER TABLE revision_contributors_global_author RENAME TO revision_contributors
-- statement
CREATE INDEX idx_revision_contributors_author ON revision_contributors(author_id, project_id, revision_id)
