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
	_, err := db.Exec(`
		INSERT INTO workspaces(id, slug, name) VALUES ('workspace', 'workspace', 'Workspace');
		INSERT INTO projects(id, workspace_id, slug, name, public_project_key)
		VALUES
		  ('project-a', 'workspace', 'a', 'Project A', 'public-a'),
		  ('project-b', 'workspace', 'b', 'Project B', 'public-b');
	`)
	if err != nil {
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

func assertSQLFails(t *testing.T, db *sql.DB, statement, expected string) {
	t.Helper()
	if _, err := db.Exec(statement); err == nil {
		t.Fatalf("expected SQL to fail: %s", statement)
	} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(expected)) {
		t.Fatalf("expected error containing %q, got %q", expected, err)
	}
}
