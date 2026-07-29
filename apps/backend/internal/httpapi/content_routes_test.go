package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
