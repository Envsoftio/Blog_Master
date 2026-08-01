CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- statement
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email_normalized TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    status TEXT NOT NULL DEFAULT 'invited' CHECK(status IN ('invited','active','disabled')),
    email_verified_at TEXT,
    password_changed_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- statement
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','suspended','archived','pending_deletion')),
    public_project_key TEXT NOT NULL UNIQUE,
    primary_domain TEXT,
    verified_domains_json TEXT NOT NULL DEFAULT '[]',
    blog_base_path TEXT NOT NULL DEFAULT '/blog',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    publisher_name TEXT,
    publisher_logo_asset_id TEXT,
    publisher_url TEXT,
    publisher_same_as_json TEXT NOT NULL DEFAULT '[]',
    default_author_id TEXT,
    default_social_image_id TEXT,
    seo_title_pattern TEXT,
    default_robots_policy TEXT NOT NULL DEFAULT 'index,follow',
    voice_profile_id TEXT,
    content_generation INTEGER NOT NULL DEFAULT 1,
    discovery_manifest_configuration TEXT NOT NULL DEFAULT '{}',
    feed_data_configuration TEXT NOT NULL DEFAULT '{}',
    landing_delivery_configuration TEXT NOT NULL DEFAULT '{}',
    solo_owner_approval_enabled INTEGER NOT NULL DEFAULT 0,
    created_by TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    archived_at TEXT,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    UNIQUE(workspace_id, slug)
);
-- statement
CREATE TABLE project_memberships (
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('project_owner','project_admin','editor','reviewer','writer')),
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('invited','active','removed')),
    invited_by TEXT,
    invited_at TEXT,
    joined_at TEXT,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    removed_at TEXT,
    PRIMARY KEY(project_id, user_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
);
-- statement
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    csrf_token_hash TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    idle_expires_at TEXT NOT NULL,
    absolute_expires_at TEXT NOT NULL,
    revoked_at TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
-- statement
CREATE TABLE invitations (
    token_hash TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    email_normalized TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('project_owner','project_admin','editor','reviewer','writer')),
    invited_by TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    accepted_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
