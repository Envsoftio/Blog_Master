package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type renderedRevisionBody struct {
	DocumentJSON    string
	HTML            string
	PlainText       string
	Markdown        string
	TableOfContents string
}

type tableOfContentsEntry struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
	Text  string `json:"text"`
}

var safeHTMLIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,127}$`)

var allowedRevisionElements = map[string]struct{}{
	"p": {}, "h2": {}, "h3": {}, "h4": {},
	"ul": {}, "ol": {}, "li": {}, "strong": {}, "em": {}, "u": {}, "s": {},
	"blockquote": {}, "pre": {}, "code": {}, "a": {}, "br": {}, "hr": {},
	"figure": {}, "figcaption": {}, "img": {}, "table": {}, "thead": {},
	"tbody": {}, "tfoot": {}, "tr": {}, "th": {}, "td": {}, "sup": {}, "sub": {}, "aside": {},
}

var droppedRevisionElements = map[string]struct{}{
	"script": {}, "style": {}, "iframe": {}, "object": {}, "embed": {},
	"form": {}, "input": {}, "button": {}, "textarea": {}, "select": {},
	"svg": {}, "math": {}, "template": {}, "meta": {}, "link": {}, "base": {},
}

func renderRevisionBody(document any, rawHTML, title string) (renderedRevisionBody, error) {
	if document == nil {
		document = map[string]any{"type": "doc", "content": []any{}}
	}
	if err := validateStructuredRevisionDocument(document); err != nil {
		return renderedRevisionBody{}, err
	}
	documentBytes, err := json.Marshal(document)
	if err != nil {
		return renderedRevisionBody{}, fmt.Errorf("encode structured body: %w", err)
	}
	if strings.TrimSpace(rawHTML) == "" {
		rawHTML = "<p>" + htmlpkg.EscapeString(title) + "</p>"
	}

	contextNode := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(rawHTML), contextNode)
	if err != nil {
		return renderedRevisionBody{}, fmt.Errorf("%w: body HTML could not be parsed", ErrValidation)
	}
	root := &html.Node{Type: html.ElementNode, Data: "div"}
	for _, node := range nodes {
		root.AppendChild(node)
	}

	usedIDs := map[string]struct{}{}
	toc := make([]tableOfContentsEntry, 0)
	if err := sanitizeRevisionChildren(root, usedIDs, &toc); err != nil {
		return renderedRevisionBody{}, err
	}
	var htmlOutput strings.Builder
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&htmlOutput, child); err != nil {
			return renderedRevisionBody{}, fmt.Errorf("render sanitized body: %w", err)
		}
	}
	plainText := normalizeRenderedText(nodeText(root))
	markdown := strings.TrimSpace(renderMarkdownChildren(root))
	tocJSON, err := json.Marshal(toc)
	if err != nil {
		return renderedRevisionBody{}, fmt.Errorf("encode table of contents: %w", err)
	}
	return renderedRevisionBody{
		DocumentJSON:    string(documentBytes),
		HTML:            htmlOutput.String(),
		PlainText:       plainText,
		Markdown:        compactMarkdown(markdown),
		TableOfContents: string(tocJSON),
	}, nil
}

func validateStructuredRevisionDocument(document any) error {
	root, ok := document.(map[string]any)
	if !ok {
		encoded, err := json.Marshal(document)
		if err != nil {
			return fmt.Errorf("%w: bodyDocument must be JSON", ErrValidation)
		}
		if err := json.Unmarshal(encoded, &root); err != nil {
			return fmt.Errorf("%w: bodyDocument must be a structured document", ErrValidation)
		}
	}
	if nodeType, _ := root["type"].(string); nodeType != "doc" {
		return fmt.Errorf("%w: bodyDocument root type must be doc", ErrValidation)
	}
	return validateStructuredRevisionNode(root)
}

func validateStructuredRevisionNode(node map[string]any) error {
	nodeType, _ := node["type"].(string)
	normalizedType := strings.ToLower(strings.TrimSpace(nodeType))
	switch normalizedType {
	case "html", "raw_html", "rawhtml", "script", "style":
		return fmt.Errorf("%w: raw HTML body nodes are disabled", ErrValidation)
	case "heading":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if level, ok := jsonNumberAsInt(attrs["level"]); ok && (level < 2 || level > 4) {
				return fmt.Errorf("%w: article body headings must use H2 through H4", ErrValidation)
			}
		}
	case "link":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if href, _ := attrs["href"].(string); href != "" && !safeRevisionURL(href, true) {
				return fmt.Errorf("%w: structured links must use HTTPS, mailto, a document anchor, or a root-relative URL", ErrValidation)
			}
		}
	case "image":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if src, _ := attrs["src"].(string); src != "" && !safeRevisionURL(src, false) {
				return fmt.Errorf("%w: structured image URLs must use HTTPS or a root-relative URL", ErrValidation)
			}
		}
	case "editorialblock":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if kind, _ := attrs["kind"].(string); !safeEditorialBlockKind(kind) {
				return fmt.Errorf("%w: structured editorial blocks must use a supported kind", ErrValidation)
			}
		}
	}
	if marks, ok := node["marks"].([]any); ok {
		for _, rawMark := range marks {
			mark, ok := rawMark.(map[string]any)
			if !ok || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(mark["type"])), "link") {
				continue
			}
			attrs, _ := mark["attrs"].(map[string]any)
			href, _ := attrs["href"].(string)
			if href == "" || !safeRevisionURL(href, true) {
				return fmt.Errorf("%w: structured link marks require a safe href", ErrValidation)
			}
		}
	}
	children, _ := node["content"].([]any)
	for _, child := range children {
		childNode, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if err := validateStructuredRevisionNode(childNode); err != nil {
			return err
		}
	}
	return nil
}

func jsonNumberAsInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), typed == float64(int(typed))
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	default:
		return 0, false
	}
}

func sanitizeRevisionChildren(parent *html.Node, usedIDs map[string]struct{}, toc *[]tableOfContentsEntry) error {
	for node := parent.FirstChild; node != nil; {
		next := node.NextSibling
		switch node.Type {
		case html.CommentNode, html.DoctypeNode:
			parent.RemoveChild(node)
		case html.ElementNode:
			tag := strings.ToLower(node.Data)
			if tag == "h1" || tag == "h5" || tag == "h6" {
				return fmt.Errorf("%w: article body headings must use H2 through H4", ErrValidation)
			}
			if _, blocked := droppedRevisionElements[tag]; blocked {
				parent.RemoveChild(node)
				break
			}
			if _, allowed := allowedRevisionElements[tag]; !allowed {
				if err := sanitizeRevisionChildren(node, usedIDs, toc); err != nil {
					return err
				}
				for child := node.FirstChild; child != nil; {
					childNext := child.NextSibling
					node.RemoveChild(child)
					parent.InsertBefore(child, node)
					child = childNext
				}
				parent.RemoveChild(node)
				break
			}
			if err := sanitizeRevisionElement(node); err != nil {
				return err
			}
			if err := sanitizeRevisionChildren(node, usedIDs, toc); err != nil {
				return err
			}
			if level := headingLevel(tag); level > 0 {
				text := normalizeRenderedText(nodeText(node))
				id := attributeValue(node, "id")
				if !safeHTMLIDPattern.MatchString(id) {
					id = headingSlug(text)
				}
				if id == "" {
					id = "section"
				}
				id = uniqueHeadingID(id, usedIDs)
				setAttribute(node, "id", id)
				*toc = append(*toc, tableOfContentsEntry{ID: id, Level: level, Text: text})
			}
		}
		node = next
	}
	return nil
}

func sanitizeRevisionElement(node *html.Node) error {
	tag := strings.ToLower(node.Data)
	attributes := make([]html.Attribute, 0, len(node.Attr))
	for _, attr := range node.Attr {
		name := strings.ToLower(attr.Key)
		value := strings.TrimSpace(attr.Val)
		switch tag {
		case "aside":
			if name == "data-editorial-block" && safeEditorialBlockKind(value) {
				attributes = append(attributes, html.Attribute{Key: name, Val: value})
			}
		case "a":
			if name == "href" && safeRevisionURL(value, true) {
				attributes = append(attributes, html.Attribute{Key: "href", Val: value})
			} else if name == "title" && len(value) <= 300 {
				attributes = append(attributes, html.Attribute{Key: "title", Val: value})
			}
		case "img":
			if name == "src" && safeRevisionURL(value, false) {
				attributes = append(attributes, html.Attribute{Key: "src", Val: value})
			} else if name == "alt" && len(value) <= 1000 {
				attributes = append(attributes, html.Attribute{Key: "alt", Val: value})
			} else if (name == "width" || name == "height") && positiveDimension(value) {
				attributes = append(attributes, html.Attribute{Key: name, Val: value})
			} else if name == "data-decorative" && strings.EqualFold(value, "true") {
				attributes = append(attributes, html.Attribute{Key: name, Val: "true"})
			}
		case "h2", "h3", "h4", "h5", "h6":
			if name == "id" && safeHTMLIDPattern.MatchString(value) {
				attributes = append(attributes, html.Attribute{Key: "id", Val: value})
			}
		case "th":
			if name == "scope" && (value == "row" || value == "col" || value == "rowgroup" || value == "colgroup") {
				attributes = append(attributes, html.Attribute{Key: "scope", Val: value})
			} else if (name == "colspan" || name == "rowspan") && boundedSpan(value) {
				attributes = append(attributes, html.Attribute{Key: name, Val: value})
			}
		case "td":
			if (name == "colspan" || name == "rowspan") && boundedSpan(value) {
				attributes = append(attributes, html.Attribute{Key: name, Val: value})
			}
		case "code":
			if name == "class" && strings.HasPrefix(value, "language-") && len(value) <= 80 {
				attributes = append(attributes, html.Attribute{Key: "class", Val: value})
			}
		}
	}
	if tag == "a" && attributeValueFrom(attributes, "href") != "" {
		attributes = append(attributes, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
	}
	if tag == "img" {
		if attributeValueFrom(attributes, "src") == "" {
			return fmt.Errorf("%w: images require a safe HTTPS or root-relative source", ErrValidation)
		}
		alt := attributeValueFrom(attributes, "alt")
		decorative := attributeValueFrom(attributes, "data-decorative") == "true"
		if alt == "" && !decorative {
			return fmt.Errorf("%w: meaningful images require alt text", ErrValidation)
		}
		if decorative {
			attributes = upsertAttribute(attributes, "alt", "")
		}
		attributes = upsertAttribute(attributes, "loading", "lazy")
	}
	node.Attr = attributes
	return nil
}

func safeEditorialBlockKind(value string) bool {
	switch value {
	case "callout", "takeaway", "steps", "pros-cons", "cta", "faq":
		return true
	default:
		return false
	}
}

func safeRevisionURL(raw string, allowAnchor bool) bool {
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\\") {
		return false
	}
	if allowAnchor && strings.HasPrefix(raw, "#") {
		return true
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if allowAnchor && scheme == "mailto" {
		return parsed.Opaque != "" && !strings.ContainsAny(parsed.Opaque, "<>\"")
	}
	return scheme == "https" && parsed.Host != ""
}

func positiveDimension(raw string) bool {
	value, err := strconv.Atoi(raw)
	return err == nil && value > 0 && value <= 20000
}

func boundedSpan(raw string) bool {
	value, err := strconv.Atoi(raw)
	return err == nil && value > 0 && value <= 100
}

func headingLevel(tag string) int {
	if len(tag) != 2 || tag[0] != 'h' || tag[1] < '2' || tag[1] > '6' {
		return 0
	}
	return int(tag[1] - '0')
}

func headingSlug(value string) string {
	var builder strings.Builder
	separator := false
	for _, char := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(char) || unicode.IsDigit(char):
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			separator = false
			builder.WriteRune(char)
		case builder.Len() > 0:
			separator = true
		}
		if builder.Len() >= 96 {
			break
		}
	}
	return strings.Trim(builder.String(), "-")
}

func uniqueHeadingID(base string, used map[string]struct{}) string {
	if _, exists := used[base]; !exists {
		used[base] = struct{}{}
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return builder.String()
}

func normalizeRenderedText(value string) string {
	return strings.Join(strings.Fields(htmlpkg.UnescapeString(value)), " ")
}

func renderMarkdownChildren(parent *html.Node) string {
	var builder strings.Builder
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		renderMarkdownNode(&builder, child, 0)
	}
	return builder.String()
}

func renderMarkdownNode(builder *strings.Builder, node *html.Node, depth int) {
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
		return
	}
	if node.Type != html.ElementNode {
		return
	}
	tag := strings.ToLower(node.Data)
	switch tag {
	case "h2", "h3", "h4", "h5", "h6":
		builder.WriteString(strings.Repeat("#", headingLevel(tag)))
		builder.WriteByte(' ')
		renderMarkdownInline(builder, node)
		builder.WriteString("\n\n")
	case "p":
		renderMarkdownInline(builder, node)
		builder.WriteString("\n\n")
	case "blockquote":
		text := normalizeRenderedText(nodeText(node))
		for _, line := range strings.Split(text, "\n") {
			builder.WriteString("> ")
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	case "pre":
		builder.WriteString("```\n")
		builder.WriteString(strings.TrimSpace(nodeText(node)))
		builder.WriteString("\n```\n\n")
	case "ul", "ol":
		index := 1
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode || child.Data != "li" {
				continue
			}
			builder.WriteString(strings.Repeat("  ", depth))
			if tag == "ol" {
				builder.WriteString(strconv.Itoa(index))
				builder.WriteString(". ")
				index++
			} else {
				builder.WriteString("- ")
			}
			renderMarkdownInline(builder, child)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	case "hr":
		builder.WriteString("---\n\n")
	case "br":
		builder.WriteString("  \n")
	case "figure", "figcaption", "table", "thead", "tbody", "tfoot", "tr", "th", "td":
		text := normalizeRenderedText(nodeText(node))
		if text != "" {
			builder.WriteString(text)
			builder.WriteString("\n\n")
		}
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderMarkdownNode(builder, child, depth+1)
		}
	}
}

