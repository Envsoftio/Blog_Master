package store

import "testing"

func TestDecodeJSONObjectRejectsNonObjects(t *testing.T) {
	for _, input := range []string{`[]`, `"text"`, `42`, `{broken`} {
		value := decodeJSONObject(input)
		if len(value) != 0 {
			t.Fatalf("expected empty object for %q, got %#v", input, value)
		}
	}
}

func TestPublishedArticleStructuredDataUsesApprovedInputs(t *testing.T) {
	post := PublishedPost{
		ArticleType:   "news_update",
		Title:         "A trustworthy headline",
		PublishedAt:   "2026-08-01 10:30:00",
		ModifiedAt:    "2026-08-02T11:45:00+05:30",
		PublisherName: "Example Publisher",
		PublisherURL:  "https://publisher.example/about",
		SEO: PublishedSEO{
			CanonicalURL: "https://example.test/blog/trustworthy-headline",
			Description:  "An approved description.",
			OpenGraph: map[string]any{
				"image": "/media/hero.jpg",
			},
		},
		Authors: []Author{
			{
				DisplayName:  "Asha Rao",
				JobTitle:     "Editor",
				Organization: "Example Labs",
				ProfileURL:   "https://example.test/authors/asha",
				SameAs: []string{
					"https://profiles.example/asha",
					"https://profiles.example/asha",
					"javascript:alert(1)",
				},
			},
			{DisplayName: "Ravi Singh"},
		},
		Taxonomy: PublishedTaxonomy{
			PrimaryCategory: &TaxonomyTerm{
				Name:      "Engineering",
				Slug:      "engineering",
				Ancestors: []TaxonomyTerm{{Name: "Technology", Slug: "technology"}},
			},
			Categories: []TaxonomyTerm{
				{Name: "Engineering"},
				{Name: "Search"},
			},
			Tags: []TaxonomyTerm{
				{Name: "SEO"},
				{Name: "Publishing"},
			},
		},
	}

	structuredData := publishedArticleStructuredData(post)
	if len(structuredData) != 2 {
		t.Fatalf("expected article and breadcrumb schemas, got %#v", structuredData)
	}
	article, ok := structuredData[0].(map[string]any)
	if !ok {
		t.Fatalf("expected an article object, got %#v", structuredData[0])
	}
	if article["@type"] != "NewsArticle" || article["headline"] != post.Title {
		t.Fatalf("unexpected article identity: %#v", article)
	}
	if article["@id"] != post.SEO.CanonicalURL+"#article" || article["mainEntityOfPage"] != post.SEO.CanonicalURL {
		t.Fatalf("expected canonical article identity, got %#v", article)
	}
	if article["datePublished"] != "2026-08-01T10:30:00Z" || article["dateModified"] != "2026-08-02T06:15:00Z" {
		t.Fatalf("expected normalized publication dates, got %#v", article)
	}
	images, ok := article["image"].([]string)
	if !ok || len(images) != 1 || images[0] != "https://example.test/media/hero.jpg" {
		t.Fatalf("expected a canonical absolute image URL, got %#v", article["image"])
	}
	authors, ok := article["author"].([]any)
	if !ok || len(authors) != 2 {
		t.Fatalf("expected separate author objects, got %#v", article["author"])
	}
	firstAuthor := authors[0].(map[string]any)
	if firstAuthor["name"] != "Asha Rao" || firstAuthor["url"] != "https://example.test/authors/asha" {
		t.Fatalf("unexpected first author: %#v", firstAuthor)
	}
	sameAs, ok := firstAuthor["sameAs"].([]string)
	if !ok || len(sameAs) != 1 || sameAs[0] != "https://profiles.example/asha" {
		t.Fatalf("expected unsafe and duplicate identity URLs to be removed, got %#v", firstAuthor["sameAs"])
	}
	sections, ok := article["articleSection"].([]string)
	if !ok || len(sections) != 2 || sections[0] != "Engineering" || sections[1] != "Search" {
		t.Fatalf("expected deduplicated article sections, got %#v", article["articleSection"])
	}
	publisher, ok := article["publisher"].(map[string]any)
	if !ok || publisher["name"] != "Example Publisher" || publisher["url"] != "https://publisher.example/about" {
		t.Fatalf("unexpected publisher: %#v", article["publisher"])
	}
	breadcrumbs, ok := structuredData[1].(map[string]any)
	if !ok || breadcrumbs["@type"] != "BreadcrumbList" {
		t.Fatalf("expected breadcrumb schema, got %#v", structuredData[1])
	}
	items, ok := breadcrumbs["itemListElement"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("expected ancestor, category and article breadcrumb items, got %#v", breadcrumbs["itemListElement"])
	}
	category := items[1].(map[string]any)
	if category["item"] != "https://example.test/categories/engineering" || category["position"] != 2 {
		t.Fatalf("unexpected category breadcrumb: %#v", category)
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

func TestPublishedArticleStructuredDataRejectsUnsafeCanonical(t *testing.T) {
	for _, canonical := range []string{"", "http://example.test/article", "https://user:secret@example.test/article", "javascript:alert(1)"} {
		post := PublishedPost{Title: "Title", SEO: PublishedSEO{CanonicalURL: canonical}}
		if structuredData := publishedArticleStructuredData(post); len(structuredData) != 0 {
			t.Fatalf("expected no structured data for unsafe canonical %q, got %#v", canonical, structuredData)
		}
	}
}
