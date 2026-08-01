-- Keep an unconstrained, queryable copy of every pre-migration row. CREATE TABLE
-- AS SELECT intentionally preserves the legacy locale column when it exists,
-- while also allowing this migration to run on newly-created locale-free DBs.
CREATE TABLE project_publications_locale_archive AS
SELECT * FROM project_publications;
-- statement
CREATE TABLE project_publications_without_locale (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    canonical_url TEXT NOT NULL,
    canonical_policy TEXT NOT NULL DEFAULT 'self',
    published_revision_id TEXT,
    publication_state TEXT NOT NULL DEFAULT 'unpublished' CHECK(publication_state IN ('unpublished','scheduled','published','archived')),
    scheduled_for_utc TEXT,
    first_published_at TEXT,
    materially_modified_at TEXT,
    review_due_at TEXT,
    content_expires_at TEXT,
    unpublished_at TEXT,
    retired_at TEXT,
    replacement_url TEXT,
    publication_version INTEGER NOT NULL DEFAULT 1,
    robots_directive TEXT NOT NULL DEFAULT 'index,follow',
    draft_seo_overrides_json TEXT NOT NULL DEFAULT '{}',
    draft_social_overrides_json TEXT NOT NULL DEFAULT '{}',
    published_metadata_revision_id TEXT,
    published_render_snapshot_hash TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, content_id, published_revision_id) REFERENCES content_revisions(project_id, content_id, id),
    UNIQUE(project_id, content_id),
    UNIQUE(project_id, slug)
);
-- statement
INSERT INTO project_publications_without_locale (
    id, project_id, content_id, slug, canonical_url, canonical_policy,
    published_revision_id, publication_state, scheduled_for_utc,
    first_published_at, materially_modified_at, review_due_at,
    content_expires_at, unpublished_at, retired_at, replacement_url,
    publication_version, robots_directive, draft_seo_overrides_json,
    draft_social_overrides_json, published_metadata_revision_id,
    published_render_snapshot_hash, created_at, updated_at
)
WITH ranked_by_content AS (
    SELECT
        publication.*,
        ROW_NUMBER() OVER (
            PARTITION BY project_id, content_id
            ORDER BY
                CASE publication_state
                    WHEN 'published' THEN 4
                    WHEN 'scheduled' THEN 3
                    WHEN 'unpublished' THEN 2
                    ELSE 1
                END DESC,
                CASE WHEN first_published_at IS NULL THEN 0 ELSE 1 END DESC,
                updated_at DESC,
                id ASC
        ) AS content_rank
    FROM project_publications publication
),
one_per_content AS (
    SELECT *
    FROM ranked_by_content
    WHERE content_rank = 1
),
ranked_by_slug AS (
    SELECT
        publication.*,
        ROW_NUMBER() OVER (
            PARTITION BY project_id, slug
            ORDER BY
                CASE publication_state
                    WHEN 'published' THEN 4
                    WHEN 'scheduled' THEN 3
                    WHEN 'unpublished' THEN 2
                    ELSE 1
                END DESC,
                updated_at DESC,
                id ASC
        ) AS slug_rank
    FROM one_per_content publication
)
SELECT
    id,
    project_id,
    content_id,
    CASE
        WHEN slug_rank = 1 THEN slug
        ELSE 'legacy-locale-' || lower(hex(id))
    END,
    canonical_url,
    canonical_policy,
    published_revision_id,
    publication_state,
    scheduled_for_utc,
    first_published_at,
    materially_modified_at,
    review_due_at,
    content_expires_at,
    unpublished_at,
    retired_at,
    replacement_url,
    publication_version,
    robots_directive,
    draft_seo_overrides_json,
    draft_social_overrides_json,
    published_metadata_revision_id,
    published_render_snapshot_hash,
    created_at,
    updated_at
FROM ranked_by_slug;
-- statement
DROP TABLE project_publications;
-- statement
ALTER TABLE project_publications_without_locale RENAME TO project_publications;
-- statement
CREATE TRIGGER publication_requires_approved_revision_insert
BEFORE INSERT ON project_publications
WHEN NEW.publication_state = 'published' AND (
    NEW.published_revision_id IS NULL OR
    NOT EXISTS (
        SELECT 1
        FROM content_revisions revision
        WHERE revision.project_id = NEW.project_id
          AND revision.content_id = NEW.content_id
          AND revision.id = NEW.published_revision_id
          AND revision.editorial_state = 'approved'
    ) OR
    1 <> (
        SELECT COUNT(*)
        FROM article_taxonomy assignment
        JOIN taxonomy_terms term
          ON term.project_id = assignment.project_id
         AND term.id = assignment.taxonomy_term_id
        WHERE assignment.project_id = NEW.project_id
          AND assignment.content_id = NEW.content_id
          AND assignment.is_primary = 1
          AND term.type = 'category'
    )
)
BEGIN
    SELECT RAISE(ABORT, 'publication requires an approved revision and exactly one primary category');
END;
-- statement
CREATE TRIGGER publication_requires_approved_revision_update
BEFORE UPDATE OF publication_state, published_revision_id ON project_publications
WHEN NEW.publication_state = 'published' AND (
    NEW.published_revision_id IS NULL OR
    NOT EXISTS (
        SELECT 1
        FROM content_revisions revision
        WHERE revision.project_id = NEW.project_id
          AND revision.content_id = NEW.content_id
          AND revision.id = NEW.published_revision_id
          AND revision.editorial_state = 'approved'
    ) OR
    1 <> (
        SELECT COUNT(*)
        FROM article_taxonomy assignment
        JOIN taxonomy_terms term
          ON term.project_id = assignment.project_id
         AND term.id = assignment.taxonomy_term_id
        WHERE assignment.project_id = NEW.project_id
          AND assignment.content_id = NEW.content_id
          AND assignment.is_primary = 1
          AND term.type = 'category'
    )
)
BEGIN
    SELECT RAISE(ABORT, 'publication requires an approved revision and exactly one primary category');
END;
-- statement
CREATE INDEX idx_publications_project_slug
ON project_publications(project_id, slug, publication_state);
