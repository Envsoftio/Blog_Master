ALTER TABLE ai_jobs ADD COLUMN output_json TEXT NOT NULL DEFAULT '{}'
-- statement
ALTER TABLE ai_jobs ADD COLUMN locked_by TEXT
-- statement
ALTER TABLE ai_jobs ADD COLUMN locked_until TEXT
-- statement
ALTER TABLE ai_jobs ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0
-- statement
CREATE INDEX idx_ai_jobs_execution_queue
ON ai_jobs(status, locked_until, started_at, id)
