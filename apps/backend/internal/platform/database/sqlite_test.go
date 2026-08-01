package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func legacyLocaleInitialMigration(t *testing.T) string {
	t.Helper()
	legacy := initialMigration
	replacements := [][2]string{
		{
			"CREATE TABLE project_publications (\n    id TEXT PRIMARY KEY,\n    project_id TEXT NOT NULL,\n    content_id TEXT NOT NULL,\n    slug TEXT NOT NULL,",
			"CREATE TABLE project_publications (\n    id TEXT PRIMARY KEY,\n    project_id TEXT NOT NULL,\n    content_id TEXT NOT NULL,\n    locale TEXT NOT NULL,\n    slug TEXT NOT NULL,",
		},
		{
			"    UNIQUE(project_id, content_id),\n    UNIQUE(project_id, slug)\n);\n-- statement\nCREATE TABLE article_taxonomy",
			"    UNIQUE(project_id, content_id, locale),\n    UNIQUE(project_id, locale, slug)\n);\n-- statement\nCREATE TABLE article_taxonomy",
		},
		{
			"CREATE INDEX idx_publications_project_slug ON project_publications(project_id, slug, publication_state);",
			"CREATE INDEX idx_publications_project_slug ON project_publications(project_id, locale, slug, publication_state);",
		},
	}
	for _, replacement := range replacements {
		if strings.Count(legacy, replacement[0]) != 1 {
			t.Fatalf("legacy publication fixture expected one occurrence of %q", replacement[0])
		}
		legacy = strings.Replace(legacy, replacement[0], replacement[1], 1)
	}
	return legacy
}

