package observability_test

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"seoblog/apps/backend/internal/observability"
)

func TestRegistryRendersBoundedProcessAndDeliveryMetrics(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "metrics.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE outbox_events (created_at TEXT NOT NULL, webhook_fanned_out_at TEXT)`,
		`CREATE TABLE webhook_attempts (status TEXT NOT NULL, attempt_count INTEGER NOT NULL, response_duration_ms INTEGER)`,
		`CREATE TABLE assets (status TEXT NOT NULL)`,
		`CREATE TABLE ai_jobs (status TEXT NOT NULL, estimated_cost_cents INTEGER)`,
		`CREATE TABLE review_assignment_notifications (status TEXT NOT NULL)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	createdAt := time.Now().UTC().Add(-3 * time.Minute).Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO outbox_events(created_at) VALUES ('` + createdAt + `')`,
		`INSERT INTO webhook_attempts(status, attempt_count, response_duration_ms) VALUES ('dead_letter', 3, 250)`,
		`INSERT INTO assets(status) VALUES ('failed')`,
		`INSERT INTO ai_jobs(status, estimated_cost_cents) VALUES ('failed', 42)`,
		`INSERT INTO review_assignment_notifications(status) VALUES ('dead_letter')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	registry := observability.NewRegistry(db, databasePath, "seoblog-api", "test")
	registry.RecordHTTPRequest("GET", "/content/v1/articles/{slug}", 200, 75*time.Millisecond)
	registry.RecordCache("hit")
	registry.RecordWorkerCycle("webhook", "error", 250*time.Millisecond, 1)
	var output bytes.Buffer
	if err := registry.Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	metrics := output.String()
	for _, expected := range []string{
		`seoblog_http_requests_total{environment="test",method="GET",route="/content/v1/articles/{slug}",service="seoblog-api",status="200"} 1`,
		`seoblog_content_cache_events_total{environment="test",outcome="hit",service="seoblog-api"} 1`,
		`seoblog_worker_cycles_total{component="webhook",environment="test",outcome="error",service="seoblog-api"} 1`,
		`seoblog_outbox_pending_events{environment="test",service="seoblog-api"} 1`,
		`seoblog_webhook_attempts{environment="test",service="seoblog-api",status="dead_letter"} 1`,
		`seoblog_media_assets{environment="test",service="seoblog-api",status="failed"} 1`,
		`seoblog_ai_estimated_cost_cents_total{environment="test",service="seoblog-api"} 42`,
	} {
		if !strings.Contains(metrics, expected) {
			t.Fatalf("expected metrics output to contain %q\n%s", expected, metrics)
		}
	}
	if strings.Contains(metrics, createdAt) {
		t.Fatal("metrics must not expose record-level timestamps or identifiers")
	}
}

func TestRegistryBoundsDynamicLabelValues(t *testing.T) {
	registry := observability.NewRegistry(nil, "", "worker", "test")
	registry.RecordHTTPRequest("get", strings.Repeat("x", 201), 404, time.Millisecond)
	registry.RecordCache("attacker-controlled")
	registry.RecordWorkerCycle("attacker-controlled", "attacker-controlled", time.Millisecond, 1)
	var output bytes.Buffer
	if err := registry.Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	metrics := output.String()
	for _, expected := range []string{`route="unmatched"`, `outcome="other"`, `component="other"`} {
		if !strings.Contains(metrics, expected) {
			t.Fatalf("expected bounded label %q in output\n%s", expected, metrics)
		}
	}
}
