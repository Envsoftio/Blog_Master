ALTER TABLE ai_jobs ADD COLUMN article_type TEXT NOT NULL DEFAULT ''
-- statement
ALTER TABLE ai_jobs ADD COLUMN prompt_template_version TEXT NOT NULL DEFAULT ''
-- statement
ALTER TABLE ai_jobs ADD COLUMN voice_profile_id TEXT
-- statement
ALTER TABLE ai_jobs ADD COLUMN voice_profile_version INTEGER
-- statement
ALTER TABLE ai_jobs ADD COLUMN evidence_packet_id TEXT
-- statement
ALTER TABLE ai_jobs ADD COLUMN evidence_packet_version INTEGER
-- statement
ALTER TABLE ai_jobs ADD COLUMN input_json TEXT NOT NULL DEFAULT '{}'
-- statement
ALTER TABLE ai_jobs ADD COLUMN source_revision_hash TEXT NOT NULL DEFAULT ''
-- statement
CREATE INDEX idx_ai_jobs_project_input_hash
ON ai_jobs(project_id, input_hash, status)
-- statement
CREATE UNIQUE INDEX idx_ai_jobs_active_input_hash
ON ai_jobs(project_id, input_hash)
WHERE input_hash IS NOT NULL
  AND prompt_template_version <> ''
  AND status IN ('queued', 'running', 'needs_input')
-- statement
CREATE TRIGGER ai_jobs_context_guard_insert
BEFORE INSERT ON ai_jobs
WHEN NEW.input_hash IS NOT NULL
BEGIN
    SELECT CASE
        WHEN NEW.revision_id IS NULL
          OR NEW.article_type = ''
          OR NEW.prompt_template_version = ''
          OR NEW.voice_profile_id IS NULL
          OR NEW.voice_profile_version IS NULL
          OR NEW.evidence_packet_id IS NULL
          OR NEW.evidence_packet_version IS NULL
          OR NEW.source_revision_hash = ''
          OR json_valid(NEW.input_json) = 0
        THEN RAISE(ABORT, 'AI job context is incomplete')
    END;
    SELECT CASE
        WHEN NOT EXISTS (
            SELECT 1
            FROM content_revisions revision
            JOIN content_items content
              ON content.project_id = revision.project_id
             AND content.id = revision.content_id
            WHERE revision.project_id = NEW.project_id
              AND revision.content_id = NEW.content_id
              AND revision.id = NEW.revision_id
              AND revision.content_hash = NEW.source_revision_hash
              AND content.article_type = NEW.article_type
        )
        THEN RAISE(ABORT, 'AI job source revision does not match its content hash')
    END;
    SELECT CASE
        WHEN NOT EXISTS (
            SELECT 1
            FROM voice_profiles profile
            WHERE profile.project_id = NEW.project_id
              AND profile.id = NEW.voice_profile_id
              AND profile.version = NEW.voice_profile_version
        )
        THEN RAISE(ABORT, 'AI job voice profile does not belong to the project')
    END;
    SELECT CASE
        WHEN NOT EXISTS (
            SELECT 1
            FROM evidence_packets packet
            WHERE packet.project_id = NEW.project_id
              AND packet.id = NEW.evidence_packet_id
              AND packet.version = NEW.evidence_packet_version
              AND packet.content_id IS NEW.content_id
              AND packet.approved_at IS NOT NULL
              AND json_extract(packet.packet_json, '$.publicationRecommendation') = 'ready'
        )
        THEN RAISE(ABORT, 'AI job evidence packet is not approved for this content')
    END;
END
-- statement
CREATE TRIGGER ai_jobs_context_guard_update
BEFORE UPDATE OF
    project_id,
    content_id,
    revision_id,
    task_type,
    article_type,
    prompt_template_version,
    voice_profile_id,
    voice_profile_version,
    evidence_packet_id,
    evidence_packet_version,
    input_hash,
    input_json,
    source_revision_hash
ON ai_jobs
WHEN OLD.input_hash IS NULL AND NEW.input_hash IS NOT NULL
BEGIN
    SELECT CASE
        WHEN NEW.revision_id IS NULL
          OR NEW.article_type = ''
          OR NEW.prompt_template_version = ''
          OR NEW.voice_profile_id IS NULL
          OR NEW.voice_profile_version IS NULL
          OR NEW.evidence_packet_id IS NULL
          OR NEW.evidence_packet_version IS NULL
          OR NEW.source_revision_hash = ''
          OR json_valid(NEW.input_json) = 0
        THEN RAISE(ABORT, 'AI job context is incomplete')
    END;
    SELECT CASE
        WHEN NOT EXISTS (
            SELECT 1
            FROM content_revisions revision
            JOIN content_items content
              ON content.project_id = revision.project_id
             AND content.id = revision.content_id
            WHERE revision.project_id = NEW.project_id
              AND revision.content_id = NEW.content_id
              AND revision.id = NEW.revision_id
              AND revision.content_hash = NEW.source_revision_hash
              AND content.article_type = NEW.article_type
        )
        THEN RAISE(ABORT, 'AI job source revision does not match its content hash')
    END;
    SELECT CASE
        WHEN NOT EXISTS (
            SELECT 1
            FROM voice_profiles profile
            WHERE profile.project_id = NEW.project_id
              AND profile.id = NEW.voice_profile_id
              AND profile.version = NEW.voice_profile_version
        )
        THEN RAISE(ABORT, 'AI job voice profile does not belong to the project')
    END;
    SELECT CASE
        WHEN NOT EXISTS (
            SELECT 1
            FROM evidence_packets packet
            WHERE packet.project_id = NEW.project_id
              AND packet.id = NEW.evidence_packet_id
              AND packet.version = NEW.evidence_packet_version
              AND packet.content_id IS NEW.content_id
              AND packet.approved_at IS NOT NULL
              AND json_extract(packet.packet_json, '$.publicationRecommendation') = 'ready'
        )
        THEN RAISE(ABORT, 'AI job evidence packet is not approved for this content')
    END;
END
-- statement
CREATE TRIGGER ai_jobs_context_immutable
BEFORE UPDATE OF
    project_id,
    content_id,
    revision_id,
    task_type,
    article_type,
    prompt_template_version,
    voice_profile_id,
    voice_profile_version,
    evidence_packet_id,
    evidence_packet_version,
    input_hash,
    input_json,
    source_revision_hash
ON ai_jobs
WHEN OLD.input_hash IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'AI job input context is immutable');
END