func seedLegacyLocalePublication(t *testing.T, db *sql.DB, id, contentID, locale, slug, updatedAt string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO project_publications(
		  id, project_id, content_id, locale, slug, canonical_url,
		  publication_state, publication_version, robots_directive,
		  draft_seo_overrides_json, draft_social_overrides_json, updated_at
		) VALUES (?, 'project-a', ?, ?, ?, ?, 'unpublished', 7, 'noindex,follow', '{"title":"draft"}', '{"image":"draft.png"}', ?)
	`, id, contentID, locale, slug, "https://example.test/blog/"+slug, updatedAt); err != nil {
		t.Fatal(err)
	}
}

func TestProjectPublicationMigrationRepairsLegacyLocaleConstraint(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy-publication.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(db, migration{version: "0001_initial", statements: legacyLocaleInitialMigration(t)}); err != nil {
		t.Fatal(err)
	}
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES ('article', 'project-a', 'guide', 'owner');
	`); err != nil {
		t.Fatal(err)
	}
	seedLegacyLocalePublication(t, db, "publication", "article", "en", "legacy-guide", "2026-01-01 00:00:00")

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var localeColumns int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM pragma_table_info('project_publications')
		WHERE name = 'locale'
	`).Scan(&localeColumns); err != nil {
		t.Fatal(err)
	}
	if localeColumns != 0 {
		t.Fatal("expected the legacy publication locale column to be removed")
	}
	var archivedRows, archivedLocaleColumns int
	if err := db.QueryRow(`SELECT COUNT(1) FROM project_publications_locale_archive`).Scan(&archivedRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM pragma_table_info('project_publications_locale_archive')
		WHERE name = 'locale'
	`).Scan(&archivedLocaleColumns); err != nil {
		t.Fatal(err)
	}
	if archivedRows != 1 || archivedLocaleColumns != 1 {
		t.Fatalf("expected a locale-preserving archive row, rows=%d localeColumns=%d", archivedRows, archivedLocaleColumns)
	}
	var version int64
	var robots, seoOverrides, socialOverrides string
	if err := db.QueryRow(`
		SELECT publication_version, robots_directive,
		       draft_seo_overrides_json, draft_social_overrides_json
		FROM project_publications
		WHERE id = 'publication'
	`).Scan(&version, &robots, &seoOverrides, &socialOverrides); err != nil {
		t.Fatal(err)
	}
	if version != 7 || robots != "noindex,follow" || seoOverrides != `{"title":"draft"}` || socialOverrides != `{"image":"draft.png"}` {
		t.Fatalf("legacy publication metadata was not preserved: version=%d robots=%q seo=%q social=%q", version, robots, seoOverrides, socialOverrides)
	}
	if _, err := db.Exec(`
		INSERT INTO project_publications(id, project_id, content_id, slug, canonical_url)
		VALUES ('replacement', 'project-a', 'article', 'updated-guide', 'https://example.test/blog/updated-guide')
		ON CONFLICT(project_id, content_id) DO UPDATE SET
		  slug = excluded.slug,
		  canonical_url = excluded.canonical_url
	`); err != nil {
		t.Fatalf("expected the repaired conflict target to be usable: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE project_publications
		SET publication_state = 'published'
		WHERE project_id = 'project-a' AND content_id = 'article'
	`); err == nil || !strings.Contains(err.Error(), "publication requires") {
		t.Fatalf("expected the recreated publication update trigger to reject an invalid publish, got %v", err)
	}
}

func TestProjectPublicationMigrationArchivesExtraLocalesForOneArticle(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy-publication-collision.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(db, migration{version: "0001_initial", statements: legacyLocaleInitialMigration(t)}); err != nil {
		t.Fatal(err)
	}
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES ('article', 'project-a', 'guide', 'owner');
	`); err != nil {
		t.Fatal(err)
	}
	seedLegacyLocalePublication(t, db, "publication-en", "article", "en", "english-guide", "2026-01-01 00:00:00")
	seedLegacyLocalePublication(t, db, "publication-fr", "article", "fr", "french-guide", "2026-02-01 00:00:00")

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var applied, rows, archivedRows, localeColumns int
	if err := db.QueryRow(`
		SELECT COUNT(1) FROM schema_migrations
		WHERE version = '0023_project_publications_locale_removal'
	`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM project_publications`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM project_publications_locale_archive`).Scan(&archivedRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM pragma_table_info('project_publications')
		WHERE name = 'locale'
	`).Scan(&localeColumns); err != nil {
		t.Fatal(err)
	}
	if applied != 1 || rows != 1 || archivedRows != 2 || localeColumns != 0 {
		t.Fatalf("unexpected migrated/archive state, applied=%d rows=%d archivedRows=%d localeColumns=%d", applied, rows, archivedRows, localeColumns)
	}
	var retainedID, retainedSlug string
	if err := db.QueryRow(`SELECT id, slug FROM project_publications`).Scan(&retainedID, &retainedSlug); err != nil {
		t.Fatal(err)
	}
	if retainedID != "publication-fr" || retainedSlug != "french-guide" {
		t.Fatalf("expected the most recently updated locale row to remain current, id=%q slug=%q", retainedID, retainedSlug)
	}
	var archivedLocales string
	if err := db.QueryRow(`
		SELECT group_concat(locale, ',')
		FROM (
			SELECT locale FROM project_publications_locale_archive ORDER BY locale
		)
	`).Scan(&archivedLocales); err != nil {
		t.Fatal(err)
	}
	if archivedLocales != "en,fr" {
		t.Fatalf("expected both original locales in the archive, got %q", archivedLocales)
	}
}

func TestProjectPublicationMigrationDisambiguatesCrossLocaleSlugs(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy-publication-slugs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(db, migration{version: "0001_initial", statements: legacyLocaleInitialMigration(t)}); err != nil {
		t.Fatal(err)
	}
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES
		  ('article-en', 'project-a', 'guide', 'owner'),
		  ('article-fr', 'project-a', 'guide', 'owner')
	`); err != nil {
		t.Fatal(err)
	}
	seedLegacyLocalePublication(t, db, "publication-en", "article-en", "en", "shared-guide", "2026-01-01 00:00:00")
	seedLegacyLocalePublication(t, db, "publication-fr", "article-fr", "fr", "shared-guide", "2026-02-01 00:00:00")

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT slug FROM project_publications ORDER BY slug`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatal(err)
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(slugs) != 2 || slugs[1] != "shared-guide" || !strings.HasPrefix(slugs[0], "legacy-locale-") {
		t.Fatalf("expected one original and one deterministic migration slug, got %#v", slugs)
	}
	var archivedRows, archivedSharedSlugs int
	if err := db.QueryRow(`SELECT COUNT(1) FROM project_publications_locale_archive`).Scan(&archivedRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM project_publications_locale_archive WHERE slug = 'shared-guide'`).Scan(&archivedSharedSlugs); err != nil {
		t.Fatal(err)
	}
	if archivedRows != 2 || archivedSharedSlugs != 2 {
		t.Fatalf("expected both original slug rows in the archive, rows=%d shared=%d", archivedRows, archivedSharedSlugs)
	}
}

