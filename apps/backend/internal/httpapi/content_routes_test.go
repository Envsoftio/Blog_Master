package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"seoblog/apps/backend/internal/store"
)

func TestPublishedValidatorsReturnNotModified(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		post := store.PublishedPost{
			ContentHash: "sha256-value",
			ModifiedAt:  "2026-07-29 10:30:00",
		}
		if setPublishedValidators(c, post) {
			return c.SendStatus(fiber.StatusNotModified)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(http.CanonicalHeaderKey("If-None-Match"), `"sha256-value"`)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", response.StatusCode)
	}
	if response.Header.Get("Last-Modified") != "Wed, 29 Jul 2026 10:30:00 GMT" {
		t.Fatalf("unexpected Last-Modified: %q", response.Header.Get("Last-Modified"))
	}
}

func TestCursorRoundTrip(t *testing.T) {
	expected := store.ChangeCursor{CreatedAt: "2026-07-29 10:30:00", ID: "event-id"}
	raw := encodeCursor(expected)
	actual, err := decodeCursor[store.ChangeCursor](raw)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestScopeMiddlewareRejectsMissingScope(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		c.Locals(projectContextKey, ProjectContext{ProjectID: "project", Scopes: []string{"redirects:read"}})
		return c.Next()
	}, requireContentScope("content:published:read"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.StatusCode)
	}
}

func TestContentRateLimiterSeparatesAPIKeys(t *testing.T) {
	app := fiber.New()
	app.Get(
		"/",
		func(c *fiber.Ctx) error {
			c.Locals(projectContextKey, ProjectContext{
				ProjectID: "project",
				KeyID:     c.Get("X-Test-Key"),
			})
			return c.Next()
		},
		newContentRateLimiter(1, func(c *fiber.Ctx) string {
			project, _ := contentProject(c)
			return project.KeyID
		}),
		func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		},
	)

	assertRateLimitStatus(t, app, "key-a", fiber.StatusOK)
	assertRateLimitStatus(t, app, "key-a", fiber.StatusTooManyRequests)
	assertRateLimitStatus(t, app, "key-b", fiber.StatusOK)
}

func TestPublishedPostCacheIsGenerationScopedAndPreservesValidators(t *testing.T) {
	server, db := newAdminTestServer(t)
	cache := newRecordingResponseCache()
	server.cache = cache

	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"cache","name":"Cache Project","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Cached Guide",
		"slug":"cached-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Cached body</p>"
	}`)
	approveTestRevision(t, server, login, project.ID, article.LatestRevision.ID)
	publishTestArticle(t, server, login, project.ID, article.ID, article.LatestRevision.ID, "cached-guide")

	firstResponse := getContentPost(t, server, project.ID, "cached-guide", "")
	if firstResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected first content read 200, got %d: %s", firstResponse.StatusCode, readBody(t, firstResponse))
	}
	firstETag := firstResponse.Header.Get("ETag")
	if firstETag == "" {
		t.Fatal("expected published response to include ETag")
	}
	var first Envelope[store.PublishedPost]
	decodeJSONResponse(t, firstResponse, &first)
	if first.Meta.ContentGeneration == 0 {
		t.Fatalf("expected content generation in response meta, got %#v", first.Meta)
	}
	if cache.setCount() == 0 {
		t.Fatal("expected first content read to populate cache")
	}

	notModified := getContentPost(t, server, project.ID, "cached-guide", firstETag)
	if notModified.StatusCode != http.StatusNotModified {
		t.Fatalf("expected cached conditional read 304, got %d: %s", notModified.StatusCode, readBody(t, notModified))
	}
	if cache.getCount() == 0 {
		t.Fatal("expected second content read to check cache")
	}

	revision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Fresh Guide",
		"html":"<p>Fresh body</p>"
	}`)
	approveTestRevision(t, server, login, project.ID, revision.ID)
	publishTestArticle(t, server, login, project.ID, article.ID, revision.ID, "cached-guide")

	updatedResponse := getContentPost(t, server, project.ID, "cached-guide", firstETag)
	if updatedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected updated generation to bypass stale 304, got %d: %s", updatedResponse.StatusCode, readBody(t, updatedResponse))
	}
	var updated Envelope[store.PublishedPost]
	decodeJSONResponse(t, updatedResponse, &updated)
	if updated.Data.Title != "Fresh Guide" {
		t.Fatalf("expected generation-scoped cache to return fresh article, got %q", updated.Data.Title)
	}
	if updated.Meta.ContentGeneration <= first.Meta.ContentGeneration {
		t.Fatalf("expected content generation to advance from %d, got %d", first.Meta.ContentGeneration, updated.Meta.ContentGeneration)
	}
}

func TestPublishedPostCacheFailureFallsBackToSQLite(t *testing.T) {
	server, db := newAdminTestServer(t)
	server.cache = failingResponseCache{}

	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"cache-fallback","name":"Cache Fallback Project"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"docs","name":"Docs"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"standard",
		"title":"Fallback Post",
		"slug":"fallback-post",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Fallback body</p>"
	}`)
	approveTestRevision(t, server, login, project.ID, article.LatestRevision.ID)
	publishTestArticle(t, server, login, project.ID, article.ID, article.LatestRevision.ID, "fallback-post")

	response := getContentPost(t, server, project.ID, "fallback-post", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected SQLite fallback response 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.PublishedPost]
	decodeJSONResponse(t, response, &payload)
	if payload.Data.Title != "Fallback Post" {
		t.Fatalf("unexpected fallback payload %#v", payload.Data)
	}
}

func assertRateLimitStatus(t *testing.T, app *fiber.App, key string, expected int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Test-Key", key)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		t.Fatalf("expected status %d for %s, got %d", expected, key, response.StatusCode)
	}
}

func getContentPost(t *testing.T, server *Server, projectID, slug, ifNoneMatch string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/content/v1/posts/"+slug+"?locale=en", nil)
	request.Header.Set("X-Dev-Project-ID", projectID)
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	return mustTest(t, server, request)
}

type recordingResponseCache struct {
	mu     sync.Mutex
	values map[string][]byte
	gets   int
	sets   int
	ttls   []time.Duration
}

func newRecordingResponseCache() *recordingResponseCache {
	return &recordingResponseCache{values: map[string][]byte{}}
}

func (c *recordingResponseCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	value, ok := c.values[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (c *recordingResponseCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	c.values[key] = append([]byte(nil), value...)
	c.ttls = append(c.ttls, ttl)
	return nil
}

func (c *recordingResponseCache) getCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gets
}

func (c *recordingResponseCache) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets
}

type failingResponseCache struct{}

func (failingResponseCache) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, errors.New("redis unavailable")
}

func (failingResponseCache) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis unavailable")
}
