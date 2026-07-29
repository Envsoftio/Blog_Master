package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingValuesWithoutOverriding(t *testing.T) {
	t.Setenv("SEOBLOG_TEST_EXISTING", "from-env")
	missingKey := "SEOBLOG_TEST_MISSING"
	_ = os.Unsetenv(missingKey)
	t.Cleanup(func() { _ = os.Unsetenv(missingKey) })

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte(`
SEOBLOG_TEST_EXISTING=from-file
SEOBLOG_TEST_MISSING="from-file"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadDotEnv(envPath); err != nil {
		t.Fatal(err)
	}
	if actual := os.Getenv("SEOBLOG_TEST_EXISTING"); actual != "from-env" {
		t.Fatalf("expected existing env to win, got %q", actual)
	}
	if actual := os.Getenv(missingKey); actual != "from-file" {
		t.Fatalf("expected missing value from .env, got %q", actual)
	}
}
