package httpapi

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"seoblog/apps/backend/internal/store"
)

func (s *Server) registerContentRoutes() {
	preview := s.app.Group(
		"/content/v1/preview",
		contentSourceRateLimiter(),
	)
	preview.Get("/revisions/:revisionID", s.requirePreviewToken, previewTokenRateLimiter(), s.getPreviewRevision)

	content := s.app.Group(
		"/content/v1",
		contentSourceRateLimiter(),
		s.requireContentKey,
		contentKeyRateLimiter(),
		contentProjectRateLimiter(),
	)

	content.Get("/posts", requireContentScope("content:published:read"), s.listPublishedPosts)
	content.Get("/posts/by-id/:contentID", requireContentScope("content:published:read"), s.getPublishedPostByID)
	content.Get("/posts/:slug/related", requireContentScope("content:published:read"), s.getRelatedPosts)
	content.Get("/posts/:slug", requireContentScope("content:published:read"), s.getPublishedPostBySlug)
	content.Head("/posts/:slug", requireContentScope("content:published:read"), s.headPublishedPostBySlug)
	content.Get("/categories", requireContentScope("taxonomy:published:read"), s.listCategories)
	content.Get("/categories/:slug", requireContentScope("taxonomy:published:read"), s.getCategory)
	content.Get("/tags", requireContentScope("taxonomy:published:read"), s.listTags)
	content.Get("/tags/:slug", requireContentScope("taxonomy:published:read"), s.getTag)
	content.Get("/authors", requireContentScope("authors:published:read"), s.listAuthors)
	content.Get("/authors/:slug", requireContentScope("authors:published:read"), s.getAuthor)
	content.Get("/series", requireContentScope("taxonomy:published:read"), s.listSeries)
	content.Get("/series/:slug", requireContentScope("taxonomy:published:read"), s.getSeries)
	content.Get("/feed-data", requireContentScope("discovery:read"), s.feedData)
	content.Get("/discovery-manifest", requireContentScope("discovery:read"), s.discoveryManifest)
	content.Get("/redirects", requireContentScope("redirects:read"), s.redirects)
	content.Get("/changes", requireContentScope("discovery:read"), s.changes)
}

func (s *Server) requirePreviewToken(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return problem(c, fiber.StatusUnauthorized, "Missing preview token", "Use Authorization: Bearer <preview-token>")
	}
	secret := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if secret == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing preview token", "The bearer token is empty")
	}
	token, err := s.store.FindPreviewToken(c.UserContext(), secret, c.Params("revisionID"))
	if err != nil {
		s.logger.Warn("preview token rejected", "error", err)
		return problem(c, fiber.StatusUnauthorized, "Invalid preview token", "The preview token is invalid, expired or revoked")
	}
	c.Locals(previewContextKey, token)
	return c.Next()
}

func previewTokenRateLimiter() fiber.Handler {
	return newContentRateLimiter(300, func(c *fiber.Ctx) string {
		preview, _ := previewContext(c)
		return "preview-token:" + preview.ID
	})
}

func previewContext(c *fiber.Ctx) (store.PreviewTokenContext, bool) {
	value := c.Locals(previewContextKey)
	if value == nil {
		return store.PreviewTokenContext{}, false
	}
	ctx, ok := value.(store.PreviewTokenContext)
	return ctx, ok
}

func (s *Server) getPreviewRevision(c *fiber.Ctx) error {
	preview, ok := previewContext(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing preview context", "")
	}
	post, err := s.store.GetPreviewPost(c.UserContext(), preview.ProjectID, preview.ArticleID, preview.RevisionID)
	if err != nil {
		return s.publishedReadError(c, err, "Preview not found", "Could not load preview")
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	c.Set("X-Robots-Tag", "noindex, nofollow")
	return writeJSON(c, fiber.StatusOK, Envelope[store.PublishedPost]{
		Data: post,
		Meta: MetaData{ProjectID: preview.ProjectID, ETag: quotedETag(post.ContentHash)},
	})
}

func (s *Server) listPublishedPosts(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	limit := boundedLimit(c.Query("limit", "20"), 50)
	cursor, err := decodeCursor[store.PublishedCursor](c.Query("cursor"))
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "The cursor is malformed")
	}
	posts, err := s.store.ListPublishedPosts(
		c.UserContext(),
		project.ProjectID,
		c.Query("locale"),
		c.Query("category"),
		c.Query("tag"),
		c.Query("author"),
		c.Query("articleType"),
		c.Query("series"),
		c.Query("categoryMode") == "exact",
		c.Query("publishedFrom"),
		c.Query("publishedTo"),
		cursor,
		limit+1,
	)
	if err != nil {
		s.logger.Error("list published posts", "project_id", project.ProjectID, "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not list posts", "")
	}
	nextCursor := ""
	if len(posts) > limit {
		posts = posts[:limit]
		last := posts[len(posts)-1]
		nextCursor = encodeCursor(store.PublishedCursor{SortAt: last.PaginationKey, ID: last.ID})
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.PublishedPost]{
		Data: posts,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) getPublishedPostBySlug(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	post, err := s.store.GetPublishedPostBySlug(c.UserContext(), project.ProjectID, c.Params("slug"), c.Query("locale", "en"))
	if err != nil {
		return s.publishedReadError(c, err, "Post not found", "Could not load post")
	}
	if setPublishedValidators(c, post) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.PublishedPost]{
		Data: post,
		Meta: MetaData{ProjectID: project.ProjectID, ETag: quotedETag(post.ContentHash)},
	})
}

