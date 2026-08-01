package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/mailer"
	"seoblog/apps/backend/internal/observability"
	"seoblog/apps/backend/internal/platform/b2"
	"seoblog/apps/backend/internal/store"
)

type Options struct {
	Config       config.Config
	Logger       *slog.Logger
	Mailer       mailer.Sender
	Store        *store.Store
	Cache        ResponseCache
	MediaStorage mediaStorage
	Metrics      *observability.Registry
}

type Server struct {
	app          *fiber.App
	openAPI      *huma.OpenAPI
	cfg          config.Config
	logger       *slog.Logger
	mailer       mailer.Sender
	mailSlots    chan struct{}
	store        *store.Store
	cache        ResponseCache
	cacheFill    cacheFlightGroup
	mediaStorage mediaStorage
	metrics      *observability.Registry
}

type mediaStorage interface {
	Bucket() string
	PublicURL(key string) string
	PresignPut(key, contentType string, maxBytes int64, now time.Time) (b2.SignedUpload, error)
	GetObject(ctx context.Context, key string, maxBytes int64) ([]byte, string, error)
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	DeleteObject(ctx context.Context, key string) error
}

func New(opts Options) *Server {
	fiberConfig := fiber.Config{
		AppName:      "seoblog-api",
		BodyLimit:    2 * 1024 * 1024,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if len(opts.Config.TrustedProxies) > 0 {
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedFor
		fiberConfig.EnableIPValidation = true
		fiberConfig.TrustProxy = true
		fiberConfig.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: opts.Config.TrustedProxies}
	}
	app := fiber.New(fiberConfig)
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	app.Use(requestid.New())
	app.Use(func(c fiber.Ctx) error {
		requestID := requestid.FromContext(c)
		c.SetContext(store.WithRequestID(c.Context(), requestID))
		return c.Next()
	})
	app.Use(func(c fiber.Ctx) error {
		startedAt := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil && status < fiber.StatusBadRequest {
			status = fiber.StatusInternalServerError
			var fiberError *fiber.Error
			if errors.As(err, &fiberError) {
				status = fiberError.Code
			}
		}
		route := c.Route().Path
		duration := time.Since(startedAt)
		if opts.Metrics != nil {
			opts.Metrics.RecordHTTPRequest(c.Method(), route, status, duration)
		}
		level := slog.LevelInfo
		category := "none"
		outcome := "success"
		if status >= fiber.StatusInternalServerError {
			level = slog.LevelError
			category = "server_error"
			outcome = "error"
		} else if status >= fiber.StatusBadRequest {
			level = slog.LevelWarn
			category = "client_error"
			outcome = "rejected"
		}
		attributes := []any{
			"request_id", requestid.FromContext(c),
			"method", c.Method(),
			"route", route,
			"status", status,
			"outcome", outcome,
			"duration_ms", duration.Milliseconds(),
			"error_category", category,
		}
		if project, ok := contentProject(c); ok {
			attributes = append(attributes, "project_id", project.ProjectID, "api_key_id", project.KeyID)
		} else if projectID := c.Params("projectID"); projectID != "" {
			attributes = append(attributes, "project_id", projectID)
		}
		if user, ok := adminUser(c); ok {
			attributes = append(attributes, "actor_user_id", user.ID)
		}
		opts.Logger.Log(c.Context(), level, "http request completed", attributes...)
		return err
	})
	app.Use(recover.New(recover.Config{EnableStackTrace: false}))

	messageSender := opts.Mailer
	if messageSender == nil {
		messageSender = mailer.NewSMTP(mailer.SMTPConfig{
			Address:         opts.Config.SMTPAddress,
			Username:        opts.Config.SMTPUsername,
			Password:        opts.Config.SMTPPassword,
			From:            opts.Config.SMTPFrom,
			FromName:        opts.Config.SMTPFromName,
			RequireStartTLS: opts.Config.SMTPRequireTLS,
		})
	}
	mediaStorage := opts.MediaStorage
	if mediaStorage == nil && opts.Config.B2MediaEnabled() {
		client, err := b2.New(b2.Config{
			Endpoint:             opts.Config.B2MediaEndpoint,
			Region:               opts.Config.B2MediaRegion,
			Bucket:               opts.Config.B2MediaBucket,
			KeyID:                opts.Config.B2MediaKeyID,
			ApplicationKey:       opts.Config.B2MediaApplicationKey,
			PublicBaseURL:        opts.Config.B2MediaPublicBaseURL,
			PresignTTL:           opts.Config.B2MediaPresignTTL,
			ServerSideEncryption: opts.Config.B2MediaSSE,
		})
		if err != nil {
			opts.Logger.Error("B2 media storage disabled", "error", err)
		} else {
			mediaStorage = client
		}
	}
	s := &Server{
		app:          app,
		cfg:          opts.Config,
		logger:       opts.Logger,
		mailer:       messageSender,
		mailSlots:    make(chan struct{}, 8),
		store:        opts.Store,
		cache:        opts.Cache,
		mediaStorage: mediaStorage,
		metrics:      opts.Metrics,
	}
	s.registerRoutes()
	return s
}

func (s *Server) Listen(addr string) error {
	return s.app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
}

type healthOutput struct {
	Body HealthResponse
}

func (s *Server) registerRoutes() {
	openapi := huma.DefaultConfig("Article Content Hub API", "0.1.0")
	api := humafiber.New(s.app, openapi)
	s.openAPI = api.OpenAPI()

	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Service health check",
	}, func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		return &healthOutput{Body: HealthResponse{
			Status:  "ok",
			Service: "seoblog-api",
			Version: "0.1.0",
		}}, nil
	})

	s.app.Get("/readyz", func(c fiber.Ctx) error {
		if err := s.store.Ping(c.Context()); err != nil {
			return problem(c, fiber.StatusServiceUnavailable, "Database unavailable", "SQLite did not respond")
		}
		return writeJSON(c, fiber.StatusOK, HealthResponse{Status: "ok", Service: "seoblog-api", Version: "0.1.0"})
	})

	s.app.Get("/metrics", func(c fiber.Ctx) error {
		if s.metrics == nil {
			return c.SendStatus(fiber.StatusNotFound)
		}
		var output bytes.Buffer
		if err := s.metrics.Render(c.Context(), &output); err != nil {
			s.logger.Error("collect metrics", "error_category", "metrics_collection", "error", err)
			return problem(c, fiber.StatusServiceUnavailable, "Metrics unavailable", "Operational metrics could not be collected")
		}
		c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
		return c.Send(output.Bytes())
	})

	s.registerAdminRoutes()
	s.registerContentRoutes()
	documentFiberRoutes(api, s.app)
}

func (s *Server) OpenAPIYAML() ([]byte, error) {
	return s.openAPI.YAML()
}