func seedProjects(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO projects(id, slug, name, public_project_key)
		VALUES
		  ('project-a', 'a', 'Project A', 'public-a'),
		  ('project-b', 'b', 'Project B', 'public-b');
	`); err != nil {
		t.Fatal(err)
	}
}

func TestProjectsAreTopLevelTenants(t *testing.T) {
	db := testDatabase(t)
	var workspaceTables int
	if err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = 'workspaces'`).Scan(&workspaceTables); err != nil {
		t.Fatal(err)
	}
	if workspaceTables != 0 {
		t.Fatal("workspace persistence must not exist")
	}
	if _, err := db.Exec(`INSERT INTO projects(id, slug, name, public_project_key) VALUES ('one', 'shared', 'One', 'public-one')`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `INSERT INTO projects(id, slug, name, public_project_key) VALUES ('two', 'shared', 'Two', 'public-two')`, "unique")
}

func TestTaxonomyHierarchyGuardsInsertAndMove(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	_, err := db.Exec(`
		INSERT INTO taxonomy_terms(id, project_id, type, slug, name, parent_id) VALUES
		  ('root', 'project-a', 'category', 'root', 'Root', NULL),
		  ('child', 'project-a', 'category', 'child', 'Child', 'root'),
		  ('grandchild', 'project-a', 'category', 'grandchild', 'Grandchild', 'child'),
		  ('tag', 'project-a', 'tag', 'tag', 'Tag', NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	assertSQLFails(t, db, `UPDATE taxonomy_terms SET parent_id = 'grandchild' WHERE id = 'root'`, "cycle")
	assertSQLFails(t, db, `UPDATE taxonomy_terms SET parent_id = 'root' WHERE id = 'tag'`, "category parents")
	assertSQLFails(t, db, `INSERT INTO taxonomy_terms(id, project_id, type, slug, name, parent_id) VALUES ('fourth', 'project-a', 'category', 'fourth', 'Fourth', 'grandchild')`, "three levels")
	assertSQLFails(t, db, `UPDATE taxonomy_terms SET type = 'tag' WHERE id = 'root'`, "remain a category")
}

func TestPublicationRequiresApprovedRevisionAndPrimaryCategory(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	_, err := db.Exec(`
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES ('article', 'project-a', 'standard', 'user');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title, content_hash, editorial_state
		) VALUES ('revision', 'project-a', 'article', 1, 'human', 'Title', 'hash', 'approved');
		INSERT INTO taxonomy_terms(id, project_id, type, slug, name)
		VALUES
		  ('category', 'project-a', 'category', 'category', 'Category'),
		  ('tag', 'project-a', 'tag', 'tag', 'Tag');
	`)
	if err != nil {
		t.Fatal(err)
	}

	assertSQLFails(t, db, `
		INSERT INTO project_publications(
		  id, project_id, content_id, slug, canonical_url, published_revision_id, publication_state
		) VALUES ('publication', 'project-a', 'article', 'article', 'https://example.test/article', 'revision', 'published')
	`, "exactly one primary category")
	assertSQLFails(t, db, `
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES ('project-a', 'article', 'tag', 1)
	`, "must be a category")

	if _, err := db.Exec(`
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES ('project-a', 'article', 'category', 1);
		INSERT INTO project_publications(
		  id, project_id, content_id, slug, canonical_url, published_revision_id, publication_state
		) VALUES ('publication', 'project-a', 'article', 'article', 'https://example.test/article', 'revision', 'published');
	`); err != nil {
		t.Fatal(err)
	}
}

func TestProjectScopedOperationalForeignKeys(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO ai_jobs(id, project_id, task_type, started_by)
		VALUES ('job', 'project-a', 'outline', 'user');
	`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `
		INSERT INTO ai_runs(
		  id, project_id, job_id, task_type, provider, model_identifier,
		  prompt_template_version, input_hash, started_by
		) VALUES ('run', 'project-b', 'job', 'outline', 'test', 'test', 'v1', 'hash', 'user')
	`, "foreign key")
}

