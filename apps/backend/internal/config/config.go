package config

import "os"

type Config struct {
	Env      string
	HTTPAddr string
	DBPath   string
	DevAuth  bool
}

func Load() Config {
	_ = LoadDotEnv(".env", "../.env", "../../.env")
	return Config{
		Env:      env("SEOBLOG_ENV", "development"),
		HTTPAddr: env("SEOBLOG_HTTP_ADDR", ":8080"),
		DBPath:   env("SEOBLOG_DB_PATH", "./seoblog.db"),
		DevAuth:  os.Getenv("SEOBLOG_DEV_AUTH") == "true",
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
