package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

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

func (s *Server) requirePreviewToken(c fiber.Ctx) error {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return problem(c, fiber.StatusUnauthorized, "Missing preview token", "Use Authorization: Bearer <preview-token>")
	}
	secret := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if secret == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing preview token", "The bearer token is empty")
	}
	token, err := s.store.FindPreviewToken(c.Context(), secret, c.Params("revisionID"))
	if err != nil {
		s.logger.Warn("preview token rejected", "error", err)
		return problem(c, fiber.StatusUnauthorized, "Invalid preview token", "The preview token is invalid, expired or revoked")
	}
	c.Locals(previewContextKey, token)
	return c.Next()
}

func previewTokenRateLimiter() fiber.Handler {
	return newContentRateLimiter(300, func(c fiber.Ctx) string {
		preview, _ := previewContext(c)
		return "preview-token:" + preview.ID
	})
}

func previewContext(c fiber.Ctx) (store.PreviewTokenContext, bool) {
	value := c.Locals(previewContextKey)
	if value == nil {
		return store.PreviewTokenContext{}, false
	}
	ctx, ok := value.(store.PreviewTokenContext)
	return ctx, ok
}

func (s *Server) getPreviewRevision(c fiber.Ctx) error {
	preview, ok := previewContext(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing preview context", "")
	}
	post, err := s.store.GetPreviewPost(c.Context(), preview.ProjectID, preview.ArticleID, preview.RevisionID)
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

func (s *Server) listPublishedPosts(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	category := c.Query("category")
	tag := c.Query("tag")
	author := c.Query("author")
	articleType := c.Query("articleType")
	series := c.Query("series")
	exactCategory := c.Query("categoryMode") == "exact"
	publishedFrom := c.Query("publishedFrom")
	publishedTo := c.Query("publishedTo")
	limit := boundedLimit(c.Query("limit", "20"), 50)
	cursor, err := decodeCursor[store.PublishedCursor](c.Query("cursor"))
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "The cursor is malformed")
	}
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "list", c.Path(), normalizedContentQuery(c), cacheLimit(limit))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (ListEnvelope[store.PublishedPost], time.Duration, error) {
		posts, err := s.store.ListPublishedPosts(
			ctx,
			projectID,
			category,
			tag,
			author,
			articleType,
			series,
			exactCategory,
			publishedFrom,
			publishedTo,
			cursor,
			limit+1,
		)
		if err != nil {
			return ListEnvelope[store.PublishedPost]{}, 0, err
		}
		nextCursor := ""
		if len(posts) > limit {
			posts = posts[:limit]
			last := posts[len(posts)-1]
			nextCursor = encodeCursor(store.PublishedCursor{SortAt: last.PaginationKey, ID: last.ID})
		}
		return ListEnvelope[store.PublishedPost]{
			Data: posts,
			Meta: PageMeta{ProjectID: projectID, ContentGeneration: generation, Limit: limit, NextCursor: nextCursor},
		}, publishedListCacheTTL, nil
	})
	if err != nil {
		s.logger.Error("list published posts", "project_id", projectID, "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not list posts", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) getPublishedPostBySlug(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	slug := c.Params("slug")
	generation := s.projectGeneration(ctx, projectID)
	cached, err := s.cachedPublishedPostBySlug(ctx, projectID, generation, slug)
	if err != nil {
		return s.publishedReadError(c, err, "Post not found", "Could not load post")
	}
	if !cached.Found {
		return problem(c, fiber.StatusNotFound, "Post not found", "")
	}
	cached.Envelope.Data.ContentHash = unquotedETag(cached.Envelope.Meta.ETag)
	if setPublishedValidators(c, cached.Envelope.Data) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return writeJSON(c, fiber.StatusOK, cached.Envelope)
}

func (s *Server) headPublishedPostBySlug(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	generation := s.projectGeneration(ctx, projectID)
	cached, err := s.cachedPublishedPostBySlug(ctx, projectID, generation, c.Params("slug"))
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if !cached.Found {
		return c.SendStatus(fiber.StatusNotFound)
	}
	cached.Envelope.Data.ContentHash = unquotedETag(cached.Envelope.Meta.ETag)
	if setPublishedValidators(c, cached.Envelope.Data) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return c.SendStatus(fiber.StatusOK)
}

func (s *Server) getPublishedPostByID(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	contentID := c.Params("contentID")
	generation := s.projectGeneration(ctx, projectID)
	cached, err := s.cachedPublishedPostByID(ctx, projectID, generation, contentID)
	if err != nil {
		return s.publishedReadError(c, err, "Post not found", "Could not load post")
	}
	if !cached.Found {
		return problem(c, fiber.StatusNotFound, "Post not found", "")
	}
	cached.Envelope.Data.ContentHash = unquotedETag(cached.Envelope.Meta.ETag)
	if setPublishedValidators(c, cached.Envelope.Data) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return writeJSON(c, fiber.StatusOK, cached.Envelope)
}

func (s *Server) cachedPublishedPostBySlug(ctx context.Context, projectID string, generation int64, slug string) (cachedPublishedPost, error) {
	cacheKey, _ := s.contentCacheKey(projectID, generation, "post", "slug", slug)
	return cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (cachedPublishedPost, time.Duration, error) {
		post, err := s.store.GetPublishedPostBySlug(ctx, projectID, slug)
		if err != nil {
			if err == sql.ErrNoRows {
				return cachedPublishedPost{Found: false}, publishedLookupMissTTL, nil
			}
			return cachedPublishedPost{}, 0, err
		}
		response := Envelope[store.PublishedPost]{
			Data: post,
			Meta: MetaData{ProjectID: projectID, ContentGeneration: generation, ETag: quotedETag(post.ContentHash)},
		}
		return cachedPublishedPost{Found: true, Envelope: response}, publishedPostCacheTTL, nil
	})
}

