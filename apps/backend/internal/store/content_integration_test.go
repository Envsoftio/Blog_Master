package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
)

func TestPublishedPostQueryAndSnapshots(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "content.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO workspaces(id, slug, name) VALUES ('workspace', 'workspace', 'Workspace');
		INSERT INTO projects(id, workspace_id, slug, name, public_project_key)
		VALUES ('project', 'workspace', 'project', 'Project', 'public');
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES ('article', 'project', 'standard', 'user');
		INSERT INTO taxonomy_terms(id, project_id, type, slug, name)
		VALUES ('category', 'project', 'category', 'category', 'Category');
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES ('project', 'article', 'category', 1);
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title,
		  body_document_json, sanitized_html, taxonomy_snapshot_json,
		  author_snapshot_json, seo_snapshot_json, content_hash, editorial_state
		) VALUES (
		  'revision', 'project', 'article', 1, 'human', 'Published title',
		  '{"type":"doc"}', '<p>Published</p>',
		  '{"primaryCategory":{"id":"category","type":"category","slug":"category","name":"Category","indexable":true},"categories":[],"tags":[],"topics":[]}',
		  '[]', '[]', 'content-hash', 'approved'
		);
		INSERT INTO project_publications(
		  id, project_id, content_id, locale, slug, canonical_url,
		  published_revision_id, publication_state, first_published_at
		) VALUES (
		  'publication', 'project', 'article', 'en', 'published',
		  'https://example.test/blog/published', 'revision', 'published',
		  '2026-07-29 10:00:00'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	contentStore := store.New(db)
	posts, err := contentStore.ListPublishedPosts(
		context.Background(), "project", "en", "category", "", "", "", "",
		false, "", "", store.PublishedCursor{}, 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected one post, got %d", len(posts))
	}
	if posts[0].ContentHash != "content-hash" {
		t.Fatalf("unexpected content hash %q", posts[0].ContentHash)
	}
	if posts[0].SEO.Title != "Published title" {
		t.Fatalf("invalid SEO JSON should safely fall back to title, got %q", posts[0].SEO.Title)
	}
	if posts[0].Taxonomy.PrimaryCategory == nil {
		t.Fatal("expected primary-category snapshot")
	}
	structuredData, ok := posts[0].SEO.StructuredData.([]any)
	if !ok || len(structuredData) != 1 {
		t.Fatalf("expected generated article structured data, got %#v", posts[0].SEO.StructuredData)
	}
	article, ok := structuredData[0].(map[string]any)
	if !ok || article["@type"] != "BlogPosting" || article["headline"] != "Published title" {
		t.Fatalf("unexpected article structured data: %#v", structuredData[0])
	}
	publisher, ok := article["publisher"].(map[string]any)
	if !ok || publisher["name"] != "Project" {
		t.Fatalf("expected project-name publisher fallback, got %#v", article["publisher"])
	}
}
