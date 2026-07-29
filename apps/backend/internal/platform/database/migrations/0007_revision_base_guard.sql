CREATE TRIGGER content_revision_base_guard_insert
BEFORE INSERT ON content_revisions
WHEN NEW.base_revision_id IS NOT NULL AND NOT EXISTS (
    SELECT 1
    FROM content_revisions base
    WHERE base.project_id = NEW.project_id
      AND base.content_id = NEW.content_id
      AND base.id = NEW.base_revision_id
      AND base.revision_number < NEW.revision_number
)
BEGIN
    SELECT RAISE(ABORT, 'base revision must be an earlier revision of the same project article');
END;
-- statement
CREATE TRIGGER content_revision_base_guard_update
BEFORE UPDATE OF project_id, content_id, revision_number, base_revision_id ON content_revisions
WHEN NEW.base_revision_id IS NOT NULL AND NOT EXISTS (
    SELECT 1
    FROM content_revisions base
    WHERE base.project_id = NEW.project_id
      AND base.content_id = NEW.content_id
      AND base.id = NEW.base_revision_id
      AND base.revision_number < NEW.revision_number
)
BEGIN
    SELECT RAISE(ABORT, 'base revision must be an earlier revision of the same project article');
END;
-- statement
UPDATE content_revisions
SET base_revision_id = base_revision_id
WHERE base_revision_id IS NOT NULL;
-- statement
CREATE INDEX idx_revisions_base
ON content_revisions(project_id, content_id, base_revision_id);
-- statement
CREATE TRIGGER content_revision_lineage_immutable
BEFORE UPDATE OF id, project_id, content_id, revision_number, base_revision_id ON content_revisions
WHEN NEW.id <> OLD.id
  OR NEW.project_id <> OLD.project_id
  OR NEW.content_id <> OLD.content_id
  OR NEW.revision_number <> OLD.revision_number
  OR NEW.base_revision_id IS NOT OLD.base_revision_id
BEGIN
    SELECT RAISE(ABORT, 'revision identity and base lineage are immutable');
END;
-- statement
CREATE TRIGGER content_revision_delete_guard
BEFORE DELETE ON content_revisions
WHEN EXISTS (
    SELECT 1
    FROM content_items item
    WHERE item.project_id = OLD.project_id
      AND item.id = OLD.content_id
)
BEGIN
    SELECT RAISE(ABORT, 'revision history is immutable while its article exists');
END;
