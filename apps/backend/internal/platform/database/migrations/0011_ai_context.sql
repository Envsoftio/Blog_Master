CREATE INDEX idx_voice_profiles_project_version
ON voice_profiles(project_id, version DESC)
-- statement
CREATE INDEX idx_evidence_packets_scope_version
ON evidence_packets(project_id, COALESCE(content_id, ''), version)
-- statement
CREATE INDEX idx_evidence_packets_project_created
ON evidence_packets(project_id, created_at DESC, id DESC)
-- statement
CREATE TRIGGER voice_profiles_version_guard_insert
BEFORE INSERT ON voice_profiles
WHEN NEW.version <= 0
BEGIN
    SELECT RAISE(ABORT, 'voice profile version must be positive');
END
-- statement
CREATE TRIGGER voice_profiles_immutable
BEFORE UPDATE ON voice_profiles
BEGIN
    SELECT RAISE(ABORT, 'voice profile versions are immutable');
END
-- statement
CREATE TRIGGER evidence_packets_scope_guard_insert
BEFORE INSERT ON evidence_packets
BEGIN
    SELECT CASE
        WHEN NEW.version <= 0
        THEN RAISE(ABORT, 'evidence packet version must be positive')
    END;
    SELECT CASE
        WHEN NEW.content_id IS NOT NULL
         AND NOT EXISTS (
            SELECT 1
            FROM content_items
            WHERE project_id = NEW.project_id AND id = NEW.content_id
         )
        THEN RAISE(ABORT, 'evidence packet content must belong to the project')
    END;
END
-- statement
CREATE TRIGGER evidence_packets_version_unique_insert
BEFORE INSERT ON evidence_packets
WHEN EXISTS (
    SELECT 1
    FROM evidence_packets existing
    WHERE existing.project_id = NEW.project_id
      AND existing.content_id IS NEW.content_id
      AND existing.version = NEW.version
)
BEGIN
    SELECT RAISE(ABORT, 'evidence packet version already exists for this scope');
END
-- statement
CREATE TRIGGER evidence_packets_scope_guard_update
BEFORE UPDATE OF project_id, content_id ON evidence_packets
WHEN NEW.content_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1
    FROM content_items
    WHERE project_id = NEW.project_id AND id = NEW.content_id
 )
BEGIN
    SELECT RAISE(ABORT, 'evidence packet content must belong to the project');
END
-- statement
CREATE TRIGGER evidence_packets_content_immutable
BEFORE UPDATE OF project_id, content_id, version, packet_json, created_by, created_at
ON evidence_packets
BEGIN
    SELECT RAISE(ABORT, 'evidence packet content and version are immutable');
END
