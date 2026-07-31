package config

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                   string
	HTTPAddr              string
	DBPath                string
	DevAuth               bool
	TrustedProxies        []string
	RedisAddr             string
	RedisPassword         string
	RedisTimeout          time.Duration
	AdminPublicURL        string
	SMTPAddress           string
	SMTPUsername          string
	SMTPPassword          string
	SMTPFrom              string
	SMTPFromName          string
	SMTPRequireTLS        bool
	WebhookEncryptionKey  []byte
	WebhookAllowedHosts   []string
	B2MediaEndpoint       string
	B2MediaRegion         string
	B2MediaBucket         string
	B2MediaKeyID          string
	B2MediaApplicationKey string
	B2MediaPublicBaseURL  string
	B2MediaPresignTTL     time.Duration
	B2MediaSSE            string
	AIProvider            string
	AIBaseURL             string
	AIAPIKey              string
	AIModel               string
	AITimeout             time.Duration
	AIMaxInputBytes       int
	AIMaxOutputTokens     int
}

func Load() Config {
	_ = LoadDotEnv(".env", "../.env", "../../.env")
	return Config{
		Env:                   env("SEOBLOG_ENV", "development"),
		HTTPAddr:              env("SEOBLOG_HTTP_ADDR", ":8080"),
		DBPath:                env("SEOBLOG_DB_PATH", "./seoblog.db"),
		DevAuth:               os.Getenv("SEOBLOG_DEV_AUTH") == "true",
		TrustedProxies:        stringList(os.Getenv("SEOBLOG_TRUSTED_PROXIES")),
		RedisAddr:             strings.TrimSpace(os.Getenv("SEOBLOG_REDIS_ADDR")),
		RedisPassword:         os.Getenv("SEOBLOG_REDIS_PASSWORD"),
		RedisTimeout:          envDuration("SEOBLOG_REDIS_TIMEOUT", 150*time.Millisecond),
		AdminPublicURL:        strings.TrimSpace(os.Getenv("SEOBLOG_ADMIN_PUBLIC_URL")),
		SMTPAddress:           strings.TrimSpace(os.Getenv("SEOBLOG_SMTP_ADDR")),
		SMTPUsername:          strings.TrimSpace(os.Getenv("SEOBLOG_SMTP_USERNAME")),
		SMTPPassword:          os.Getenv("SEOBLOG_SMTP_PASSWORD"),
		SMTPFrom:              env("SEOBLOG_SMTP_FROM", "no-reply@localhost"),
		SMTPFromName:          env("SEOBLOG_SMTP_FROM_NAME", "SEO Blog"),
		SMTPRequireTLS:        envBool("SEOBLOG_SMTP_REQUIRE_STARTTLS"),
		WebhookEncryptionKey:  webhookEncryptionKey(os.Getenv("SEOBLOG_WEBHOOK_ENCRYPTION_KEY")),
		WebhookAllowedHosts:   stringList(os.Getenv("SEOBLOG_WEBHOOK_ALLOWED_HOSTS")),
		B2MediaEndpoint:       strings.TrimSpace(os.Getenv("SEOBLOG_B2_MEDIA_ENDPOINT")),
		B2MediaRegion:         env("SEOBLOG_B2_MEDIA_REGION", "us-west-004"),
		B2MediaBucket:         strings.TrimSpace(os.Getenv("SEOBLOG_B2_MEDIA_BUCKET")),
		B2MediaKeyID:          strings.TrimSpace(os.Getenv("SEOBLOG_B2_MEDIA_KEY_ID")),
		B2MediaApplicationKey: os.Getenv("SEOBLOG_B2_MEDIA_APPLICATION_KEY"),
		B2MediaPublicBaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("SEOBLOG_B2_MEDIA_PUBLIC_BASE_URL")), "/"),
		B2MediaPresignTTL:     envDuration("SEOBLOG_B2_MEDIA_PRESIGN_TTL", 15*time.Minute),
		B2MediaSSE:            strings.TrimSpace(os.Getenv("SEOBLOG_B2_MEDIA_SSE")),
		AIProvider:            env("SEOBLOG_AI_PROVIDER", "openai-compatible"),
		AIBaseURL:             strings.TrimRight(strings.TrimSpace(os.Getenv("SEOBLOG_AI_BASE_URL")), "/"),
		AIAPIKey:              os.Getenv("SEOBLOG_AI_API_KEY"),
		AIModel:               strings.TrimSpace(os.Getenv("SEOBLOG_AI_MODEL")),
		AITimeout:             envBoundedDuration("SEOBLOG_AI_TIMEOUT", 90*time.Second, 10*time.Second, 10*time.Minute),
		AIMaxInputBytes:       envPositiveInt("SEOBLOG_AI_MAX_INPUT_BYTES", 256*1024),
		AIMaxOutputTokens:     envPositiveInt("SEOBLOG_AI_MAX_OUTPUT_TOKENS", 4096),
	}
}

func (c Config) B2MediaEnabled() bool {
	return c.B2MediaEndpoint != "" &&
		c.B2MediaBucket != "" &&
		c.B2MediaKeyID != "" &&
		c.B2MediaApplicationKey != ""
}

func (c Config) AIEnabled() bool {
	return c.AIBaseURL != "" && c.AIAPIKey != "" && c.AIModel != ""
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

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func envBoundedDuration(key string, fallback, minimum, maximum time.Duration) time.Duration {
	duration := envDuration(key, fallback)
	if duration < minimum || duration > maximum {
		return fallback
	}
	return duration
}

func envPositiveInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func webhookEncryptionKey(raw string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(decoded) != 32 {
		return nil
	}
	return decoded
}
