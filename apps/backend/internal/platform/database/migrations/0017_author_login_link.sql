ALTER TABLE authors ADD COLUMN login_user_id TEXT REFERENCES users(id) ON DELETE SET NULL;
-- statement
CREATE UNIQUE INDEX one_author_login_per_project
ON authors(project_id, login_user_id)
WHERE login_user_id IS NOT NULL;
-- statement
CREATE TRIGGER authors_login_member_insert_guard
BEFORE INSERT ON authors
WHEN NEW.login_user_id IS NOT NULL AND NOT EXISTS (
    SELECT 1
    FROM project_memberships membership
    JOIN users user ON user.id = membership.user_id
    WHERE membership.project_id = NEW.project_id
      AND membership.user_id = NEW.login_user_id
      AND membership.status IN ('active','invited')
      AND user.status IN ('active','invited')
)
BEGIN
    SELECT RAISE(ABORT, 'author login user must be an invited or active member of the same project');
END;
-- statement
CREATE TRIGGER authors_login_member_update_guard
BEFORE UPDATE OF project_id, login_user_id ON authors
WHEN NEW.login_user_id IS NOT NULL AND NOT EXISTS (
    SELECT 1
    FROM project_memberships membership
    JOIN users user ON user.id = membership.user_id
    WHERE membership.project_id = NEW.project_id
      AND membership.user_id = NEW.login_user_id
      AND membership.status IN ('active','invited')
      AND user.status IN ('active','invited')
)
BEGIN
    SELECT RAISE(ABORT, 'author login user must be an invited or active member of the same project');
END;
