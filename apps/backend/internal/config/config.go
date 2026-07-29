package config

import (
	"os"
	"strings"
)

type Config struct {
	Env            string
	HTTPAddr       string
	DBPath         string
	DevAuth        bool
	TrustedProxies []string
}

func Load() Config {
	_ = LoadDotEnv(".env", "../.env", "../../.env")
	return Config{
		Env:            env("SEOBLOG_ENV", "development"),
		HTTPAddr:       env("SEOBLOG_HTTP_ADDR", ":8080"),
		DBPath:         env("SEOBLOG_DB_PATH", "./seoblog.db"),
		DevAuth:        os.Getenv("SEOBLOG_DEV_AUTH") == "true",
		TrustedProxies: stringList(os.Getenv("SEOBLOG_TRUSTED_PROXIES")),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func stringList(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}
