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

func TestFirstPublishedHeroReferenceUsesFirstProjectImage(t *testing.T) {
	document := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{"type": "paragraph"},
			map[string]any{
				"type": "figure",
				"content": []any{map[string]any{
					"type": "image",
					"attrs": map[string]any{
						"src": "/api/v1/projects/project-a/media/asset-hero/file?variant=square_1x1",
						"alt": "Project environment",
					},
				}},
			},
		},
	}
	reference := firstPublishedHeroReference(document, "project-a")
	if reference.AssetID != "asset-hero" || reference.AltText != "Project environment" || reference.Decorative {
		t.Fatalf("unexpected hero reference %#v", reference)
	}
	if crossProject := firstPublishedHeroReference(document, "project-b"); crossProject.AssetID != "" {
		t.Fatalf("cross-project media must not become hero: %#v", crossProject)
	}
}

func TestPublishedHeroFromAssetPrefersWidescreenVariant(t *testing.T) {
	hero := publishedHeroFromAsset(AdminMediaAsset{
		ID: "asset-hero", ProjectID: "project-a", Status: "ready",
		ContentType: "image/png", Width: 736, Height: 736, AltText: "Library alt",
		Variants: []AdminMediaVariant{
			{Name: "square_1x1", ContentType: "image/jpeg", Width: 800, Height: 800},
			{Name: "widescreen_16x9", ContentType: "image/jpeg", Width: 1600, Height: 900},
		},
	}, publishedHeroReference{AssetID: "asset-hero", AltText: "Article alt"})
	if hero == nil {
		t.Fatal("expected a published hero")
	}
	if hero.URL != "/api/v1/projects/project-a/media/asset-hero/file?variant=widescreen_16x9" || hero.Width != 1600 || hero.Height != 900 {
		t.Fatalf("expected widescreen hero, got %#v", hero)
	}
	if hero.AltText != "Article alt" || len(hero.Variants) != 2 {
		t.Fatalf("expected article accessibility metadata and variants, got %#v", hero)
	}
}