func renderMarkdownInline(builder *strings.Builder, node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			builder.WriteString(child.Data)
			continue
		}
		if child.Type != html.ElementNode {
			continue
		}
		switch child.Data {
		case "strong":
			builder.WriteString("**")
			renderMarkdownInline(builder, child)
			builder.WriteString("**")
		case "em":
			builder.WriteByte('*')
			renderMarkdownInline(builder, child)
			builder.WriteByte('*')
		case "s":
			builder.WriteString("~~")
			renderMarkdownInline(builder, child)
			builder.WriteString("~~")
		case "code":
			builder.WriteByte('`')
			builder.WriteString(strings.TrimSpace(nodeText(child)))
			builder.WriteByte('`')
		case "a":
			builder.WriteByte('[')
			renderMarkdownInline(builder, child)
			builder.WriteString("](")
			builder.WriteString(attributeValue(child, "href"))
			builder.WriteByte(')')
		case "img":
			builder.WriteString("![")
			builder.WriteString(attributeValue(child, "alt"))
			builder.WriteString("](")
			builder.WriteString(attributeValue(child, "src"))
			builder.WriteByte(')')
		case "br":
			builder.WriteString("  \n")
		default:
			renderMarkdownInline(builder, child)
		}
	}
}

func attributeValue(node *html.Node, name string) string {
	return attributeValueFrom(node.Attr, name)
}

func attributeValueFrom(attributes []html.Attribute, name string) string {
	for _, attribute := range attributes {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}

func setAttribute(node *html.Node, name, value string) {
	node.Attr = upsertAttribute(node.Attr, name, value)
}

func upsertAttribute(attributes []html.Attribute, name, value string) []html.Attribute {
	for index := range attributes {
		if strings.EqualFold(attributes[index].Key, name) {
			attributes[index].Key = name
			attributes[index].Val = value
			return attributes
		}
	}
	return append(attributes, html.Attribute{Key: name, Val: value})
}

func compactMarkdown(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	var output bytes.Buffer
	blank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			if blank || output.Len() == 0 {
				continue
			}
			blank = true
			output.WriteByte('\n')
			continue
		}
		blank = false
		output.WriteString(line)
		output.WriteByte('\n')
	}
	return strings.TrimSpace(output.String())
}
