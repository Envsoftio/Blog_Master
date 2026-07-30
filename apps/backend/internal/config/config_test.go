package config

import (
	"reflect"
	"testing"
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
