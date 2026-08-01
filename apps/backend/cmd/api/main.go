package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/httpapi"
	"seoblog/apps/backend/internal/observability"
	"seoblog/apps/backend/internal/platform/database"
	redisclient "seoblog/apps/backend/internal/platform/redis"
	"seoblog/apps/backend/internal/store"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "seoblog-api", "environment", cfg.Env)

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

	var responseCache httpapi.ResponseCache
	if cfg.RedisAddr != "" {
		redisCache := redisclient.New(redisclient.Config{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			Timeout:  cfg.RedisTimeout,
		})
		responseCache = redisCache
		pingContext, cancel := context.WithTimeout(context.Background(), time.Second)
		if err := redisCache.Ping(pingContext); err != nil {
			logger.Warn("redis unavailable; content API will fall back to SQLite", "addr", cfg.RedisAddr, "error", err)
		} else {
			logger.Info("redis cache enabled", "addr", cfg.RedisAddr)
		}
		cancel()
	}

	metrics := observability.NewRegistry(db, cfg.DBPath, "seoblog-api", cfg.Env)
	srv := httpapi.New(httpapi.Options{
		Config:  cfg,
		Logger:  logger,
		Store:   store.New(db, store.WithWebhookEncryptionKey(cfg.WebhookEncryptionKey)),
		Cache:   responseCache,
		Metrics: metrics,
	})

	logger.Info("starting api", "addr", cfg.HTTPAddr)
	if err := srv.Listen(cfg.HTTPAddr); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