func TestReviewAssignmentIntegrityGuards(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO users(id, email_normalized, status)
		VALUES
		  ('owner', 'owner@example.test', 'active'),
		  ('reviewer', 'reviewer@example.test', 'active'),
		  ('outsider', 'outsider@example.test', 'active');
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES
		  ('project-a', 'owner', 'project_owner', 'active', CURRENT_TIMESTAMP),
		  ('project-a', 'reviewer', 'reviewer', 'active', CURRENT_TIMESTAMP),
		  ('project-b', 'outsider', 'reviewer', 'active', CURRENT_TIMESTAMP);
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES
		  ('article-a', 'project-a', 'standard', 'owner'),
		  ('article-b', 'project-b', 'standard', 'outsider');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title, content_hash
		) VALUES
		  ('revision-a', 'project-a', 'article-a', 1, 'human', 'A', 'hash-a'),
		  ('revision-b', 'project-b', 'article-b', 1, 'human', 'B', 'hash-b');
	`); err != nil {
		t.Fatal(err)
	}

	assertSQLFails(t, db, `
		INSERT INTO review_assignments(id, project_id, content_id, revision_id, assigned_to, assignment_type, created_by)
		VALUES ('assignment-cross-revision', 'project-a', 'article-a', 'revision-b', 'reviewer', 'reviewer', 'owner')
	`, "revision must belong")
	assertSQLFails(t, db, `
		INSERT INTO review_assignments(id, project_id, content_id, revision_id, assigned_to, assignment_type, created_by)
		VALUES ('assignment-outsider', 'project-a', 'article-a', 'revision-a', 'outsider', 'reviewer', 'owner')
	`, "active project member")
	assertSQLFails(t, db, `
		INSERT INTO review_assignments(id, project_id, content_id, revision_id, assigned_to, assignment_type, status, created_by)
		VALUES ('assignment-status', 'project-a', 'article-a', 'revision-a', 'reviewer', 'reviewer', 'stalled', 'owner')
	`, "status is invalid")

	if _, err := db.Exec(`
		INSERT INTO review_assignments(id, project_id, content_id, revision_id, assigned_to, assignment_type, created_by)
		VALUES ('assignment', 'project-a', 'article-a', 'revision-a', 'reviewer', 'reviewer', 'owner')
	`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `
		INSERT INTO review_assignments(id, project_id, content_id, revision_id, assigned_to, assignment_type, created_by)
		VALUES ('assignment-duplicate', 'project-a', 'article-a', 'revision-a', 'reviewer', 'reviewer', 'owner')
	`, "unique")
	assertSQLFails(t, db, `
		UPDATE review_assignments
		SET status = 'completed'
		WHERE id = 'assignment'
	`, "closure metadata is invalid")
	if _, err := db.Exec(`
		UPDATE review_assignments
		SET status = 'completed', closed_by = 'reviewer', closed_at = CURRENT_TIMESTAMP
		WHERE id = 'assignment'
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO review_assignments(id, project_id, content_id, revision_id, assigned_to, assignment_type, created_by)
		VALUES ('assignment-reopened-slot', 'project-a', 'article-a', 'revision-a', 'reviewer', 'reviewer', 'owner')
	`); err != nil {
		t.Fatalf("expected a closed assignment to release its unique open slot: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO review_assignment_notifications(
		  id, project_id, assignment_id, recipient_user_id, recipient_email
		) VALUES ('notification', 'project-a', 'assignment-reopened-slot', 'reviewer', 'reviewer@example.test')
	`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `
		INSERT INTO review_assignment_notifications(
		  id, project_id, assignment_id, recipient_user_id, recipient_email
		) VALUES ('notification-cross-project', 'project-b', 'assignment-reopened-slot', 'reviewer', 'reviewer@example.test')
	`, "notification scope is invalid")
	assertSQLFails(t, db, `
		INSERT INTO review_assignment_notifications(
		  id, project_id, assignment_id, recipient_user_id, recipient_email
		) VALUES ('notification-wrong-user', 'project-a', 'assignment-reopened-slot', 'owner', 'owner@example.test')
	`, "notification scope is invalid")
}

