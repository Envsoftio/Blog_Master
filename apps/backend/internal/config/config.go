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
	AdminPublicURL string
	SMTPAddress    string
	SMTPUsername   string
	SMTPPassword   string
	SMTPFrom       string
	SMTPFromName   string
	SMTPRequireTLS bool
}

func Load() Config {
	_ = LoadDotEnv(".env", "../.env", "../../.env")
	return Config{
		Env:            env("SEOBLOG_ENV", "development"),
		HTTPAddr:       env("SEOBLOG_HTTP_ADDR", ":8080"),
		DBPath:         env("SEOBLOG_DB_PATH", "./seoblog.db"),
		DevAuth:        os.Getenv("SEOBLOG_DEV_AUTH") == "true",
		TrustedProxies: stringList(os.Getenv("SEOBLOG_TRUSTED_PROXIES")),
		AdminPublicURL: strings.TrimSpace(os.Getenv("SEOBLOG_ADMIN_PUBLIC_URL")),
		SMTPAddress:    strings.TrimSpace(os.Getenv("SEOBLOG_SMTP_ADDR")),
		SMTPUsername:   strings.TrimSpace(os.Getenv("SEOBLOG_SMTP_USERNAME")),
		SMTPPassword:   os.Getenv("SEOBLOG_SMTP_PASSWORD"),
		SMTPFrom:       env("SEOBLOG_SMTP_FROM", "no-reply@localhost"),
		SMTPFromName:   env("SEOBLOG_SMTP_FROM_NAME", "SEO Blog"),
		SMTPRequireTLS: envBool("SEOBLOG_SMTP_REQUIRE_STARTTLS"),
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

func envBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	return strings.EqualFold(value, "true") || value == "1"
}
