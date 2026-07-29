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
	"seoblog/apps/backend/internal/store"
)

type Options struct {
	Config config.Config
	Logger *slog.Logger
	Store  *store.Store
}

type Server struct {
	app     *fiber.App
	openAPI *huma.OpenAPI
	cfg     config.Config
	logger  *slog.Logger
	store   *store.Store
}

func New(opts Options) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "seoblog-api",
		BodyLimit:             2 * 1024 * 1024,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           60 * time.Second,
		DisableStartupMessage: true,
	})
	app.Use(requestid.New())
	app.Use(recover.New(recover.Config{EnableStackTrace: false}))

	s := &Server{
		app:    app,
		cfg:    opts.Config,
		logger: opts.Logger,
		store:  opts.Store,
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
