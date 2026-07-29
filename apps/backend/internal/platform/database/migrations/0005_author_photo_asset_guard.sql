CREATE TRIGGER authors_photo_asset_insert_guard
BEFORE INSERT ON authors
WHEN NEW.photo_asset_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1
    FROM assets
    WHERE project_id = NEW.project_id
      AND id = NEW.photo_asset_id
 )
BEGIN
    SELECT RAISE(ABORT, 'author photo asset must belong to the same project');
END
-- statement
CREATE TRIGGER assets_author_photo_delete_guard
BEFORE DELETE ON assets
WHEN EXISTS (
    SELECT 1
    FROM authors
    WHERE project_id = OLD.project_id
      AND photo_asset_id = OLD.id
)
BEGIN
    SELECT RAISE(ABORT, 'asset is used as an author photo');
END
-- statement
CREATE TRIGGER assets_author_photo_identity_update_guard
BEFORE UPDATE OF id, project_id ON assets
WHEN (NEW.id <> OLD.id OR NEW.project_id <> OLD.project_id)
 AND EXISTS (
    SELECT 1
    FROM authors
    WHERE project_id = OLD.project_id
      AND photo_asset_id = OLD.id
 )
BEGIN
    SELECT RAISE(ABORT, 'asset is used as an author photo');
END
-- statement
CREATE TRIGGER authors_photo_asset_update_guard
BEFORE UPDATE OF project_id, photo_asset_id ON authors
WHEN NEW.photo_asset_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1
    FROM assets
    WHERE project_id = NEW.project_id
      AND id = NEW.photo_asset_id
 )
BEGIN
    SELECT RAISE(ABORT, 'author photo asset must belong to the same project');
END
