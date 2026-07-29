package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

const projectContextKey = "projectContext"

func (s *Server) requireContentKey(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		if s.cfg.DevAuth && s.cfg.Env == "development" {
			devProjectID := c.Get("X-Dev-Project-ID")
			if devProjectID != "" {
				c.Locals(projectContextKey, ProjectContext{ProjectID: devProjectID, KeyID: "dev", Scopes: []string{"content:published:read", "taxonomy:published:read", "authors:published:read", "discovery:read", "redirects:read"}})
				return c.Next()
			}
		}
		return problem(c, fiber.StatusUnauthorized, "Missing API key", "Use Authorization: Bearer <project-api-key>")
	}

	secret := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if secret == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing API key", "The bearer token is empty")
	}

	sum := sha256.Sum256([]byte(secret))
	key, err := s.store.FindAPIKeyByHash(c.UserContext(), hex.EncodeToString(sum[:]))
	if err != nil {
		s.logger.Warn("content api key rejected", "error", err)
		return problem(c, fiber.StatusUnauthorized, "Invalid API key", "The project API key is invalid, expired or revoked")
	}

	c.Locals(projectContextKey, ProjectContext{ProjectID: key.ProjectID, KeyID: key.ID, Scopes: key.Scopes})
	return c.Next()
}

func requireContentScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		project, ok := contentProject(c)
		if !ok {
			return problem(c, fiber.StatusUnauthorized, "Missing project context", "")
		}
		for _, granted := range project.Scopes {
			if granted == scope {
				return c.Next()
			}
		}
		return problem(c, fiber.StatusForbidden, "Insufficient API-key scope", "This API key does not grant "+scope)
	}
}

func contentProject(c *fiber.Ctx) (ProjectContext, bool) {
	value := c.Locals(projectContextKey)
	if value == nil {
		return ProjectContext{}, false
	}
	ctx, ok := value.(ProjectContext)
	return ctx, ok
}

func contentSourceRateLimiter() fiber.Handler {
	return newContentRateLimiter(300, func(c *fiber.Ctx) string {
		source := strings.TrimSpace(strings.Split(c.Get("X-Forwarded-For"), ",")[0])
		if source == "" {
			source = c.IP()
		}
		return "source:" + source
	})
}

func contentKeyRateLimiter() fiber.Handler {
	return newContentRateLimiter(1200, func(c *fiber.Ctx) string {
		project, _ := contentProject(c)
		return "key:" + project.KeyID
	})
}

func contentProjectRateLimiter() fiber.Handler {
	return newContentRateLimiter(6000, func(c *fiber.Ctx) string {
		project, _ := contentProject(c)
		return "project:" + project.ProjectID
	})
}

func newContentRateLimiter(max int, keyGenerator func(*fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   time.Minute,
		KeyGenerator: keyGenerator,
		LimitReached: func(c *fiber.Ctx) error {
			return problem(c, fiber.StatusTooManyRequests, "Content API rate limit exceeded", "Retry after the current rate-limit window")
		},
	})
}
