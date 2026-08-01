package store

import (
	"net/url"
	"path"
	"strings"
	"time"
)

func publishedArticleStructuredData(post PublishedPost) []any {
	canonical, ok := safeStructuredDataURL(post.SEO.CanonicalURL, true)
	if !ok {
		return nil
	}

	articleType := "BlogPosting"
	if post.ArticleType == "news_update" {
		articleType = "NewsArticle"
	}
	article := map[string]any{
		"@context":         "https://schema.org",
		"@type":            articleType,
		"@id":              canonical + "#article",
		"url":              canonical,
		"mainEntityOfPage": canonical,
		"headline":         strings.TrimSpace(post.Title),
	}
	setNonEmptyString(article, "description", post.SEO.Description)
	setNonEmptyString(article, "inLanguage", "en")
	setStructuredDataDate(article, "datePublished", post.PublishedAt)
	setStructuredDataDate(article, "dateModified", post.ModifiedAt)

	if authors := structuredDataAuthors(post.Authors); len(authors) > 0 {
		article["author"] = authors
	}
	if publisher := structuredDataPublisher(post.PublisherName, post.PublisherURL); publisher != nil {
		article["publisher"] = publisher
	}
	if images := structuredDataImages(post, canonical); len(images) > 0 {
		article["image"] = images
	}
	if sections := structuredDataSections(post.Taxonomy); len(sections) > 0 {
		article["articleSection"] = sections
	}
	if keywords := taxonomyNames(post.Taxonomy.Tags); len(keywords) > 0 {
		article["keywords"] = keywords
	}

	result := []any{article}
	if breadcrumbs := structuredDataBreadcrumbs(post, canonical); breadcrumbs != nil {
		result = append(result, breadcrumbs)
	}
	return result
}

func structuredDataImages(post PublishedPost, canonical string) []string {
	images := []string{}
	if image := structuredDataImage(canonical, openGraphImage(post.SEO.OpenGraph)); image != "" {
		images = appendUniqueString(images, image)
	}
	if post.Media.Hero == nil {
		return images
	}
	if image := structuredDataImage(canonical, post.Media.Hero.URL); image != "" {
		images = appendUniqueString(images, image)
	}
	for _, variant := range post.Media.Hero.Variants {
		if image := structuredDataImage(canonical, variant.URL); image != "" {
			images = appendUniqueString(images, image)
		}
	}
	return images
}

func structuredDataBreadcrumbs(post PublishedPost, canonical string) map[string]any {
	primary := post.Taxonomy.PrimaryCategory
	if primary == nil || strings.TrimSpace(primary.Name) == "" || strings.TrimSpace(primary.Slug) == "" {
		return nil
	}
	terms := append([]TaxonomyTerm{}, primary.Ancestors...)
	terms = append(terms, *primary)
	items := make([]any, 0, len(terms)+1)
	position := 1
	for _, term := range terms {
		name := strings.TrimSpace(term.Name)
		itemURL := structuredDataCategoryURL(canonical, term.Slug)
		if name == "" || itemURL == "" {
			continue
		}
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": position,
			"name":     name,
			"item":     itemURL,
		})
		position++
	}
	if title := strings.TrimSpace(post.Title); title != "" {
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": position,
			"name":     title,
			"item":     canonical,
		})
	}
	if len(items) < 2 {
		return nil
	}
	return map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"@id":             canonical + "#breadcrumbs",
		"itemListElement": items,
	}
}

func structuredDataCategoryURL(canonical, slug string) string {
	base, err := url.Parse(canonical)
	if err != nil {
		return ""
	}
	slug = strings.TrimSpace(slug)
	if slug == "" || strings.ContainsAny(slug, "/\\\x00\r\n") {
		return ""
	}
	base.Path = path.Join("/categories", url.PathEscape(slug))
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	value, ok := safeStructuredDataURL(base.String(), true)
	if !ok {
		return ""
	}
	return value
}

func structuredDataAuthors(authors []Author) []any {
	result := make([]any, 0, len(authors))
	for _, author := range authors {
		name := strings.TrimSpace(author.DisplayName)
		if name == "" {
			continue
		}
		value := map[string]any{
			"@type": "Person",
			"name":  name,
		}
		if profileURL, ok := safeStructuredDataURL(author.ProfileURL, false); ok {
			value["url"] = profileURL
		}
		setNonEmptyString(value, "jobTitle", author.JobTitle)
		if organization := strings.TrimSpace(author.Organization); organization != "" {
			value["affiliation"] = map[string]any{
				"@type": "Organization",
				"name":  organization,
			}
		}
		sameAs := make([]string, 0, len(author.SameAs))
		for _, candidate := range author.SameAs {
			if externalURL, ok := safeStructuredDataURL(candidate, false); ok {
				sameAs = appendUniqueString(sameAs, externalURL)
			}
		}
		if len(sameAs) > 0 {
			value["sameAs"] = sameAs
		}
		result = append(result, value)
	}
	return result
}

func structuredDataPublisher(name, rawURL string) map[string]any {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	publisher := map[string]any{
		"@type": "Organization",
		"name":  name,
	}
	if publisherURL, ok := safeStructuredDataURL(rawURL, false); ok {
		publisher["url"] = publisherURL
	}
	return publisher
}

func structuredDataSections(taxonomy PublishedTaxonomy) []string {
	sections := []string{}
	if taxonomy.PrimaryCategory != nil {
		sections = appendUniqueString(sections, strings.TrimSpace(taxonomy.PrimaryCategory.Name))
	}
	for _, category := range taxonomy.Categories {
		sections = appendUniqueString(sections, strings.TrimSpace(category.Name))
	}
	return sections
}

func taxonomyNames(terms []TaxonomyTerm) []string {
	values := make([]string, 0, len(terms))
	for _, term := range terms {
		values = appendUniqueString(values, strings.TrimSpace(term.Name))
	}
	return values
}

func openGraphImage(openGraph any) string {
	value, ok := openGraph.(map[string]any)
	if !ok {
		return ""
	}
	image, _ := value["image"].(string)
	return strings.TrimSpace(image)
}

func structuredDataImage(canonical, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || strings.ContainsAny(candidate, "\x00\r\n\\") {
		return ""
	}
	if image, ok := safeStructuredDataURL(candidate, true); ok {
		return image
	}
	if !strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "//") {
		return ""
	}
	base, err := url.Parse(canonical)
	if err != nil {
		return ""
	}
	reference, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(reference).String()
	if image, ok := safeStructuredDataURL(resolved, true); ok {
		return image
	}
	return ""
}

func safeStructuredDataURL(raw string, requireHTTPS bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\\") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (requireHTTPS && scheme != "https") || (!requireHTTPS && scheme != "http" && scheme != "https") {
		return "", false
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), true
}

func setStructuredDataDate(target map[string]any, key, raw string) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	parsed, err := parseSQLiteTime(strings.TrimSpace(raw))
	if err == nil {
		target[key] = parsed.UTC().Format(time.RFC3339)
	}
}

func setNonEmptyString(target map[string]any, key, value string) {
	if value = strings.TrimSpace(value); value != "" {
		target[key] = value
	}
}

func appendUniqueString(values []string, candidate string) []string {
	if candidate == "" {
		return values
	}
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}