func TestWebhookDeliveryIntegrityGuards(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO users(id, email_normalized, status)
		VALUES ('owner', 'owner@example.test', 'active');
		INSERT INTO webhook_endpoints(
		  id, project_id, name, url, secret_hash, events_json, created_by
		) VALUES (
		  'endpoint-a', 'project-a', 'A', 'https://hooks.example.test/a',
		  'hash-a', '["content.published"]', 'owner'
		), (
		  'endpoint-b', 'project-b', 'B', 'https://hooks.example.test/b',
		  'hash-b', '["content.published"]', 'owner'
		);
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (
		  'event-a', 'project-a', 'content.published', 'content', 'article-a',
		  '{}', 'event-a'
		), (
		  'event-b', 'project-b', 'content.published', 'content', 'article-b',
		  '{}', 'event-b'
		);
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status
		) VALUES ('attempt-a', 'project-a', 'endpoint-a', 'event-a', 'failed');
	`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `
		UPDATE webhook_endpoints
		SET events_json = '{broken'
		WHERE id = 'endpoint-a'
	`, "events are invalid")
	assertSQLFails(t, db, `
		UPDATE webhook_endpoints
		SET events_json = '["content.published","unknown.event"]'
		WHERE id = 'endpoint-a'
	`, "events are invalid")
	assertSQLFails(t, db, `
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status
		) VALUES ('attempt-invalid-status', 'project-a', 'endpoint-a', 'event-a', 'waiting')
	`, "status is invalid")
	assertSQLFails(t, db, `
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status, attempt_count, max_attempts
		) VALUES ('attempt-invalid-count', 'project-b', 'endpoint-b', 'event-b', 'queued', 3, 2)
	`, "counters are invalid")
	assertSQLFails(t, db, `
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status
		) VALUES ('attempt-duplicate-original', 'project-a', 'endpoint-a', 'event-a', 'queued')
	`, "unique")
	if _, err := db.Exec(`
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status, replay_of_attempt_id
		) VALUES ('attempt-replay', 'project-a', 'endpoint-a', 'event-a', 'queued', 'attempt-a')
	`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status, replay_of_attempt_id
		) VALUES ('attempt-duplicate-replay', 'project-a', 'endpoint-a', 'event-a', 'queued', 'attempt-a')
	`, "unique")
	assertSQLFails(t, db, `
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status, replay_of_attempt_id
		) VALUES ('attempt-cross-replay', 'project-b', 'endpoint-b', 'event-b', 'queued', 'attempt-a')
	`, "replay scope is invalid")
}

