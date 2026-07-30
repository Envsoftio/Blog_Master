CREATE TABLE ai_job_events (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK(sequence > 0),
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    progress INTEGER NOT NULL DEFAULT 0 CHECK(progress BETWEEN 0 AND 100),
    message TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, job_id) REFERENCES ai_jobs(project_id, id) ON DELETE CASCADE,
    UNIQUE(project_id, job_id, sequence)
)
-- statement
CREATE INDEX idx_ai_job_events_project_job_sequence
ON ai_job_events(project_id, job_id, sequence)
-- statement
CREATE INDEX idx_ai_runs_project_started
ON ai_runs(project_id, started_at, id)
-- statement
CREATE INDEX idx_quality_checks_project_revision_created
ON quality_check_results(project_id, revision_id, created_at, id)
-- statement
CREATE TRIGGER ai_jobs_scope_guard_insert
BEFORE INSERT ON ai_jobs
BEGIN
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'AI job content must belong to the project')
    END;
    SELECT CASE
        WHEN NEW.revision_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_revisions
            WHERE project_id = NEW.project_id
              AND id = NEW.revision_id
              AND (NEW.content_id IS NULL OR content_id = NEW.content_id)
         )
        THEN RAISE(ABORT, 'AI job revision must belong to the project and content')
    END;
END
-- statement
CREATE TRIGGER ai_jobs_scope_guard_update
BEFORE UPDATE OF project_id, content_id, revision_id ON ai_jobs
BEGIN
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'AI job content must belong to the project')
    END;
    SELECT CASE
        WHEN NEW.revision_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_revisions
            WHERE project_id = NEW.project_id
              AND id = NEW.revision_id
              AND (NEW.content_id IS NULL OR content_id = NEW.content_id)
         )
        THEN RAISE(ABORT, 'AI job revision must belong to the project and content')
    END;
END
-- statement
CREATE TRIGGER ai_runs_scope_guard_insert
BEFORE INSERT ON ai_runs
BEGIN
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'AI run content must belong to the project')
    END;
    SELECT CASE
        WHEN NEW.revision_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_revisions
            WHERE project_id = NEW.project_id
              AND id = NEW.revision_id
              AND (NEW.content_id IS NULL OR content_id = NEW.content_id)
         )
        THEN RAISE(ABORT, 'AI run revision must belong to the project and content')
    END;
END
-- statement
CREATE TRIGGER ai_runs_scope_guard_update
BEFORE UPDATE OF project_id, content_id, revision_id ON ai_runs
BEGIN
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'AI run content must belong to the project')
    END;
    SELECT CASE
        WHEN NEW.revision_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_revisions
            WHERE project_id = NEW.project_id
              AND id = NEW.revision_id
              AND (NEW.content_id IS NULL OR content_id = NEW.content_id)
         )
        THEN RAISE(ABORT, 'AI run revision must belong to the project and content')
    END;
END
-- statement
CREATE TRIGGER quality_checks_scope_guard_insert
BEFORE INSERT ON quality_check_results
BEGIN
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'quality check content must belong to the project')
    END;
    SELECT CASE
        WHEN NEW.revision_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_revisions
            WHERE project_id = NEW.project_id
              AND id = NEW.revision_id
              AND (NEW.content_id IS NULL OR content_id = NEW.content_id)
         )
        THEN RAISE(ABORT, 'quality check revision must belong to the project and content')
    END;
END
-- statement
CREATE TRIGGER quality_checks_scope_guard_update
BEFORE UPDATE OF project_id, content_id, revision_id ON quality_check_results
BEGIN
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'quality check content must belong to the project')
    END;
    SELECT CASE
        WHEN NEW.revision_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_revisions
            WHERE project_id = NEW.project_id
              AND id = NEW.revision_id
              AND (NEW.content_id IS NULL OR content_id = NEW.content_id)
         )
        THEN RAISE(ABORT, 'quality check revision must belong to the project and content')
    END;
END
