package main

import (
	"log/slog"
	"os"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/httpapi"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := database.OpenSQLite(cfg.DBPath)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}

	srv := httpapi.New(httpapi.Options{
		Config: cfg,
		Logger: logger,
		Store:  store.New(db),
	})

	logger.Info("starting api", "addr", cfg.HTTPAddr)
	if err := srv.Listen(cfg.HTTPAddr); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
