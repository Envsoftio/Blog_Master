package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeJSONObjectRejectsNonObjects(t *testing.T) {
	for _, input := range []string{`[]`, `"text"`, `42`, `{broken`} {
		value := decodeJSONObject(input)
		if len(value) != 0 {
			t.Fatalf("expected empty object for %q, got %#v", input, value)
		}
	}
}

func TestPublishedPostReturnsSEOInputsWithoutRenderingJSONLD(t *testing.T) {
	encoded, err := json.Marshal(PublishedPost{
		Title: "A trustworthy headline",
		SEO: PublishedSEOInputs{
			Title:        "Search title",
			Description:  "Search description",
			CanonicalURL: "https://example.test/blog/trustworthy-headline",
			Robots:       "index,follow",
			Index:        true,
			OpenGraph:    map[string]any{"image": "/media/hero.jpg"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := string(encoded)
	if !strings.Contains(output, `"seo":{"title":"Search title"`) {
		t.Fatalf("expected advisory SEO fields in published JSON, got %s", output)
	}
	if strings.Contains(output, "structuredData") || strings.Contains(output, "schema.org") {
		t.Fatalf("provider must not render JSON-LD, got %s", output)
	}
}

func TestPublishedMediaSnapshotRequiresResponsiveDimensionsAndSafeURLs(t *testing.T) {
	media := publishedMediaFromSnapshot(`{
		"hero": {
			"id": "asset-1", "url": "/media/hero.webp", "mimeType": "image/webp",
			"width": 1600, "height": 900, "altText": "A useful diagram", "decorative": false,
			"variants": [
				{"name":"square_1x1","url":"https://assets.example.test/square.webp","mimeType":"image/webp","width":800,"height":800},
				{"name":"unsafe","url":"javascript:alert(1)","mimeType":"image/webp","width":800,"height":600},
				{"name":"missing-dimensions","url":"/media/no-size.webp","mimeType":"image/webp","width":0,"height":0}
			]
		}
	}`)
	if media.Hero == nil || media.Hero.Width != 1600 || media.Hero.Height != 900 {
		t.Fatalf("expected typed hero with explicit dimensions, got %#v", media.Hero)
	}
	if len(media.Hero.Variants) != 1 || media.Hero.Variants[0].Name != "square_1x1" {
		t.Fatalf("expected only complete safe responsive variants, got %#v", media.Hero.Variants)
	}

	invalid := publishedMediaFromSnapshot(`{"hero":{"id":"asset-2","url":"/media/no-size.webp","mimeType":"image/webp"}}`)
	if invalid.Hero != nil {
		t.Fatalf("expected incomplete hero to be omitted, got %#v", invalid.Hero)
	}
}
