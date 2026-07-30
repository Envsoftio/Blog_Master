ALTER TABLE assets ADD COLUMN status TEXT NOT NULL DEFAULT 'registered' CHECK(status IN ('registered','uploading','processing','ready','rejected','failed'));
-- statement
ALTER TABLE assets ADD COLUMN upload_expires_at TEXT;
-- statement
ALTER TABLE assets ADD COLUMN checksum_sha256 TEXT;
-- statement
ALTER TABLE assets ADD COLUMN scan_status TEXT NOT NULL DEFAULT 'pending' CHECK(scan_status IN ('pending','passed','failed','skipped'));
-- statement
ALTER TABLE assets ADD COLUMN scan_reason TEXT;
-- statement
CREATE INDEX assets_project_status_created_idx
ON assets(project_id, status, created_at DESC);
