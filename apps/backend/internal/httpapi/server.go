package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/mailer"
	"seoblog/apps/backend/internal/store"
)

type Options struct {
	Config config.Config
	Logger *slog.Logger
	Mailer mailer.Sender
	Store  *store.Store
}

type Server struct {
	app       *fiber.App
	openAPI   *huma.OpenAPI
	cfg       config.Config
	logger    *slog.Logger
	mailer    mailer.Sender
	mailSlots chan struct{}
	store     *store.Store
}

func New(opts Options) *Server {
	fiberConfig := fiber.Config{
		AppName:               "seoblog-api",
		BodyLimit:             2 * 1024 * 1024,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           60 * time.Second,
		DisableStartupMessage: true,
	}
	if len(opts.Config.TrustedProxies) > 0 {
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedFor
		fiberConfig.EnableIPValidation = true
		fiberConfig.EnableTrustedProxyCheck = true
		fiberConfig.TrustedProxies = opts.Config.TrustedProxies
	}
	app := fiber.New(fiberConfig)
	app.Use(requestid.New())
	app.Use(func(c *fiber.Ctx) error {
		requestID, _ := c.Locals("requestid").(string)
		c.SetUserContext(store.WithRequestID(c.UserContext(), requestID))
		return c.Next()
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
	s := &Server{
		app:       app,
		cfg:       opts.Config,
		logger:    opts.Logger,
		mailer:    messageSender,
		mailSlots: make(chan struct{}, 8),
		store:     opts.Store,
	}
	s.registerRoutes()
	return s
}

func (s *Server) Listen(addr string) error {
	return s.app.Listen(addr)
}

type healthOutput struct {
	Body HealthResponse
}

func (s *Server) registerRoutes() {
	openapi := huma.DefaultConfig("SEO Blog CMS API", "0.1.0")
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

	s.app.Get("/readyz", func(c *fiber.Ctx) error {
		if err := s.store.Ping(c.UserContext()); err != nil {
			return problem(c, fiber.StatusServiceUnavailable, "Database unavailable", "SQLite did not respond")
		}
		return writeJSON(c, fiber.StatusOK, HealthResponse{Status: "ok", Service: "seoblog-api", Version: "0.1.0"})
	})

	s.registerAdminRoutes()
	s.registerContentRoutes()
	documentFiberRoutes(api, s.app)
}

func (s *Server) OpenAPIYAML() ([]byte, error) {
	return s.openAPI.YAML()
}
