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
		INSERT INTO projects(id, slug, name, public_project_key)
		VALUES ('project', 'project', 'Project', 'public');
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
		  id, project_id, content_id, slug, canonical_url,
		  published_revision_id, publication_state, first_published_at
		) VALUES (
		  'publication', 'project', 'article', 'published',
		  'https://example.test/blog/published', 'revision', 'published',
		  '2026-07-29 10:00:00'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	contentStore := store.New(db)
	posts, err := contentStore.ListPublishedPosts(
		context.Background(), "project", "category", "", "", "", "",
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
}

func TestPublishedPostHydratesOrderedSeriesAndRelationships(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "relationships.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO projects(id, slug, name, public_project_key)
		VALUES ('project', 'project', 'Project', 'public');
		INSERT INTO taxonomy_terms(id, project_id, type, slug, name)
		VALUES ('category', 'project', 'category', 'guides', 'Guides');
		INSERT INTO series(id, project_id, slug, name, description)
		VALUES ('series', 'project', 'seo-course', 'SEO Course', 'An ordered course');

		INSERT INTO content_items(id, project_id, article_type, created_by) VALUES
		  ('previous', 'project', 'guide', 'user'),
		  ('current', 'project', 'guide', 'user'),
		  ('next', 'project', 'guide', 'user'),
		  ('pillar', 'project', 'guide', 'user');
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary) VALUES
		  ('project', 'previous', 'category', 1),
		  ('project', 'current', 'category', 1),
		  ('project', 'next', 'category', 1),
		  ('project', 'pillar', 'category', 1);
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title, excerpt,
		  taxonomy_snapshot_json, content_hash, editorial_state
		) VALUES
		  ('revision-previous', 'project', 'previous', 1, 'human', 'Previous lesson', 'Previous excerpt',
		   '{"primaryCategory":{"id":"category","type":"category","slug":"guides","name":"Guides","indexable":true}}', 'hash-previous', 'approved'),
		  ('revision-current', 'project', 'current', 1, 'human', 'Current lesson', 'Current excerpt',
		   '{"primaryCategory":{"id":"category","type":"category","slug":"guides","name":"Guides","indexable":true}}', 'hash-current', 'approved'),
		  ('revision-next', 'project', 'next', 1, 'human', 'Next lesson', 'Next excerpt',
		   '{"primaryCategory":{"id":"category","type":"category","slug":"guides","name":"Guides","indexable":true}}', 'hash-next', 'approved'),
		  ('revision-pillar', 'project', 'pillar', 1, 'human', 'Pillar page', 'Pillar excerpt',
		   '{"primaryCategory":{"id":"category","type":"category","slug":"guides","name":"Guides","indexable":true}}', 'hash-pillar', 'approved');
		INSERT INTO project_publications(
		  id, project_id, content_id, slug, canonical_url, published_revision_id, publication_state
		) VALUES
		  ('publication-previous', 'project', 'previous', 'previous', 'https://example.test/blog/previous', 'revision-previous', 'published'),
		  ('publication-current', 'project', 'current', 'current', 'https://example.test/blog/current', 'revision-current', 'published'),
		  ('publication-next', 'project', 'next', 'next', 'https://example.test/blog/next', 'revision-next', 'published'),
		  ('publication-pillar', 'project', 'pillar', 'pillar', 'https://example.test/blog/pillar', 'revision-pillar', 'published');
		INSERT INTO series_articles(project_id, series_id, content_id, position) VALUES
		  ('project', 'series', 'previous', 1),
		  ('project', 'series', 'current', 2),
		  ('project', 'series', 'next', 3);
		INSERT INTO content_relationships(
		  project_id, source_content_id, target_content_id, relationship_type, origin, position
		) VALUES
		  ('project', 'current', 'next', 'related', 'manual', 1),
		  ('project', 'current', 'pillar', 'pillar', 'deterministic', 2);
	`)
	if err != nil {
		t.Fatal(err)
	}

	post, err := store.New(db).GetPublishedPostBySlug(context.Background(), "project", "current")
	if err != nil {
		t.Fatal(err)
	}
	if post.Taxonomy.Series == nil || post.Taxonomy.Series.Position != 2 {
		t.Fatalf("expected ordered series membership, got %#v", post.Taxonomy.Series)
	}
	if post.Taxonomy.Series.Previous == nil || post.Taxonomy.Series.Previous.Slug != "previous" ||
		post.Taxonomy.Series.Next == nil || post.Taxonomy.Series.Next.Slug != "next" {
		t.Fatalf("expected published previous/next navigation, got %#v", post.Taxonomy.Series)
	}
	if len(post.RelatedArticles) != 1 || post.RelatedArticles[0].Article.Slug != "next" || post.RelatedArticles[0].Position != 1 {
		t.Fatalf("expected compact ordered related article output, got %#v", post.RelatedArticles)
	}
	if len(post.TopicRelationships) != 1 || post.TopicRelationships[0].RelationshipType != "pillar" ||
		post.TopicRelationships[0].Origin != "deterministic" {
		t.Fatalf("expected topic-cluster relationship output, got %#v", post.TopicRelationships)
	}
}
