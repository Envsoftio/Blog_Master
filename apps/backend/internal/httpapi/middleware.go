package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

const (
	projectContextKey = "projectContext"
	previewContextKey = "previewContext"
)

func (s *Server) requireContentKey(c fiber.Ctx) error {
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
	key, err := s.store.FindAPIKeyByHash(c.Context(), hex.EncodeToString(sum[:]))
	if err != nil {
		s.logger.Warn("content api key rejected", "error", err)
		return problem(c, fiber.StatusUnauthorized, "Invalid API key", "The project API key is invalid, expired or revoked")
	}

	c.Locals(projectContextKey, ProjectContext{ProjectID: key.ProjectID, KeyID: key.ID, Scopes: key.Scopes})
	return c.Next()
}

func requireContentScope(scope string) fiber.Handler {
	return func(c fiber.Ctx) error {
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

func contentProject(c fiber.Ctx) (ProjectContext, bool) {
	value := c.Locals(projectContextKey)
	if value == nil {
		return ProjectContext{}, false
	}
	ctx, ok := value.(ProjectContext)
	return ctx, ok
}

func contentSourceRateLimiter() fiber.Handler {
	return newContentRateLimiter(300, func(c fiber.Ctx) string {
		return "source:" + requestSource(c)
	})
}

func contentKeyRateLimiter() fiber.Handler {
	return newContentRateLimiter(1200, func(c fiber.Ctx) string {
		project, _ := contentProject(c)
		return "key:" + project.KeyID
	})
}

func contentProjectRateLimiter() fiber.Handler {
	return newContentRateLimiter(6000, func(c fiber.Ctx) string {
		project, _ := contentProject(c)
		return "project:" + project.ProjectID
	})
}

func newContentRateLimiter(max int, keyGenerator func(fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   time.Minute,
		KeyGenerator: keyGenerator,
		LimitReached: func(c fiber.Ctx) error {
			return problem(c, fiber.StatusTooManyRequests, "Content API rate limit exceeded", "Retry after the current rate-limit window")
		},
	})
}

func invitationCreationSourceRateLimiter() fiber.Handler {
	return newAdminRateLimiter(30, time.Minute, func(c fiber.Ctx) string {
		return "invitation-create-source:" + requestSource(c)
	})
}

func invitationRecipientRateLimiter() fiber.Handler {
	return newAdminRateLimiter(5, time.Hour, func(c fiber.Ctx) string {
		var input struct {
			Email string `json:"email"`
		}
		_ = json.Unmarshal(c.Body(), &input)
		identity := strings.ToLower(strings.TrimSpace(input.Email))
		if identity == "" {
			identity = "invalid"
		}
		return "invitation-recipient:" + hashRateLimitIdentity(identity)
	})
}

func invitationAcceptanceSourceRateLimiter() fiber.Handler {
	return newAdminRateLimiter(30, 15*time.Minute, func(c fiber.Ctx) string {
		return "invitation-accept-source:" + requestSource(c)
	})
}

func invitationTokenRateLimiter() fiber.Handler {
	return newAdminRateLimiter(10, 15*time.Minute, func(c fiber.Ctx) string {
		return "invitation-token:" + hashRateLimitIdentity(c.Params("token"))
	})
}

func passwordResetRequestSourceRateLimiter() fiber.Handler {
	return newAdminRateLimiter(10, time.Hour, func(c fiber.Ctx) string {
		return "password-reset-request-source:" + requestSource(c)
	})
}

func passwordResetEmailRateLimiter() fiber.Handler {
	return newAdminRateLimiter(3, time.Hour, func(c fiber.Ctx) string {
		var input struct {
			Email string `json:"email"`
		}
		_ = json.Unmarshal(c.Body(), &input)
		identity := strings.ToLower(strings.TrimSpace(input.Email))
		if identity == "" {
			identity = "invalid"
		}
		return "password-reset-email:" + hashRateLimitIdentity(identity)
	})
}

func passwordResetCompletionSourceRateLimiter() fiber.Handler {
	return newAdminRateLimiter(20, time.Hour, func(c fiber.Ctx) string {
		return "password-reset-completion-source:" + requestSource(c)
	})
}

func passwordResetTokenRateLimiter() fiber.Handler {
	return newAdminRateLimiter(5, time.Hour, func(c fiber.Ctx) string {
		var input struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(c.Body(), &input)
		identity := strings.TrimSpace(input.Token)
		if identity == "" {
			identity = "invalid"
		}
		return "password-reset-token:" + hashRateLimitIdentity(identity)
	})
}

func newAdminRateLimiter(max int, expiration time.Duration, keyGenerator func(fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   expiration,
		KeyGenerator: keyGenerator,
		LimitReached: func(c fiber.Ctx) error {
			return problem(c, fiber.StatusTooManyRequests, "Too many requests", "Retry after the current rate-limit window")
		},
	})
}

func requestSource(c fiber.Ctx) string {
	return c.IP()
}

func hashRateLimitIdentity(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
