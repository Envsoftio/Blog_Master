package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"seoblog/apps/backend/internal/platform/database"
)

func TestUpdatePublicationSupportsLegacyLocaleConstraint(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "legacy-publication.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE content_revisions (
			project_id TEXT NOT NULL,
			content_id TEXT NOT NULL,
			id TEXT NOT NULL,
			seo_snapshot_json TEXT NOT NULL DEFAULT '{}'
		);
		CREATE TABLE project_publications (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			content_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			slug TEXT NOT NULL,
			canonical_url TEXT NOT NULL,
			robots_directive TEXT NOT NULL DEFAULT 'index,follow',
			published_revision_id TEXT,
			publication_state TEXT NOT NULL DEFAULT 'unpublished',
			scheduled_for_utc TEXT,
			first_published_at TEXT,
			materially_modified_at TEXT,
			publication_version INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_id, content_id, locale),
			UNIQUE(project_id, locale, slug)
		);
		INSERT INTO content_revisions(project_id, content_id, id, seo_snapshot_json)
		VALUES ('project', 'article', 'revision', '{"robots":"noindex,follow"}');
		INSERT INTO project_publications(id, project_id, content_id, locale, slug, canonical_url)
		VALUES
			('publication-en', 'project', 'article', 'en', 'old-en', 'https://example.test/old-en'),
			('publication-fr', 'project', 'article', 'fr', 'old-fr', 'https://example.test/old-fr');
	`); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := updatePublication(
		context.Background(), tx, "publication-en", "project", "article", "revision",
		"updated", "https://example.test/updated", "", "published",
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("expected legacy publication update to avoid conflict-target SQL: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var slug, state, robots string
	var version int64
	if err := db.QueryRow(`
		SELECT slug, publication_state, robots_directive, publication_version
		FROM project_publications
		WHERE id = 'publication-en'
	`).Scan(&slug, &state, &robots, &version); err != nil {
		t.Fatal(err)
	}
	if slug != "updated" || state != "published" || robots != "noindex,follow" || version != 2 {
		t.Fatalf("unexpected updated legacy publication slug=%q state=%q robots=%q version=%d", slug, state, robots, version)
	}
	var untouchedSlug string
	if err := db.QueryRow(`SELECT slug FROM project_publications WHERE id = 'publication-fr'`).Scan(&untouchedSlug); err != nil {
		t.Fatal(err)
	}
	if untouchedSlug != "old-fr" {
		t.Fatalf("expected the other legacy locale to remain untouched, got %q", untouchedSlug)
	}
}

func TestValidateCopyBodyReferences(t *testing.T) {
	t.Run("allows local document anchors and external URLs", func(t *testing.T) {
		err := validateCopyBodyReferences(
			`{"type":"doc","content":[{"type":"heading","attrs":{"id":"stable-heading"}},{"type":"link","attrs":{"href":"https://example.test/reference"}}]}`,
			`<h2 id="stable-heading">Heading</h2><a href="https://example.test/reference">Reference</a>`,
			`## Heading`,
		)
		if err != nil {
			t.Fatalf("expected safe body to be copyable, got %v", err)
		}
	})

	for _, test := range []struct {
		name     string
		document string
		html     string
		markdown string
	}{
		{
			name:     "structured asset reference",
			document: `{"type":"image","attrs":{"assetId":"asset_source"}}`,
		},
		{
			name:     "snake case related article reference",
			document: `{"type":"relatedArticle","attrs":{"related_article_id":"art_source"}}`,
		},
		{
			name:     "rendered media reference",
			document: `{"type":"doc","content":[]}`,
			html:     `<figure data-media-id="asset_source"></figure>`,
		},
		{
			name:     "markdown content reference",
			document: `{"type":"doc","content":[]}`,
			markdown: `<aside data-related-article-id="art_source"></aside>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateCopyBodyReferences(test.document, test.html, test.markdown)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected project-scoped reference validation error, got %v", err)
			}
		})
	}

	t.Run("rejects malformed structured body", func(t *testing.T) {
		err := validateCopyBodyReferences(`{broken`, "", "")
		if !errors.Is(err, ErrInvalidWorkflow) {
			t.Fatalf("expected invalid workflow error, got %v", err)
		}
	})
}

func TestCanonicalURLsEqual(t *testing.T) {
	if !canonicalURLsEqual("HTTPS://SOURCE.EXAMPLE.TEST", "https://source.example.test/") {
		t.Fatal("expected canonical URL normalization to ignore scheme and host casing and an empty root path")
	}
	for _, candidate := range []string{
		"https://other.example.test/",
		"https://user:password@source.example.test/",
		"https://source.example.test/#fragment",
	} {
		if canonicalURLsEqual(candidate, "https://source.example.test/") {
			t.Fatalf("expected canonical URL %q to be rejected or differ", candidate)
		}
	}
}

func TestSEOSnapshotNormalizationValidationAndCopy(t *testing.T) {
	input := normalizeSEOInput(SEOInput{
		Robots:           " NoIndex, NoFollow ",
		OpenGraphImage:   "/media/social-card.png",
		OpenGraphSummary: "Social summary",
	}, "Fallback title", "Fallback description")
	if err := validateSEOInput(input); err != nil {
		t.Fatal(err)
	}
	if input.Title != "Fallback title" || input.Description != "Fallback description" {
		t.Fatalf("expected editorial fallbacks, got %#v", input)
	}
	if input.Robots != "noindex,nofollow" || input.OpenGraphTitle != "Fallback title" {
		t.Fatalf("unexpected normalized SEO input: %#v", input)
	}

	raw, err := seoSnapshotJSON(input, "https://source.example.test/blog/post")
	if err != nil {
		t.Fatal(err)
	}
	copied, err := copySEOSnapshotJSON(raw, "Other title", "Other description", "https://destination.example.test/blog/post")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(copied), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["canonicalUrl"] != "https://destination.example.test/blog/post" || snapshot["robots"] != "noindex,nofollow" {
		t.Fatalf("copy did not preserve SEO while replacing canonical URL: %#v", snapshot)
	}

	input.OpenGraphImage = "javascript:alert(1)"
	if !errors.Is(validateSEOInput(input), ErrValidation) {
		t.Fatal("expected an unsafe Open Graph image URL to be rejected")
	}
}