func (s *Server) cachedPublishedPostByID(ctx context.Context, projectID string, generation int64, contentID string) (cachedPublishedPost, error) {
	cacheKey, _ := s.contentCacheKey(projectID, generation, "post", "id", contentID)
	return cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (cachedPublishedPost, time.Duration, error) {
		post, err := s.store.GetPublishedPostByID(ctx, projectID, contentID)
		if err != nil {
			if err == sql.ErrNoRows {
				return cachedPublishedPost{Found: false}, publishedLookupMissTTL, nil
			}
			return cachedPublishedPost{}, 0, err
		}
		response := Envelope[store.PublishedPost]{
			Data: post,
			Meta: MetaData{ProjectID: projectID, ContentGeneration: generation, ETag: quotedETag(post.ContentHash)},
		}
		return cachedPublishedPost{Found: true, Envelope: response}, publishedPostCacheTTL, nil
	})
}

func (s *Server) publishedReadError(c fiber.Ctx, err error, notFoundTitle, internalTitle string) error {
	if err == sql.ErrNoRows {
		return problem(c, fiber.StatusNotFound, notFoundTitle, "")
	}
	s.logger.Error("published content read", "error", err)
	return problem(c, fiber.StatusInternalServerError, internalTitle, "")
}

func (s *Server) getRelatedPosts(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	slug := c.Params("slug")
	limit := boundedLimit(c.Query("limit", "6"), 12)
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "related", slug, cacheLimit(limit))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (ListEnvelope[store.RelatedPost], time.Duration, error) {
		posts, err := s.store.ListRelatedPosts(ctx, projectID, slug, limit)
		if err != nil {
			return ListEnvelope[store.RelatedPost]{}, 0, err
		}
		return ListEnvelope[store.RelatedPost]{
			Data: posts,
			Meta: PageMeta{ProjectID: projectID, ContentGeneration: generation, Limit: limit},
		}, publishedListCacheTTL, nil
	})
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not load related posts", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) listCategories(c fiber.Ctx) error { return s.listTerms(c, "category") }
func (s *Server) getCategory(c fiber.Ctx) error    { return s.getTerm(c, "category") }
func (s *Server) listTags(c fiber.Ctx) error       { return s.listTerms(c, "tag") }
func (s *Server) getTag(c fiber.Ctx) error         { return s.getTerm(c, "tag") }

