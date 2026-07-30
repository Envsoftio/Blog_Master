CREATE TRIGGER review_assignments_status_valid_insert
BEFORE INSERT ON review_assignments
WHEN NEW.status NOT IN ('open','completed','cancelled')
BEGIN
    SELECT RAISE(ABORT, 'review assignment status is invalid');
END;
-- statement
CREATE TRIGGER review_assignments_status_valid_update
BEFORE UPDATE OF status ON review_assignments
WHEN NEW.status NOT IN ('open','completed','cancelled')
BEGIN
    SELECT RAISE(ABORT, 'review assignment status is invalid');
END;
-- statement
CREATE TRIGGER review_assignments_assignee_active_member_insert
BEFORE INSERT ON review_assignments
WHEN NOT EXISTS (
    SELECT 1
    FROM project_memberships membership
    WHERE membership.project_id = NEW.project_id
      AND membership.user_id = NEW.assigned_to
      AND membership.status = 'active'
)
BEGIN
    SELECT RAISE(ABORT, 'review assignment assignee must be an active project member');
END;
-- statement
CREATE TRIGGER review_assignments_assignee_active_member_update
BEFORE UPDATE OF project_id, assigned_to ON review_assignments
WHEN NOT EXISTS (
    SELECT 1
    FROM project_memberships membership
    WHERE membership.project_id = NEW.project_id
      AND membership.user_id = NEW.assigned_to
      AND membership.status = 'active'
)
BEGIN
    SELECT RAISE(ABORT, 'review assignment assignee must be an active project member');
END;
-- statement
CREATE TRIGGER review_assignments_revision_matches_article_insert
BEFORE INSERT ON review_assignments
WHEN NEW.revision_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM content_revisions revision
    WHERE revision.project_id = NEW.project_id
      AND revision.content_id = NEW.content_id
      AND revision.id = NEW.revision_id
)
BEGIN
    SELECT RAISE(ABORT, 'review assignment revision must belong to the project article');
END;
-- statement
CREATE TRIGGER review_assignments_revision_matches_article_update
BEFORE UPDATE OF project_id, content_id, revision_id ON review_assignments
WHEN NEW.revision_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM content_revisions revision
    WHERE revision.project_id = NEW.project_id
      AND revision.content_id = NEW.content_id
      AND revision.id = NEW.revision_id
)
BEGIN
    SELECT RAISE(ABORT, 'review assignment revision must belong to the project article');
END;
-- statement
CREATE UNIQUE INDEX idx_review_assignments_open_unique
ON review_assignments(project_id, content_id, IFNULL(revision_id, ''), assigned_to, assignment_type)
WHERE status = 'open';
-- statement
CREATE INDEX idx_review_assignments_project_content
ON review_assignments(project_id, content_id, id);