func TestWebhookDeliveryMigrationSkipsHistoricalOutboxAndNormalizesLegacyReplays(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy-webhooks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE schema_migrations(
		  version TEXT PRIMARY KEY,
		  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []migration{
		{version: "0001_initial", statements: initialMigration},
		{version: "0008_admin_frontend_services", statements: adminFrontendServicesMigration},
	} {
		if err := applyMigration(db, item); err != nil {
			t.Fatal(err)
		}
	}
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO webhook_endpoints(
		  id, project_id, name, url, secret_hash, events_json, created_by
		) VALUES (
		  'legacy-endpoint', 'project-a', 'Legacy',
		  'https://hooks.example.test/legacy', 'hash',
		  '["content.published"]', 'owner'
		);
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (
		  'legacy-event', 'project-a', 'content.published',
		  'content', 'article', '{}', 'legacy-event'
		);
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status
		) VALUES
		  ('legacy-attempt', 'project-a', 'legacy-endpoint', 'legacy-event', 'failed'),
		  ('legacy-replay', 'project-a', 'legacy-endpoint', 'legacy-event', 'queued');
	`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(db, migration{version: "0016_webhook_delivery", statements: webhookDeliveryMigration}); err != nil {
		t.Fatalf("expected legacy webhook state to migrate: %v", err)
	}
	var fannedOutAt, replayOf string
	if err := db.QueryRow(`
		SELECT COALESCE(webhook_fanned_out_at, '')
		FROM outbox_events
		WHERE id = 'legacy-event'
	`).Scan(&fannedOutAt); err != nil {
		t.Fatal(err)
	}
	if fannedOutAt == "" {
		t.Fatal("expected historical outbox event to be excluded from initial webhook fan-out")
	}
	if err := db.QueryRow(`
		SELECT COALESCE(replay_of_attempt_id, '')
		FROM webhook_attempts
		WHERE id = 'legacy-replay'
	`).Scan(&replayOf); err != nil {
		t.Fatal(err)
	}
	if replayOf != "legacy-attempt" {
		t.Fatalf("expected duplicate legacy delivery to become a replay, got %q", replayOf)
	}
	if _, err := db.Exec(`
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (
		  'new-event', 'project-a', 'content.published',
		  'content', 'article', '{}', 'new-event'
		)
	`); err != nil {
		t.Fatal(err)
	}
	var newFannedOutAt any
	if err := db.QueryRow(`
		SELECT webhook_fanned_out_at
		FROM outbox_events
		WHERE id = 'new-event'
	`).Scan(&newFannedOutAt); err != nil {
		t.Fatal(err)
	}
	if newFannedOutAt != nil {
		t.Fatalf("expected post-migration event to remain eligible for fan-out, got %#v", newFannedOutAt)
	}
}

func TestAIJobContextMigrationAllowsLegacyDuplicateHashesAndGuardsUpgrades(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy-ai-jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE schema_migrations(
		  version TEXT PRIMARY KEY,
		  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []migration{
		{version: "0001_initial", statements: initialMigration},
		{version: "0002_session_reauthentication", statements: sessionReauthenticationMigration},
		{version: "0003_invitation_revocation", statements: invitationRevocationMigration},
		{version: "0004_audit_event_ids", statements: auditEventIDsMigration},
		{version: "0005_author_photo_asset_guard", statements: authorPhotoAssetGuardMigration},
		{version: "0006_review_comments_index", statements: reviewCommentsIndexMigration},
		{version: "0007_revision_base_guard", statements: revisionBaseGuardMigration},
		{version: "0008_admin_frontend_services", statements: adminFrontendServicesMigration},
		{version: "0009_preview_tokens", statements: previewTokensMigration},
		{version: "0010_ai_observability", statements: aiObservabilityMigration},
		{version: "0011_ai_context", statements: aiContextMigration},
	} {
		if err := applyMigration(db, item); err != nil {
			t.Fatal(err)
		}
	}
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO ai_jobs(id, project_id, task_type, status, input_hash, started_by)
		VALUES
		  ('legacy-ai-job-one', 'project-a', 'outline', 'queued', 'legacy-duplicate-hash', 'user'),
		  ('legacy-ai-job-two', 'project-a', 'outline', 'queued', 'legacy-duplicate-hash', 'user'),
		  ('legacy-ai-job-unbound', 'project-a', 'outline', 'queued', NULL, 'user')
	`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(db, migration{version: "0012_ai_job_context", statements: aiJobContextMigration}); err != nil {
		t.Fatalf("expected legacy duplicate hashes not to block migration: %v", err)
	}
	assertSQLFails(t, db, `
		UPDATE ai_jobs
		SET input_hash = 'newly-bound-without-context'
		WHERE id = 'legacy-ai-job-unbound'
	`, "context is incomplete")
}

