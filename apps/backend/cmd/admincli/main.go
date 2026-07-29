package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/httpapi"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/security"
	"seoblog/apps/backend/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: admincli migrate | openapi [output-path] | bootstrap-owner -email owner@example.com")
		os.Exit(2)
	}

	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	switch os.Args[1] {
	case "migrate":
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
		logger.Info("migrations applied", "db_path", cfg.DBPath)
	case "openapi":
		outputPath := "../../contracts/openapi/openapi.yaml"
		if len(os.Args) >= 3 {
			outputPath = os.Args[2]
		}
		db, err := database.OpenSQLite(":memory:")
		if err != nil {
			logger.Error("open in-memory database", "error", err)
			os.Exit(1)
		}
		defer db.Close()
		if err := database.Migrate(db); err != nil {
			logger.Error("migrate in-memory database", "error", err)
			os.Exit(1)
		}
		server := httpapi.New(httpapi.Options{Config: cfg, Logger: logger, Store: store.New(db)})
		spec, err := server.OpenAPIYAML()
		if err != nil {
			logger.Error("generate OpenAPI", "error", err)
			os.Exit(1)
		}
		if err := os.WriteFile(outputPath, spec, 0o644); err != nil {
			logger.Error("write OpenAPI", "path", outputPath, "error", err)
			os.Exit(1)
		}
		logger.Info("OpenAPI generated", "path", outputPath)
	case "bootstrap-owner":
		flags := flag.NewFlagSet("bootstrap-owner", flag.ExitOnError)
		email := flags.String("email", "", "owner email address")
		if err := flags.Parse(os.Args[2:]); err != nil {
			os.Exit(2)
		}
		if *email == "" {
			fmt.Fprintln(os.Stderr, "bootstrap-owner requires -email")
			os.Exit(2)
		}
		password := os.Getenv("SEOBLOG_BOOTSTRAP_PASSWORD")
		if password == "" {
			fmt.Fprintln(os.Stderr, "bootstrap-owner requires SEOBLOG_BOOTSTRAP_PASSWORD in .env or the process environment")
			os.Exit(2)
		}
		passwordHash, err := security.HashPassword(password)
		if err != nil {
			logger.Error("hash password", "error", err)
			os.Exit(1)
		}
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
		user, err := store.New(db).BootstrapOwner(context.Background(), *email, passwordHash)
		if err != nil {
			logger.Error("bootstrap owner", "error", err)
			os.Exit(1)
		}
		logger.Info("owner bootstrapped", "user_id", user.ID, "email", user.Email)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}
