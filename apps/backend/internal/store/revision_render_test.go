package store

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRenderRevisionBodySanitizesAndBuildsDerivedRepresentations(t *testing.T) {
	document := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{"type": "heading", "attrs": map[string]any{"level": 2}},
		},
	}
	rendered, err := renderRevisionBody(document, `
		<h2 onclick="alert(1)">Safe heading</h2>
		<p>Read <a href="javascript:alert(1)" style="color:red">this</a> and <strong>learn</strong>.</p>
		<script>alert(1)</script>
		<h2 id="safe-heading">Safe heading</h2>
	`, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"onclick", "javascript:", "style=", "<script", "alert(1)"} {
		if strings.Contains(strings.ToLower(rendered.HTML), strings.ToLower(forbidden)) {
			t.Fatalf("sanitized HTML still contains %q: %s", forbidden, rendered.HTML)
		}
	}
	if !strings.Contains(rendered.HTML, `<h2 id="safe-heading">Safe heading</h2>`) {
		t.Fatalf("expected generated stable heading ID, got %s", rendered.HTML)
	}
	if !strings.Contains(rendered.HTML, `<h2 id="safe-heading-2">Safe heading</h2>`) {
		t.Fatalf("expected duplicate heading ID to be made unique, got %s", rendered.HTML)
	}
	if rendered.PlainText != "Safe heading Read this and learn . Safe heading" {
		t.Fatalf("unexpected plain text: %q", rendered.PlainText)
	}
	if !strings.Contains(rendered.Markdown, "## Safe heading") || !strings.Contains(rendered.Markdown, "**learn**") {
		t.Fatalf("unexpected Markdown export: %q", rendered.Markdown)
	}
	var toc []tableOfContentsEntry
	if err := json.Unmarshal([]byte(rendered.TableOfContents), &toc); err != nil {
		t.Fatal(err)
	}
	if len(toc) != 2 || toc[0].ID != "safe-heading" || toc[1].ID != "safe-heading-2" {
		t.Fatalf("unexpected table of contents: %#v", toc)
	}
}