func TestRevisionBaseMustBeEarlierAndBelongToSameProjectArticle(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES
		  ('article-a', 'project-a', 'standard', 'user'),
		  ('article-a-other', 'project-a', 'standard', 'user'),
		  ('article-b', 'project-b', 'standard', 'user');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title, content_hash
		) VALUES
		  ('revision-a-1', 'project-a', 'article-a', 1, 'human', 'A1', 'hash-a-1'),
		  ('revision-a-other-1', 'project-a', 'article-a-other', 1, 'human', 'Other A1', 'hash-a-other-1'),
		  ('revision-b-1', 'project-b', 'article-b', 1, 'human', 'B1', 'hash-b-1');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, base_revision_id,
		  created_by_type, title, content_hash
		) VALUES (
		  'revision-a-2', 'project-a', 'article-a', 2, 'revision-a-1',
		  'human', 'A2', 'hash-a-2'
		);
	`); err != nil {
		t.Fatal(err)
	}

	assertSQLFails(t, db, `
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, base_revision_id,
		  created_by_type, title, content_hash
		) VALUES (
		  'revision-a-cross-project', 'project-a', 'article-a', 3, 'revision-b-1',
		  'human', 'Cross project', 'hash-cross-project'
		)
	`, "same project article")
	assertSQLFails(t, db, `
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, base_revision_id,
		  created_by_type, title, content_hash
		) VALUES (
		  'revision-a-cross-article', 'project-a', 'article-a', 3, 'revision-a-other-1',
		  'human', 'Cross article', 'hash-cross-article'
		)
	`, "same project article")
	assertSQLFails(t, db, `
		UPDATE content_revisions
		SET base_revision_id = 'revision-a-2'
		WHERE id = 'revision-a-2'
	`, "revision")
	assertSQLFails(t, db, `
		UPDATE content_revisions
		SET revision_number = 3
		WHERE id = 'revision-a-1'
	`, "immutable")
	assertSQLFails(t, db, `
		DELETE FROM content_revisions
		WHERE id = 'revision-a-1'
	`, "immutable")
	if _, err := db.Exec(`
		DELETE FROM content_items
		WHERE project_id = 'project-a' AND id = 'article-a'
	`); err != nil {
		t.Fatalf("expected whole-article deletion to cascade its revision history: %v", err)
	}
	var remainingArticleRevisions int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM content_revisions
		WHERE project_id = 'project-a' AND content_id = 'article-a'
	`).Scan(&remainingArticleRevisions); err != nil {
		t.Fatal(err)
	}
	if remainingArticleRevisions != 0 {
		t.Fatalf("expected article revision cascade to remove all history, got %d rows", remainingArticleRevisions)
	}
}

func TestRevisionBaseMigrationRejectsInvalidExistingLineage(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE schema_migrations(
		  version TEXT PRIMARY KEY,
		  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []migration{
		{version: "0001_initial", statements: initialMigration},
		{version: "0002_session_reauthentication", statements: sessionReauthenticationMigration},
		{version: "0003_invitation_revocation", statements: invitationRevocationMigration},
		{version: "0004_audit_event_ids", statements: auditEventIDsMigration},
		{version: "0005_author_photo_asset_guard", statements: authorPhotoAssetGuardMigration},
		{version: "0006_review_comments_index", statements: reviewCommentsIndexMigration},
	} {
		if err := applyMigration(db, item); err != nil {
			t.Fatal(err)
		}
	}
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES
		  ('article-a', 'project-a', 'standard', 'user'),
		  ('article-b', 'project-b', 'standard', 'user');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, base_revision_id,
		  created_by_type, title, content_hash
		) VALUES
		  ('revision-b-1', 'project-b', 'article-b', 1, NULL, 'human', 'B1', 'hash-b-1'),
		  ('revision-a-1', 'project-a', 'article-a', 1, 'revision-b-1', 'human', 'A1', 'hash-a-1');
	`); err != nil {
		t.Fatal(err)
	}

	err = applyMigration(db, migration{version: "0007_revision_base_guard", statements: revisionBaseGuardMigration})
	if err == nil {
		t.Fatal("expected migration to reject invalid existing base lineage")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "same project article") {
		t.Fatalf("expected lineage validation error, got %q", err)
	}
}

func TestAuthorPhotoAssetMustBelongToProject(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO assets(
		  id, project_id, object_key, filename, mime_type, byte_size, created_by
		) VALUES
		  ('asset-a', 'project-a', 'a.jpg', 'a.jpg', 'image/jpeg', 100, 'user'),
		  ('asset-b', 'project-b', 'b.jpg', 'b.jpg', 'image/jpeg', 100, 'user');
		INSERT INTO authors(id, project_id, slug, display_name, photo_asset_id)
		VALUES ('author-a', 'project-a', 'author-a', 'Author A', 'asset-a');
	`); err != nil {
		t.Fatal(err)
	}

	assertSQLFails(t, db, `
		INSERT INTO authors(id, project_id, slug, display_name, photo_asset_id)
		VALUES ('author-cross-project', 'project-a', 'cross-project', 'Cross Project', 'asset-b')
	`, "same project")
	assertSQLFails(t, db, `
		INSERT INTO authors(id, project_id, slug, display_name, photo_asset_id)
		VALUES ('author-missing', 'project-a', 'missing', 'Missing', 'asset-missing')
	`, "same project")
	assertSQLFails(t, db, `
		UPDATE authors
		SET photo_asset_id = 'asset-b'
		WHERE id = 'author-a'
	`, "same project")
	assertSQLFails(t, db, `
		DELETE FROM assets
		WHERE id = 'asset-a'
	`, "author photo")
	assertSQLFails(t, db, `
		UPDATE assets
		SET id = 'asset-a-renamed'
		WHERE id = 'asset-a'
	`, "author photo")
}