-- statement
CREATE TABLE password_resets (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
-- statement
CREATE TABLE project_api_keys (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    environment TEXT NOT NULL CHECK(environment IN ('production','staging','development','preview')),
    name TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    scopes TEXT NOT NULL DEFAULT '[]',
    expires_at TEXT,
    last_used_at TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE authors (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    short_bio TEXT,
    full_bio TEXT,
    photo_asset_id TEXT,
    job_title TEXT,
    organization TEXT,
    credentials_json TEXT NOT NULL DEFAULT '[]',
    expertise_json TEXT NOT NULL DEFAULT '[]',
    profile_url TEXT,
    external_profiles_json TEXT NOT NULL DEFAULT '[]',
    same_as_json TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','inactive')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, slug),
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE taxonomy_terms (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('category','tag','topic')),
    parent_id TEXT,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    seo_json TEXT NOT NULL DEFAULT '{}',
    indexability TEXT NOT NULL DEFAULT 'index' CHECK(indexability IN ('index','noindex')),
    canonical_term_id TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','merged','archived')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, parent_id) REFERENCES taxonomy_terms(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, canonical_term_id) REFERENCES taxonomy_terms(project_id, id) ON DELETE SET NULL,
    UNIQUE(project_id, slug),
    UNIQUE(project_id, id)
);
-- statement
CREATE TRIGGER taxonomy_terms_no_self_parent
BEFORE INSERT ON taxonomy_terms
WHEN NEW.parent_id IS NOT NULL AND NEW.parent_id = NEW.id
BEGIN
    SELECT RAISE(ABORT, 'category cannot parent itself');
END;
-- statement
CREATE TRIGGER taxonomy_terms_update_no_self_parent
BEFORE UPDATE OF parent_id, type, project_id ON taxonomy_terms
WHEN NEW.parent_id IS NOT NULL
BEGIN
    SELECT CASE
        WHEN NEW.parent_id = NEW.id
        THEN RAISE(ABORT, 'category cannot parent itself')
    END;

    SELECT CASE
        WHEN NEW.type <> 'category' OR NOT EXISTS (
            SELECT 1
            FROM taxonomy_terms parent
            WHERE parent.project_id = NEW.project_id
              AND parent.id = NEW.parent_id
              AND parent.type = 'category'
        )
        THEN RAISE(ABORT, 'only categories may have category parents')
    END;

    SELECT CASE
        WHEN NEW.parent_id IN (
            WITH RECURSIVE descendants(id) AS (
                SELECT id
                FROM taxonomy_terms
                WHERE project_id = OLD.project_id
                  AND parent_id = OLD.id
                UNION
                SELECT child.id
                FROM taxonomy_terms child
                JOIN descendants parent ON parent.id = child.parent_id
                WHERE child.project_id = OLD.project_id
            )
            SELECT id FROM descendants
        )
        THEN RAISE(ABORT, 'category hierarchy cannot contain a cycle')
    END;

    SELECT CASE
        WHEN EXISTS (
            SELECT 1
            FROM taxonomy_terms parent
            JOIN taxonomy_terms grandparent
              ON grandparent.project_id = parent.project_id
             AND grandparent.id = parent.parent_id
            WHERE parent.project_id = NEW.project_id
              AND parent.id = NEW.parent_id
        ) AND EXISTS (
            SELECT 1
            FROM taxonomy_terms child
            WHERE child.project_id = OLD.project_id
              AND child.parent_id = OLD.id
        )
        THEN RAISE(ABORT, 'category hierarchy cannot exceed three levels')
    END;

    SELECT CASE
        WHEN EXISTS (
            SELECT 1
            FROM taxonomy_terms parent
            WHERE parent.project_id = NEW.project_id
              AND parent.id = NEW.parent_id
        ) AND EXISTS (
            SELECT 1
            FROM taxonomy_terms child
            JOIN taxonomy_terms grandchild
              ON grandchild.project_id = child.project_id
             AND grandchild.parent_id = child.id
            WHERE child.project_id = OLD.project_id
              AND child.parent_id = OLD.id
        )
        THEN RAISE(ABORT, 'category hierarchy cannot exceed three levels')
    END;
END;
-- statement
CREATE TRIGGER taxonomy_terms_parent_must_be_category
BEFORE INSERT ON taxonomy_terms
WHEN NEW.parent_id IS NOT NULL AND (
    NEW.type <> 'category' OR
    NOT EXISTS (
        SELECT 1 FROM taxonomy_terms parent
        WHERE parent.project_id = NEW.project_id
          AND parent.id = NEW.parent_id
          AND parent.type = 'category'
    )
)
BEGIN
    SELECT RAISE(ABORT, 'only categories may have category parents');
END;
-- statement
CREATE TRIGGER taxonomy_terms_max_three_levels
BEFORE INSERT ON taxonomy_terms
WHEN NEW.parent_id IS NOT NULL AND EXISTS (
    SELECT 1
    FROM taxonomy_terms parent
    JOIN taxonomy_terms grandparent
      ON grandparent.project_id = parent.project_id
     AND grandparent.id = parent.parent_id
    WHERE parent.project_id = NEW.project_id
      AND parent.id = NEW.parent_id
      AND grandparent.parent_id IS NOT NULL
)
BEGIN
    SELECT RAISE(ABORT, 'category hierarchy cannot exceed three levels');
END;
-- statement
CREATE TRIGGER taxonomy_terms_with_children_must_remain_category
BEFORE UPDATE OF type, project_id ON taxonomy_terms
WHEN NEW.type <> 'category' AND EXISTS (
    SELECT 1
    FROM taxonomy_terms child
    WHERE child.project_id = OLD.project_id
      AND child.parent_id = OLD.id
)
BEGIN
    SELECT RAISE(ABORT, 'a taxonomy term with children must remain a category');
END;
-- statement
CREATE TABLE series (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    seo_json TEXT NOT NULL DEFAULT '{}',
    indexability TEXT NOT NULL DEFAULT 'index',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, slug),
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE content_items (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    article_type TEXT NOT NULL CHECK(article_type IN ('standard','guide','tutorial','comparison','case_study','research','listicle','news_update','opinion','reference','glossary','release_note')),
    origin_project_id TEXT,
    origin_content_id TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    archived_at TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    UNIQUE(project_id, id)
);
-- statement
CREATE TRIGGER content_items_project_immutable
BEFORE UPDATE OF project_id ON content_items
WHEN NEW.project_id <> OLD.project_id
BEGIN
    SELECT RAISE(ABORT, 'article project_id is immutable');
END;
-- statement
CREATE TABLE content_revisions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    revision_number INTEGER NOT NULL,
    base_revision_id TEXT,
    created_by_type TEXT NOT NULL CHECK(created_by_type IN ('human','ai')),
    created_by_user_id TEXT,
    title TEXT NOT NULL,
    alternate_title TEXT,
    deck TEXT,
    excerpt TEXT,
    short_answer TEXT,
    body_document_json TEXT NOT NULL DEFAULT '{}',
    sanitized_html TEXT NOT NULL DEFAULT '',
    plain_text TEXT NOT NULL DEFAULT '',
    markdown_export TEXT NOT NULL DEFAULT '',
    table_of_contents_json TEXT NOT NULL DEFAULT '[]',
    word_count INTEGER NOT NULL DEFAULT 0,
    reading_time_seconds INTEGER NOT NULL DEFAULT 0,
    author_snapshot_json TEXT NOT NULL DEFAULT '[]',
    contributor_snapshot_json TEXT NOT NULL DEFAULT '[]',
    taxonomy_snapshot_json TEXT NOT NULL DEFAULT '{}',
    source_snapshot_json TEXT NOT NULL DEFAULT '[]',
    claim_snapshot_json TEXT NOT NULL DEFAULT '[]',
    seo_snapshot_json TEXT NOT NULL DEFAULT '{}',
    social_snapshot_json TEXT NOT NULL DEFAULT '{}',
    media_snapshot_json TEXT NOT NULL DEFAULT '{}',
    disclosure_snapshot_json TEXT NOT NULL DEFAULT '[]',
    correction_summary_json TEXT NOT NULL DEFAULT '[]',
    change_summary TEXT,
    content_hash TEXT NOT NULL,
    ai_assistance_level TEXT NOT NULL DEFAULT 'none',
    ai_provenance_summary_json TEXT NOT NULL DEFAULT '{}',
    editorial_state TEXT NOT NULL DEFAULT 'draft' CHECK(editorial_state IN ('draft','in_review','changes_requested','approved')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    UNIQUE(project_id, id),
    UNIQUE(project_id, content_id, id),
    UNIQUE(project_id, content_id, revision_number)
);
-- statement
CREATE TABLE project_publications (
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
CREATE TABLE article_taxonomy (
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    taxonomy_term_id TEXT NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK(is_primary IN (0, 1)),
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(project_id, content_id, taxonomy_term_id),
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, taxonomy_term_id) REFERENCES taxonomy_terms(project_id, id) ON DELETE RESTRICT
);
-- statement
CREATE UNIQUE INDEX one_primary_category_per_article
ON article_taxonomy(project_id, content_id)
WHERE is_primary = 1;
-- statement
CREATE TRIGGER article_taxonomy_primary_must_be_category_insert
BEFORE INSERT ON article_taxonomy
WHEN NEW.is_primary = 1 AND NOT EXISTS (
    SELECT 1
    FROM taxonomy_terms term
    WHERE term.project_id = NEW.project_id
      AND term.id = NEW.taxonomy_term_id
      AND term.type = 'category'
)
BEGIN
    SELECT RAISE(ABORT, 'primary taxonomy assignment must be a category');
END;
-- statement
CREATE TRIGGER article_taxonomy_primary_must_be_category_update
BEFORE UPDATE OF is_primary, taxonomy_term_id ON article_taxonomy
WHEN NEW.is_primary = 1 AND NOT EXISTS (
    SELECT 1
    FROM taxonomy_terms term
    WHERE term.project_id = NEW.project_id
      AND term.id = NEW.taxonomy_term_id
      AND term.type = 'category'
)
BEGIN
    SELECT RAISE(ABORT, 'primary taxonomy assignment must be a category');
END;
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
CREATE TABLE assets (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    storage_provider TEXT NOT NULL DEFAULT 'b2',
    bucket TEXT,
    object_key TEXT NOT NULL,
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    width INTEGER,
    height INTEGER,
    focal_point_json TEXT NOT NULL DEFAULT '{}',
    alt_text TEXT,
    decorative INTEGER NOT NULL DEFAULT 0,
    caption TEXT,
    creator_credit TEXT,
    license TEXT,
    source_type TEXT NOT NULL DEFAULT 'uploaded',
    provenance_json TEXT NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE asset_variants (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    asset_id TEXT NOT NULL,
    variant_name TEXT NOT NULL,
    object_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    byte_size INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, asset_id) REFERENCES assets(project_id, id) ON DELETE CASCADE,
    UNIQUE(project_id, asset_id, variant_name)
);
-- statement
CREATE TABLE revision_contributors (
    project_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    author_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('primary_author','co_author','editor','expert_reviewer','photographer','other')),
    position INTEGER NOT NULL DEFAULT 0,
    public_snapshot_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY(project_id, revision_id, author_id, role),
    FOREIGN KEY (project_id, revision_id) REFERENCES content_revisions(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, author_id) REFERENCES authors(project_id, id) ON DELETE RESTRICT
);
-- statement
CREATE TABLE series_articles (
    project_id TEXT NOT NULL,
    series_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    PRIMARY KEY(project_id, series_id, content_id),
    FOREIGN KEY (project_id, series_id) REFERENCES series(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    UNIQUE(project_id, series_id, position)
);
-- statement
CREATE TABLE content_relationships (
    project_id TEXT NOT NULL,
    source_content_id TEXT NOT NULL,
    target_content_id TEXT NOT NULL,
    relationship_type TEXT NOT NULL CHECK(relationship_type IN ('related','pillar','cluster','translation','canonical_original')),
    origin TEXT NOT NULL CHECK(origin IN ('manual','deterministic','imported')),
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(project_id, source_content_id, target_content_id, relationship_type),
    FOREIGN KEY (project_id, source_content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, target_content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE
);
-- statement
CREATE TABLE review_assignments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    revision_id TEXT,
    assigned_to TEXT NOT NULL,
    assignment_type TEXT NOT NULL CHECK(assignment_type IN ('editor','reviewer','sme')),
    due_at TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE
);
-- statement
CREATE TABLE review_comments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    revision_id TEXT,
    block_id TEXT,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','resolved','reopened')),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_by TEXT,
    resolved_at TEXT,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE
);
-- statement
CREATE TABLE approval_decisions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    decision TEXT NOT NULL CHECK(decision IN ('approved','changes_requested','rejected')),
    content_hash TEXT NOT NULL,
    decided_by TEXT NOT NULL,
    decided_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    note TEXT,
    self_approval INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, revision_id) REFERENCES content_revisions(project_id, id) ON DELETE CASCADE
);
-- statement
CREATE TABLE publication_metadata_revisions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    seo_json TEXT NOT NULL DEFAULT '{}',
    social_json TEXT NOT NULL DEFAULT '{}',
    metadata_hash TEXT NOT NULL,
    approved_by TEXT,
    approved_at TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE slug_redirects (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    source_path TEXT NOT NULL,
    target_path TEXT NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 301,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, source_path)
);
-- statement
CREATE TABLE disclosures (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    revision_id TEXT,
    disclosure_type TEXT NOT NULL,
    public_text TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE
);
-- statement
CREATE TABLE correction_notices (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT NOT NULL,
    affected_revision_id TEXT,
    public_note TEXT NOT NULL,
    corrected_by TEXT NOT NULL,
    corrected_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    supersedes_notice_id TEXT,
    FOREIGN KEY (project_id, content_id) REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, affected_revision_id) REFERENCES content_revisions(project_id, id) ON DELETE SET NULL
);
-- statement
CREATE TABLE sources (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    title TEXT NOT NULL,
    publisher TEXT,
    author TEXT,
    url TEXT,
    publication_date TEXT,
    accessed_at TEXT,
    source_type TEXT NOT NULL DEFAULT 'web',
    is_primary INTEGER NOT NULL DEFAULT 0,
    archived_copy_reference TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE voice_profiles (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    profile_json TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, version),
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE ai_briefs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT,
    article_type TEXT NOT NULL,
    brief_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
-- statement
CREATE TABLE evidence_packets (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT,
    version INTEGER NOT NULL,
    packet_json TEXT NOT NULL,
    approved_by TEXT,
    approved_at TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, content_id, version)
);
-- statement
CREATE TABLE claims (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    claim_text TEXT NOT NULL,
    block_id TEXT,
    importance TEXT NOT NULL DEFAULT 'normal',
    verification_state TEXT NOT NULL DEFAULT 'unverified',
    verified_by TEXT,
    verified_at TEXT,
    FOREIGN KEY (project_id, revision_id) REFERENCES content_revisions(project_id, id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE claim_sources (
    project_id TEXT NOT NULL,
    claim_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    PRIMARY KEY(project_id, claim_id, source_id),
    FOREIGN KEY (project_id, claim_id) REFERENCES claims(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, source_id) REFERENCES sources(project_id, id) ON DELETE RESTRICT
);
-- statement
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    job_type TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','succeeded','failed','dead_letter','cancelled')),
    priority INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    next_attempt_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_by TEXT,
    locked_until TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TEXT,
    completed_at TEXT,
    last_error_code TEXT,
    last_error_safe_message TEXT,
    trace_id TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id),
    UNIQUE(project_id, idempotency_key)
);
-- statement
CREATE TABLE ai_jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT,
    revision_id TEXT,
    task_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    provider TEXT,
    model_identifier TEXT,
    input_hash TEXT,
    output_hash TEXT,
    started_by TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT,
    error_category TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    estimated_cost_cents INTEGER,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE ai_runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT,
    revision_id TEXT,
    job_id TEXT,
    task_type TEXT NOT NULL,
    provider TEXT NOT NULL,
    model_identifier TEXT NOT NULL,
    prompt_template_version TEXT NOT NULL,
    voice_profile_version INTEGER,
    evidence_packet_version INTEGER,
    input_hash TEXT NOT NULL,
    output_hash TEXT,
    source_ids TEXT NOT NULL DEFAULT '[]',
    started_by TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT,
    status TEXT NOT NULL DEFAULT 'running',
    input_tokens INTEGER,
    output_tokens INTEGER,
    estimated_cost_cents INTEGER,
    error_category TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, job_id) REFERENCES ai_jobs(project_id, id) ON DELETE SET NULL
);
-- statement
CREATE TABLE quality_check_results (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    content_id TEXT,
    revision_id TEXT,
    check_type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK(severity IN ('info','warning','blocking','critical')),
    status TEXT NOT NULL CHECK(status IN ('passed','failed','overridden')),
    message TEXT NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '{}',
    override_reason TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
-- statement
CREATE TABLE webhook_endpoints (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id)
);
-- statement
CREATE TABLE webhook_attempts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    outbox_event_id TEXT NOT NULL,
    status TEXT NOT NULL,
    status_code INTEGER,
    error_category TEXT,
    attempted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id, endpoint_id) REFERENCES webhook_endpoints(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, outbox_event_id) REFERENCES outbox_events(project_id, id) ON DELETE CASCADE
);
-- statement
CREATE TABLE outbox_events (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    available_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, id),
    UNIQUE(project_id, idempotency_key)
);
-- statement
CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    actor_type TEXT NOT NULL,
    actor_id TEXT,
    action TEXT NOT NULL,
    target_type TEXT,
    target_id TEXT,
    outcome TEXT NOT NULL,
    request_id TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- statement
CREATE INDEX idx_projects_workspace ON projects(workspace_id, slug);
-- statement
CREATE INDEX idx_memberships_user ON project_memberships(user_id, status);
-- statement
CREATE INDEX idx_api_keys_hash ON project_api_keys(token_hash);
-- statement
CREATE INDEX idx_content_items_project_created ON content_items(project_id, created_at, id);
-- statement
CREATE INDEX idx_revisions_content ON content_revisions(project_id, content_id, revision_number);
-- statement
CREATE INDEX idx_publications_project_slug ON project_publications(project_id, slug, publication_state);
-- statement
CREATE INDEX idx_taxonomy_tree ON taxonomy_terms(project_id, type, parent_id, slug);
-- statement
CREATE INDEX idx_outbox_available ON outbox_events(processed_at, available_at, id);
-- statement
CREATE INDEX idx_jobs_claim ON jobs(status, next_attempt_at, priority, locked_until);
-- statement
CREATE INDEX idx_revision_contributors_author ON revision_contributors(project_id, author_id, revision_id);
-- statement
CREATE INDEX idx_ai_jobs_project_status ON ai_jobs(project_id, status, started_at);
