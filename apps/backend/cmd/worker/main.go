package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"seoblog/apps/backend/internal/config"
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

	workerStore := store.New(db)
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Info("worker ready", "poll_interval", "5s")
	for {
		select {
		case <-stopContext.Done():
			logger.Info("worker stopped")
			return
		case <-ticker.C:
			cycleContext, cancel := context.WithTimeout(stopContext, 4*time.Second)
			published, err := workerStore.PublishDueSchedules(cycleContext, 50)
			cancel()
			if err != nil {
				logger.Error("worker cycle failed", "error", err)
				continue
			}
			if published > 0 {
				logger.Info("scheduled publications completed", "count", published)
			}
		}
	}
}
