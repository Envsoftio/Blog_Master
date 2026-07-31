package store

import (
	"encoding/json"
	"errors"
	"testing"
)

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