func TestRenderRevisionBodyRejectsH1RawNodesAndMissingImageAlt(t *testing.T) {
	tests := []struct {
		name     string
		document any
		html     string
	}{
		{
			name:     "HTML H1",
			document: map[string]any{"type": "doc", "content": []any{}},
			html:     `<h1>Duplicate page title</h1>`,
		},
		{
			name:     "unsupported HTML heading depth",
			document: map[string]any{"type": "doc", "content": []any{}},
			html:     `<h5>Too deeply nested</h5>`,
		},
		{
			name: "structured H1",
			document: map[string]any{
				"type": "doc",
				"content": []any{map[string]any{
					"type":  "heading",
					"attrs": map[string]any{"level": float64(1)},
				}},
			},
			html: `<p>Body</p>`,
		},
		{
			name: "raw structured HTML",
			document: map[string]any{
				"type":    "doc",
				"content": []any{map[string]any{"type": "raw_html"}},
			},
			html: `<p>Body</p>`,
		},
		{
			name: "unsafe structured link mark",
			document: map[string]any{
				"type": "doc",
				"content": []any{map[string]any{
					"type": "paragraph",
					"content": []any{map[string]any{
						"type": "text",
						"text": "unsafe",
						"marks": []any{map[string]any{
							"type":  "link",
							"attrs": map[string]any{"href": "javascript:alert(1)"},
						}},
					}},
				}},
			},
			html: `<p>Body</p>`,
		},
		{
			name:     "image missing alt",
			document: map[string]any{"type": "doc", "content": []any{}},
			html:     `<p><img src="https://cdn.example.test/image.jpg"></p>`,
		},
		{
			name:     "unsafe image source",
			document: map[string]any{"type": "doc", "content": []any{}},
			html:     `<img src="javascript:alert(1)" alt="Unsafe">`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := renderRevisionBody(test.document, test.html, "Title")
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestSafeRevisionURLSupportsMailAndRejectsCredentials(t *testing.T) {
	if !safeRevisionURL("mailto:editor@example.test", true) {
		t.Fatal("expected a mailto editorial link to be allowed")
	}
	if safeRevisionURL("https://user:password@example.test/private", true) {
		t.Fatal("expected URL credentials to be rejected")
	}
	if safeRevisionURL(`/\evil.example.test/path`, true) {
		t.Fatal("expected a backslash-based network-path URL to be rejected")
	}
}

func TestRenderRevisionBodyAllowsExplicitDecorativeImage(t *testing.T) {
	rendered, err := renderRevisionBody(
		map[string]any{"type": "doc", "content": []any{}},
		`<img src="/media/divider.png" data-decorative="true" onclick="bad()">`,
		"Title",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.HTML, `alt=""`) || !strings.Contains(rendered.HTML, `loading="lazy"`) {
		t.Fatalf("decorative image contract missing: %s", rendered.HTML)
	}
	if strings.Contains(rendered.HTML, "onclick") {
		t.Fatalf("unsafe image attribute survived: %s", rendered.HTML)
	}
}

func TestRenderRevisionBodyAcceptsStructuredEditorOutput(t *testing.T) {
	document := map[string]any{
		"type":          "doc",
		"schemaVersion": "tiptap-v1",
		"content": []any{
			map[string]any{
				"type":    "heading",
				"attrs":   map[string]any{"level": 2, "id": "editor-heading"},
				"content": []any{map[string]any{"type": "text", "text": "Editor heading"}},
			},
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": "Structured ", "marks": []any{map[string]any{"type": "bold"}}},
					map[string]any{
						"type": "text",
						"text": "link",
						"marks": []any{map[string]any{
							"type":  "link",
							"attrs": map[string]any{"href": "https://example.test/reference"},
						}},
					},
				},
			},
			map[string]any{
				"type": "table",
				"content": []any{map[string]any{
					"type": "tableRow",
					"content": []any{map[string]any{
						"type": "tableHeader",
						"content": []any{map[string]any{
							"type":    "paragraph",
							"content": []any{map[string]any{"type": "text", "text": "Column"}},
						}},
					}},
				}},
			},
		},
	}
	rendered, err := renderRevisionBody(
		document,
		`<h2 id="editor-heading">Editor heading</h2><p><strong>Structured </strong><u>content</u> with <a href="https://example.test/reference">link</a>.</p><table><tbody><tr><th>Column</th></tr><tr><td>Value</td></tr></tbody></table><img src="/media/chart.png" alt="Quarterly chart">`,
		"Fallback",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`<h2 id="editor-heading">Editor heading</h2>`,
		`<u>content</u>`,
		`href="https://example.test/reference" rel="noopener noreferrer"`,
		`<table>`,
		`src="/media/chart.png" alt="Quarterly chart" loading="lazy"`,
	} {
		if !strings.Contains(rendered.HTML, expected) {
			t.Fatalf("structured editor HTML is missing %q: %s", expected, rendered.HTML)
		}
	}
	if !strings.Contains(rendered.DocumentJSON, `"schemaVersion":"tiptap-v1"`) {
		t.Fatalf("structured document schema version was not retained: %s", rendered.DocumentJSON)
	}
	if !strings.Contains(rendered.Markdown, "## Editor heading") || !strings.Contains(rendered.Markdown, "**Structured **") {
		t.Fatalf("unexpected editor Markdown export: %q", rendered.Markdown)
	}
}

func TestRenderRevisionBodyPreservesSupportedEditorialBlocks(t *testing.T) {
	document := map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type": "editorialBlock",
			"attrs": map[string]any{"kind": "takeaway"},
			"content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Keep the source nearby."}}}},
		}},
	}
	rendered, err := renderRevisionBody(document, `<aside data-editorial-block="takeaway" onclick="bad()"><p>Keep the source nearby.</p></aside>`, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.HTML, `<aside data-editorial-block="takeaway"><p>Keep the source nearby.</p></aside>`) {
		t.Fatalf("editorial block contract missing: %s", rendered.HTML)
	}
	if strings.Contains(rendered.HTML, "onclick") {
		t.Fatalf("unsafe editorial block attribute survived: %s", rendered.HTML)
	}
}

func TestRenderRevisionBodyRejectsUnknownEditorialBlockKind(t *testing.T) {
	err := validateStructuredRevisionDocument(map[string]any{
		"type": "doc",
		"content": []any{map[string]any{"type": "editorialBlock", "attrs": map[string]any{"kind": "unsafe"}}},
	})
	if err == nil {
		t.Fatal("expected unsupported editorial block kind to be rejected")
	}
}

func TestRenderRevisionBodyPreservesProjectCitation(t *testing.T) {
	document := map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type": "paragraph",
			"content": []any{map[string]any{
				"type": "citation",
				"attrs": map[string]any{"sourceId": "source-123", "href": "https://example.test/evidence"},
				"content": []any{map[string]any{"type": "text", "text": "Primary evidence"}},
			}},
		}},
	}
	rendered, err := renderRevisionBody(document, `<p><cite data-source-id="source-123" onclick="bad()"><a href="https://example.test/evidence">Primary evidence</a></cite></p>`, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`<cite data-source-id="source-123">`, `href="https://example.test/evidence" rel="noopener noreferrer"`} {
		if !strings.Contains(rendered.HTML, expected) {
			t.Fatalf("citation contract missing %q: %s", expected, rendered.HTML)
		}
	}
	if strings.Contains(rendered.HTML, "onclick") {
		t.Fatalf("unsafe citation attribute survived: %s", rendered.HTML)
	}
}

func TestRenderRevisionBodyRejectsInvalidCitationReference(t *testing.T) {
	err := validateStructuredRevisionDocument(map[string]any{
		"type": "doc",
		"content": []any{map[string]any{"type": "citation", "attrs": map[string]any{"sourceId": "../../source"}}},
	})
	if err == nil {
		t.Fatal("expected invalid source reference to be rejected")
	}
}