func (s *Server) headPublishedPostBySlug(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	post, err := s.store.GetPublishedPostBySlug(c.UserContext(), project.ProjectID, c.Params("slug"), c.Query("locale", "en"))
	if err != nil {
		if err == sql.ErrNoRows {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if setPublishedValidators(c, post) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return c.SendStatus(fiber.StatusOK)
}

func (s *Server) getPublishedPostByID(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	post, err := s.store.GetPublishedPostByID(c.UserContext(), project.ProjectID, c.Params("contentID"), c.Query("locale", "en"))
	if err != nil {
		return s.publishedReadError(c, err, "Post not found", "Could not load post")
	}
	if setPublishedValidators(c, post) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.PublishedPost]{
		Data: post,
		Meta: MetaData{ProjectID: project.ProjectID, ETag: quotedETag(post.ContentHash)},
	})
}

func (s *Server) publishedReadError(c *fiber.Ctx, err error, notFoundTitle, internalTitle string) error {
	if err == sql.ErrNoRows {
		return problem(c, fiber.StatusNotFound, notFoundTitle, "")
	}
	s.logger.Error("published content read", "error", err)
	return problem(c, fiber.StatusInternalServerError, internalTitle, "")
}

func (s *Server) getRelatedPosts(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	limit := boundedLimit(c.Query("limit", "6"), 12)
	posts, err := s.store.ListRelatedPosts(c.UserContext(), project.ProjectID, c.Params("slug"), c.Query("locale", "en"), limit)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not load related posts", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.RelatedPost]{
		Data: posts,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: limit},
	})
}

func (s *Server) listCategories(c *fiber.Ctx) error { return s.listTerms(c, "category") }
func (s *Server) getCategory(c *fiber.Ctx) error    { return s.getTerm(c, "category") }
func (s *Server) listTags(c *fiber.Ctx) error       { return s.listTerms(c, "tag") }
func (s *Server) getTag(c *fiber.Ctx) error         { return s.getTerm(c, "tag") }

func (s *Server) listTerms(c *fiber.Ctx, termType string) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	terms, err := s.store.ListTerms(c.UserContext(), project.ProjectID, termType)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not list taxonomy", "")
	}
	page, next, pageErr := paginateByID(terms, c.Query("cursor"), boundedLimit(c.Query("limit", "50"), 100), func(term store.TaxonomyTerm) string { return term.ID })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.TaxonomyTerm]{
		Data: page,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: len(page), NextCursor: next},
	})
}

func (s *Server) getTerm(c *fiber.Ctx, termType string) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	term, err := s.store.GetTerm(c.UserContext(), project.ProjectID, termType, c.Params("slug"))
	if err != nil {
		if err == sql.ErrNoRows {
			prefix := taxonomyRoutePrefix(termType)
			if prefix != "" {
				redirect, redirectErr := s.store.GetRedirect(c.UserContext(), project.ProjectID, prefix+c.Params("slug"))
				if redirectErr == nil && strings.HasPrefix(redirect.TargetPath, prefix) {
					return c.Redirect("/content/v1"+redirect.TargetPath, fiber.StatusMovedPermanently)
				}
				if redirectErr != nil && redirectErr != sql.ErrNoRows {
					return problem(c, fiber.StatusInternalServerError, "Could not load taxonomy redirect", "")
				}
			}
			return problem(c, fiber.StatusNotFound, "Taxonomy term not found", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not load taxonomy term", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.TaxonomyTerm]{Data: term, Meta: MetaData{ProjectID: project.ProjectID}})
}

func taxonomyRoutePrefix(termType string) string {
	switch termType {
	case "category":
		return "/categories/"
	case "tag":
		return "/tags/"
	default:
		return ""
	}
}

func (s *Server) listAuthors(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	authors, err := s.store.ListAuthors(c.UserContext(), project.ProjectID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not list authors", "")
	}
	page, next, pageErr := paginateByID(authors, c.Query("cursor"), boundedLimit(c.Query("limit", "50"), 100), func(author store.Author) string { return author.ID })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.Author]{
		Data: page,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: len(page), NextCursor: next},
	})
}

