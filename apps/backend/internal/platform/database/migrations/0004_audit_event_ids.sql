UPDATE audit_events
SET id = 'audit_' || lower(hex(randomblob(12)))
WHERE id IS NULL OR id = ''
-- statement
CREATE TRIGGER audit_events_assign_id
AFTER INSERT ON audit_events
WHEN NEW.id IS NULL OR NEW.id = ''
BEGIN
    UPDATE audit_events
    SET id = 'audit_' || lower(hex(randomblob(12)))
    WHERE rowid = NEW.rowid;
END
-- statement
CREATE TRIGGER audit_events_no_update
BEFORE UPDATE ON audit_events
WHEN OLD.id IS NOT NULL AND OLD.id <> ''
BEGIN
    SELECT RAISE(ABORT, 'audit events are append-only');
END
-- statement
CREATE TRIGGER audit_events_no_delete
BEFORE DELETE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit events are append-only');
END
-- statement
CREATE INDEX idx_audit_events_project_created
ON audit_events(project_id, created_at, id)
