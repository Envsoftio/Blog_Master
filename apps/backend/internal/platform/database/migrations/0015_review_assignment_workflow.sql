ALTER TABLE review_assignments ADD COLUMN closed_by TEXT;
-- statement
ALTER TABLE review_assignments ADD COLUMN closed_at TEXT;
-- statement
CREATE TRIGGER review_assignments_closure_valid_insert
BEFORE INSERT ON review_assignments
WHEN (NEW.status = 'open' AND (NEW.closed_by IS NOT NULL OR NEW.closed_at IS NOT NULL))
  OR (
      NEW.status IN ('completed','cancelled')
      AND (
          TRIM(COALESCE(NEW.closed_by, '')) = ''
          OR TRIM(COALESCE(NEW.closed_at, '')) = ''
      )
  )
BEGIN
    SELECT RAISE(ABORT, 'review assignment closure metadata is invalid');
END;
-- statement
CREATE TRIGGER review_assignments_closure_valid_update
BEFORE UPDATE OF status, closed_by, closed_at ON review_assignments
WHEN (NEW.status = 'open' AND (NEW.closed_by IS NOT NULL OR NEW.closed_at IS NOT NULL))
  OR (
      NEW.status IN ('completed','cancelled')
      AND (
          TRIM(COALESCE(NEW.closed_by, '')) = ''
          OR TRIM(COALESCE(NEW.closed_at, '')) = ''
      )
  )
BEGIN
    SELECT RAISE(ABORT, 'review assignment closure metadata is invalid');
END;
-- statement
CREATE TABLE review_assignment_notifications (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    assignment_id TEXT NOT NULL,
    recipient_user_id TEXT NOT NULL,
    recipient_email TEXT NOT NULL CHECK(LENGTH(TRIM(recipient_email)) > 0),
    status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','processing','retry','delivered','dead_letter','suppressed')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK(max_attempts > 0),
    next_attempt_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_by TEXT,
    locked_until TEXT,
    last_error_safe_message TEXT,
    delivered_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (assignment_id) REFERENCES review_assignments(id) ON DELETE CASCADE,
    FOREIGN KEY (recipient_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE(assignment_id, recipient_user_id)
);
-- statement
CREATE TRIGGER review_assignment_notifications_scope_insert
BEFORE INSERT ON review_assignment_notifications
WHEN NOT EXISTS (
    SELECT 1
    FROM review_assignments assignment
    WHERE assignment.id = NEW.assignment_id
      AND assignment.project_id = NEW.project_id
      AND assignment.assigned_to = NEW.recipient_user_id
)
BEGIN
    SELECT RAISE(ABORT, 'review assignment notification scope is invalid');
END;
-- statement
CREATE TRIGGER review_assignment_notifications_scope_update
BEFORE UPDATE OF project_id, assignment_id, recipient_user_id ON review_assignment_notifications
WHEN NOT EXISTS (
    SELECT 1
    FROM review_assignments assignment
    WHERE assignment.id = NEW.assignment_id
      AND assignment.project_id = NEW.project_id
      AND assignment.assigned_to = NEW.recipient_user_id
)
BEGIN
    SELECT RAISE(ABORT, 'review assignment notification scope is invalid');
END;
-- statement
CREATE INDEX idx_review_assignment_notifications_claim
ON review_assignment_notifications(status, next_attempt_at, locked_until, id);
-- statement
DROP INDEX idx_review_assignments_project_content;
-- statement
CREATE INDEX idx_review_assignments_project_content_created
ON review_assignments(project_id, content_id, created_at DESC, id DESC);
