CREATE TABLE projects_without_workspace (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
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
    archived_at TEXT
)
-- statement
INSERT INTO projects_without_workspace (
    id, slug, name, status, public_project_key, primary_domain,
    verified_domains_json, blog_base_path, timezone, publisher_name,
    publisher_logo_asset_id, publisher_url, publisher_same_as_json,
    default_author_id, default_social_image_id, seo_title_pattern,
    default_robots_policy, voice_profile_id, content_generation,
    discovery_manifest_configuration, feed_data_configuration,
    landing_delivery_configuration, solo_owner_approval_enabled,
    created_by, created_at, updated_at, archived_at
)
SELECT
    id, slug, name, status, public_project_key, primary_domain,
    verified_domains_json, blog_base_path, timezone, publisher_name,
    publisher_logo_asset_id, publisher_url, publisher_same_as_json,
    default_author_id, default_social_image_id, seo_title_pattern,
    default_robots_policy, voice_profile_id, content_generation,
    discovery_manifest_configuration, feed_data_configuration,
    landing_delivery_configuration, solo_owner_approval_enabled,
    created_by, created_at, updated_at, archived_at
FROM projects
-- statement
DROP TABLE projects
-- statement
ALTER TABLE projects_without_workspace RENAME TO projects
-- statement
DROP TABLE IF EXISTS workspace_memberships
-- statement
DROP TABLE IF EXISTS workspaces