func (s *Server) listTerms(c fiber.Ctx, termType string) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	cursor := c.Query("cursor")
	limit := boundedLimit(c.Query("limit", "50"), 100)
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "taxonomy", termType, normalizedContentQuery(c))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (ListEnvelope[store.TaxonomyTerm], time.Duration, error) {
		terms, err := s.store.ListTerms(ctx, projectID, termType)
		if err != nil {
			return ListEnvelope[store.TaxonomyTerm]{}, 0, err
		}
		page, next, pageErr := paginateByID(terms, cursor, limit, func(term store.TaxonomyTerm) string { return term.ID })
		if pageErr != nil {
			return ListEnvelope[store.TaxonomyTerm]{}, 0, pageErr
		}
		return ListEnvelope[store.TaxonomyTerm]{
			Data: page,
			Meta: PageMeta{ProjectID: projectID, ContentGeneration: generation, Limit: len(page), NextCursor: next},
		}, publishedTaxonomyCacheTTL, nil
	})
	if err != nil {
		if errors.Is(err, errInvalidCursor) {
			return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not list taxonomy", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) getTerm(c fiber.Ctx, termType string) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	slug := c.Params("slug")
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "taxonomy-detail", termType, slug)
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (Envelope[store.TaxonomyTerm], time.Duration, error) {
		term, err := s.store.GetTerm(ctx, projectID, termType, slug)
		if err != nil {
			return Envelope[store.TaxonomyTerm]{}, 0, err
		}
		return Envelope[store.TaxonomyTerm]{
			Data: term,
			Meta: MetaData{ProjectID: projectID, ContentGeneration: generation},
		}, publishedTaxonomyCacheTTL, nil
	})
	if err != nil {
		if err == sql.ErrNoRows {
			prefix := taxonomyRoutePrefix(termType)
			if prefix != "" {
				redirect, redirectErr := s.store.GetRedirect(ctx, projectID, prefix+slug)
				if redirectErr == nil && strings.HasPrefix(redirect.TargetPath, prefix) {
					return c.Redirect().Status(fiber.StatusMovedPermanently).To("/content/v1" + redirect.TargetPath)
				}
				if redirectErr != nil && redirectErr != sql.ErrNoRows {
					return problem(c, fiber.StatusInternalServerError, "Could not load taxonomy redirect", "")
				}
			}
			return problem(c, fiber.StatusNotFound, "Taxonomy term not found", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not load taxonomy term", "")
	}
	return writePublishedJSON(c, response)
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

func (s *Server) listAuthors(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	cursor := c.Query("cursor")
	limit := boundedLimit(c.Query("limit", "50"), 100)
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "authors", normalizedContentQuery(c))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (ListEnvelope[store.Author], time.Duration, error) {
		authors, err := s.store.ListAuthors(ctx, projectID)
		if err != nil {
			return ListEnvelope[store.Author]{}, 0, err
		}
		page, next, pageErr := paginateByID(authors, cursor, limit, func(author store.Author) string { return author.ID })
		if pageErr != nil {
			return ListEnvelope[store.Author]{}, 0, pageErr
		}
		return ListEnvelope[store.Author]{
			Data: page,
			Meta: PageMeta{ProjectID: projectID, ContentGeneration: generation, Limit: len(page), NextCursor: next},
		}, publishedTaxonomyCacheTTL, nil
	})
	if err != nil {
		if errors.Is(err, errInvalidCursor) {
			return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not list authors", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) getAuthor(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	slug := c.Params("slug")
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "author-detail", slug)
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (Envelope[store.Author], time.Duration, error) {
		author, err := s.store.GetAuthor(ctx, projectID, slug)
		if err != nil {
			return Envelope[store.Author]{}, 0, err
		}
		return Envelope[store.Author]{
			Data: author,
			Meta: MetaData{ProjectID: projectID, ContentGeneration: generation},
		}, publishedTaxonomyCacheTTL, nil
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return problem(c, fiber.StatusNotFound, "Author not found", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not load author", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) listSeries(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	cursor := c.Query("cursor")
	limit := boundedLimit(c.Query("limit", "50"), 100)
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "series", normalizedContentQuery(c))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (ListEnvelope[store.Series], time.Duration, error) {
		items, err := s.store.ListSeries(ctx, projectID)
		if err != nil {
			return ListEnvelope[store.Series]{}, 0, err
		}
		page, next, pageErr := paginateByID(items, cursor, limit, func(item store.Series) string { return item.ID })
		if pageErr != nil {
			return ListEnvelope[store.Series]{}, 0, pageErr
		}
		return ListEnvelope[store.Series]{
			Data: page,
			Meta: PageMeta{ProjectID: projectID, ContentGeneration: generation, Limit: len(page), NextCursor: next},
		}, publishedTaxonomyCacheTTL, nil
	})
	if err != nil {
		if errors.Is(err, errInvalidCursor) {
			return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not list series", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) getSeries(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	slug := c.Params("slug")
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "series-detail", slug)
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (Envelope[store.Series], time.Duration, error) {
		item, err := s.store.GetSeries(ctx, projectID, slug)
		if err != nil {
			return Envelope[store.Series]{}, 0, err
		}
		return Envelope[store.Series]{
			Data: item,
			Meta: MetaData{ProjectID: projectID, ContentGeneration: generation},
		}, publishedTaxonomyCacheTTL, nil
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return problem(c, fiber.StatusNotFound, "Series not found", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not load series", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) feedData(c fiber.Ctx) error {
	c.Request().URI().QueryArgs().Set("limit", c.Query("limit", "50"))
	return s.listPublishedPosts(c)
}

func (s *Server) discoveryManifest(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "discovery", normalizedContentQuery(c))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (Envelope[map[string]any], time.Duration, error) {
		entries, err := s.store.ListDiscovery(ctx, projectID)
		if err != nil {
			return Envelope[map[string]any]{}, 0, err
		}
		return Envelope[map[string]any]{
			Data: map[string]any{"urls": entries},
			Meta: MetaData{ProjectID: projectID, ContentGeneration: generation},
		}, publishedListCacheTTL, nil
	})
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not build discovery manifest", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) redirects(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	ctx := c.Context()
	projectID := project.ProjectID
	cursor := c.Query("cursor")
	limit := boundedLimit(c.Query("limit", "100"), 250)
	generation := s.projectGeneration(ctx, projectID)
	cacheKey, _ := s.contentCacheKey(projectID, generation, "redirects", normalizedContentQuery(c))
	response, err := cachedContentJSON(ctx, s, cacheKey, func(ctx context.Context) (ListEnvelope[store.RedirectRecord], time.Duration, error) {
		items, err := s.store.ListRedirects(ctx, projectID)
		if err != nil {
			return ListEnvelope[store.RedirectRecord]{}, 0, err
		}
		page, next, pageErr := paginateByID(items, cursor, limit, func(item store.RedirectRecord) string { return item.SourcePath })
		if pageErr != nil {
			return ListEnvelope[store.RedirectRecord]{}, 0, pageErr
		}
		return ListEnvelope[store.RedirectRecord]{
			Data: page,
			Meta: PageMeta{ProjectID: projectID, ContentGeneration: generation, Limit: len(page), NextCursor: next},
		}, publishedListCacheTTL, nil
	})
	if err != nil {
		if errors.Is(err, errInvalidCursor) {
			return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
		}
		return problem(c, fiber.StatusInternalServerError, "Could not list redirects", "")
	}
	return writePublishedJSON(c, response)
}

func (s *Server) changes(c fiber.Ctx) error {
	project, ok := contentProject(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
	}
	cursor, err := decodeCursor[store.ChangeCursor](c.Query("after"))
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	limit := boundedLimit(c.Query("limit", "100"), 250)
	items, err := s.store.ListChanges(c.Context(), project.ProjectID, cursor, limit+1)
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

func setPublishedValidators(c fiber.Ctx, post store.PublishedPost) bool {
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

func writePublishedJSON(c fiber.Ctx, value any) error {
	c.Set(fiber.HeaderCacheControl, "private, max-age=60, stale-while-revalidate=300, stale-if-error=86400")
	return writeJSON(c, fiber.StatusOK, value)
}

func quotedETag(hash string) string {
	if hash == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(hash, `"`, "") + `"`
}

func unquotedETag(etag string) string {
	return strings.Trim(strings.TrimSpace(etag), `"`)
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
