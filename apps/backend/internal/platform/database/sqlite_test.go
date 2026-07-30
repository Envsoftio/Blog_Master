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

func seedProjects(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO workspaces(id, slug, name) VALUES ('workspace', 'workspace', 'Workspace');
		INSERT INTO projects(id, workspace_id, slug, name, public_project_key)
		VALUES
		  ('project-a', 'workspace', 'a', 'Project A', 'public-a'),
		  ('project-b', 'workspace', 'b', 'Project B', 'public-b');
	`); err != nil {
		t.Fatal(err)
	}
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
		  id, project_id, content_id, locale, slug, canonical_url, published_revision_id, publication_state
		) VALUES ('publication', 'project-a', 'article', 'en', 'article', 'https://example.test/article', 'revision', 'published')
	`, "exactly one primary category")
	assertSQLFails(t, db, `
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES ('project-a', 'article', 'tag', 1)
	`, "must be a category")

	if _, err := db.Exec(`
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES ('project-a', 'article', 'category', 1);
		INSERT INTO project_publications(
		  id, project_id, content_id, locale, slug, canonical_url, published_revision_id, publication_state
		) VALUES ('publication', 'project-a', 'article', 'en', 'article', 'https://example.test/article', 'revision', 'published');
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
