ALTER TABLE webhook_endpoints ADD COLUMN events_json TEXT NOT NULL DEFAULT '[]'
-- statement
CREATE INDEX idx_assets_project_created
ON assets(project_id, created_at, id)
-- statement
CREATE INDEX idx_webhook_endpoints_project_status
ON webhook_endpoints(project_id, status, created_at)