func TestAuthorLoginUserMustBelongToProject(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO users(id, email_normalized, status)
		VALUES
		  ('member-a', 'member-a@example.test', 'active'),
		  ('member-b', 'member-b@example.test', 'active'),
		  ('disabled-a', 'disabled-a@example.test', 'disabled');
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES
		  ('project-a', 'member-a', 'writer', 'active', CURRENT_TIMESTAMP),
		  ('project-b', 'member-b', 'writer', 'active', CURRENT_TIMESTAMP),
		  ('project-a', 'disabled-a', 'writer', 'active', CURRENT_TIMESTAMP);
		INSERT INTO authors(id, project_id, slug, display_name, login_user_id)
		VALUES ('author-a', 'project-a', 'author-a', 'Author A', 'member-a');
	`); err != nil {
		t.Fatal(err)
	}

	assertSQLFails(t, db, `
		INSERT INTO authors(id, project_id, slug, display_name, login_user_id)
		VALUES ('author-cross-project', 'project-a', 'cross-project', 'Cross Project', 'member-b')
	`, "same project")
	assertSQLFails(t, db, `
		INSERT INTO authors(id, project_id, slug, display_name, login_user_id)
		VALUES ('author-missing', 'project-a', 'missing', 'Missing', 'missing-user')
	`, "same project")
	assertSQLFails(t, db, `
		INSERT INTO authors(id, project_id, slug, display_name, login_user_id)
		VALUES ('author-disabled', 'project-a', 'disabled', 'Disabled', 'disabled-a')
	`, "same project")
	assertSQLFails(t, db, `
		INSERT INTO authors(id, project_id, slug, display_name, login_user_id)
		VALUES ('author-duplicate', 'project-a', 'duplicate', 'Duplicate', 'member-a')
	`, "unique")
	assertSQLFails(t, db, `
		UPDATE authors
		SET login_user_id = 'member-b'
		WHERE id = 'author-a'
	`, "same project")
}

func TestAuditEventsGenerateLegacyIDsAndRemainAppendOnly(t *testing.T) {
	db := testDatabase(t)
	seedProjects(t, db)
	if _, err := db.Exec(`
		INSERT INTO audit_events(project_id, actor_type, action, outcome)
		VALUES ('project-a', 'user', 'project.test', 'success')
	`); err != nil {
		t.Fatal(err)
	}
	var generatedID string
	if err := db.QueryRow(`
		SELECT id
		FROM audit_events
		WHERE project_id = 'project-a' AND action = 'project.test'
	`).Scan(&generatedID); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(generatedID, "audit_") {
		t.Fatalf("expected a generated legacy audit ID, got %q", generatedID)
	}

	if _, err := db.Exec(`
		INSERT INTO audit_events(id, project_id, actor_type, action, outcome)
		VALUES ('audit_test', 'project-a', 'user', 'project.test', 'success')
	`); err != nil {
		t.Fatal(err)
	}
	assertSQLFails(t, db, `UPDATE audit_events SET outcome = 'failure' WHERE id = '`+generatedID+`'`, "append-only")
	assertSQLFails(t, db, `UPDATE audit_events SET outcome = 'failure' WHERE id = 'audit_test'`, "append-only")
	assertSQLFails(t, db, `DELETE FROM audit_events WHERE id = 'audit_test'`, "append-only")
}

func TestReviewCommentsContentIndex(t *testing.T) {
	db := testDatabase(t)
	var definition string
	if err := db.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_review_comments_content'
	`).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	normalized := strings.Join(strings.Fields(definition), " ")
	if !strings.Contains(normalized, "review_comments(project_id, content_id, id)") {
		t.Fatalf("unexpected review comments index definition %q", definition)
	}
}

func assertSQLFails(t *testing.T, db *sql.DB, statement, expected string) {
	t.Helper()
	if _, err := db.Exec(statement); err == nil {
		t.Fatalf("expected SQL to fail: %s", statement)
	} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(expected)) {
		t.Fatalf("expected error containing %q, got %q", expected, err)
	}
}