func (s *Server) getAuthor(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	author, err := s.store.GetAuthor(c.UserContext(), project.ProjectID, c.Params("slug"))
	if err != nil {
		if err == sql.ErrNoRows {
			return problem(c, fiber.StatusNotFound, "Author not found", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not load author", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Author]{Data: author, Meta: MetaData{ProjectID: project.ProjectID}})
}

func (s *Server) listSeries(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	items, err := s.store.ListSeries(c.UserContext(), project.ProjectID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not list series", "")
	}
	page, next, pageErr := paginateByID(items, c.Query("cursor"), boundedLimit(c.Query("limit", "50"), 100), func(item store.Series) string { return item.ID })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.Series]{
		Data: page,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: len(page), NextCursor: next},
	})
}

func (s *Server) getSeries(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	item, err := s.store.GetSeries(c.UserContext(), project.ProjectID, c.Params("slug"))
	if err != nil {
		if err == sql.ErrNoRows {
			return problem(c, fiber.StatusNotFound, "Series not found", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not load series", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Series]{Data: item, Meta: MetaData{ProjectID: project.ProjectID}})
}

func (s *Server) feedData(c *fiber.Ctx) error {
	c.Request().URI().QueryArgs().Set("limit", c.Query("limit", "50"))
	return s.listPublishedPosts(c)
}

func (s *Server) discoveryManifest(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	entries, err := s.store.ListDiscovery(c.UserContext(), project.ProjectID, c.Query("locale"))
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not build discovery manifest", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[map[string]any]{
		Data: map[string]any{"urls": entries},
		Meta: MetaData{ProjectID: project.ProjectID},
	})
}

func (s *Server) redirects(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	items, err := s.store.ListRedirects(c.UserContext(), project.ProjectID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not list redirects", "")
	}
	page, next, pageErr := paginateByID(items, c.Query("cursor"), boundedLimit(c.Query("limit", "100"), 250), func(item store.RedirectRecord) string { return item.SourcePath })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.RedirectRecord]{
		Data: page,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: len(page), NextCursor: next},
	})
}

func (s *Server) changes(c *fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	cursor, err := decodeCursor[store.ChangeCursor](c.Query("after"))
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	limit := boundedLimit(c.Query("limit", "100"), 250)
	items, err := s.store.ListChanges(c.UserContext(), project.ProjectID, cursor, limit+1)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not list changes", "")
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = encodeCursor(store.ChangeCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.ChangeRecord]{
		Data: items,
		Meta: PageMeta{ProjectID: project.ProjectID, Limit: limit, NextCursor: next},
	})
}

func boundedLimit(raw string, max int) int {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return min(20, max)
	}
	if limit > max {
		return max
	}
	return limit
}

func setPublishedValidators(c *fiber.Ctx, post store.PublishedPost) bool {
	etag := quotedETag(post.ContentHash)
	if etag != "" {
		c.Set(fiber.HeaderETag, etag)
	}
	modified := parseDatabaseTime(post.ModifiedAt)
	if modified.IsZero() {
		modified = parseDatabaseTime(post.PublishedAt)
	}
	if !modified.IsZero() {
		c.Set(fiber.HeaderLastModified, modified.UTC().Format(http.TimeFormat))
	}
	c.Set(fiber.HeaderCacheControl, "private, max-age=60, stale-while-revalidate=300, stale-if-error=86400")

	if etag != "" && headerContainsETag(c.Get(fiber.HeaderIfNoneMatch), etag) {
		return true
	}
	if c.Get(fiber.HeaderIfNoneMatch) == "" && !modified.IsZero() {
		if since, err := http.ParseTime(c.Get(fiber.HeaderIfModifiedSince)); err == nil && !modified.After(since) {
			return true
		}
	}
	return false
}

func quotedETag(hash string) string {
	if hash == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(hash, `"`, "") + `"`
}

func headerContainsETag(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

func parseDatabaseTime(value string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

type idCursor struct {
	ID string `json:"id"`
}

func paginateByID[T any](items []T, rawCursor string, limit int, id func(T) string) ([]T, string, error) {
	cursor, err := decodeCursor[idCursor](rawCursor)
	if err != nil {
		return nil, "", err
	}
	start := 0
	if cursor.ID != "" {
		start = -1
		for index, item := range items {
			if id(item) == cursor.ID {
				start = index + 1
				break
			}
		}
		if start < 0 {
			return nil, "", errInvalidCursor
		}
	}
	if start >= len(items) {
		return []T{}, "", nil
	}
	end := min(start+limit, len(items))
	next := ""
	if end < len(items) {
		next = encodeCursor(idCursor{ID: id(items[end-1])})
	}
	return items[start:end], next, nil
}

func encodeCursor(value any) string {
	payload, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCursor[T any](raw string) (T, error) {
	var cursor T
	if raw == "" {
		return cursor, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor, err
	}
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return cursor, err
	}
	return cursor, nil
}

var errInvalidCursor = &cursorError{}

type cursorError struct{}

func (*cursorError) Error() string { return "cursor does not exist in the current result set" }
