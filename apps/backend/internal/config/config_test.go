package config

import (
	"reflect"
	"testing"
	"time"
)

func TestStringListCleansTrustedProxyConfiguration(t *testing.T) {
	actual := stringList(" 127.0.0.1, 172.16.0.0/12 ,, ::1 ")
	expected := []string{"127.0.0.1", "172.16.0.0/12", "::1"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestLoadSMTPConfig(t *testing.T) {
	t.Setenv("SEOBLOG_SMTP_ADDR", "smtp.zeptomail.com:587")
	t.Setenv("SEOBLOG_SMTP_USERNAME", "emailapikey")
	t.Setenv("SEOBLOG_SMTP_PASSWORD", "secret")
	t.Setenv("SEOBLOG_SMTP_FROM", "noreply@proctorplus.io")
	t.Setenv("SEOBLOG_SMTP_FROM_NAME", "Example Team")
	t.Setenv("SEOBLOG_SMTP_REQUIRE_STARTTLS", "true")

	cfg := Load()
	if cfg.SMTPAddress != "smtp.zeptomail.com:587" {
		t.Fatalf("expected ZeptoMail address, got %q", cfg.SMTPAddress)
	}
	if cfg.SMTPUsername != "emailapikey" || cfg.SMTPPassword != "secret" {
		t.Fatal("expected SMTP credentials to be loaded")
	}
	if cfg.SMTPFrom != "noreply@proctorplus.io" || cfg.SMTPFromName != "Example Team" {
		t.Fatalf("unexpected sender identity: %q <%s>", cfg.SMTPFromName, cfg.SMTPFrom)
	}
	if !cfg.SMTPRequireTLS {
		t.Fatal("expected STARTTLS to be required")
	}
}

func TestLoadRedisConfig(t *testing.T) {
	t.Setenv("SEOBLOG_REDIS_ADDR", "redis:6379")
	t.Setenv("SEOBLOG_REDIS_PASSWORD", "redis-secret")
	t.Setenv("SEOBLOG_REDIS_TIMEOUT", "250ms")

	cfg := Load()
	if cfg.RedisAddr != "redis:6379" || cfg.RedisPassword != "redis-secret" {
		t.Fatalf("unexpected redis config: %#v", cfg)
	}
	if cfg.RedisTimeout.String() != "250ms" {
		t.Fatalf("expected redis timeout 250ms, got %s", cfg.RedisTimeout)
	}
}

func TestLoadWebhookSecurityConfig(t *testing.T) {
	t.Setenv("SEOBLOG_WEBHOOK_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("SEOBLOG_WEBHOOK_ALLOWED_HOSTS", "hooks.staging.example, *.preview.example")

	cfg := Load()
	if string(cfg.WebhookEncryptionKey) != "0123456789abcdef0123456789abcdef" {
		t.Fatal("expected a decoded 32-byte webhook encryption key")
	}
	expected := []string{"hooks.staging.example", "*.preview.example"}
	if !reflect.DeepEqual(cfg.WebhookAllowedHosts, expected) {
		t.Fatalf("expected %#v, got %#v", expected, cfg.WebhookAllowedHosts)
	}
}

func TestInvalidWebhookEncryptionKeyIsDisabled(t *testing.T) {
	t.Setenv("SEOBLOG_WEBHOOK_ENCRYPTION_KEY", "not-base64")
	if key := Load().WebhookEncryptionKey; key != nil {
		t.Fatalf("expected invalid webhook key to be disabled, got %d bytes", len(key))
	}
}

func TestLoadAIExecutionConfigRequiresCompleteProviderSettings(t *testing.T) {
	t.Setenv("SEOBLOG_AI_PROVIDER", "compatible-test")
	t.Setenv("SEOBLOG_AI_BASE_URL", "https://provider.example.test/v1/")
	t.Setenv("SEOBLOG_AI_API_KEY", "secret")
	t.Setenv("SEOBLOG_AI_MODEL", "model")
	t.Setenv("SEOBLOG_AI_TIMEOUT", "45s")
	t.Setenv("SEOBLOG_AI_MAX_INPUT_BYTES", "4096")
	t.Setenv("SEOBLOG_AI_MAX_OUTPUT_TOKENS", "700")

	cfg := Load()
	if !cfg.AIEnabled() || cfg.AIBaseURL != "https://provider.example.test/v1" {
		t.Fatalf("unexpected AI provider config: %#v", cfg)
	}
	if cfg.AIProvider != "compatible-test" || cfg.AIModel != "model" || cfg.AITimeout.String() != "45s" {
		t.Fatalf("unexpected AI execution identity: %#v", cfg)
	}
	if cfg.AIMaxInputBytes != 4096 || cfg.AIMaxOutputTokens != 700 {
		t.Fatalf("unexpected AI execution budgets: %#v", cfg)
	}
}

func TestLoadBoundsAIExecutionTimeout(t *testing.T) {
	t.Setenv("SEOBLOG_AI_TIMEOUT", "2h")
	if got := Load().AITimeout; got != 90*time.Second {
		t.Fatalf("expected unsafe AI timeout to fall back to 90s, got %s", got)
	}
}
