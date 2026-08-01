package main

import (
	"log/slog"
	"testing"

	"seoblog/apps/backend/internal/observability"
)

func TestWorkerMetricsServerRejectsNonLoopbackBind(t *testing.T) {
	server, err := startMetricsServer("0.0.0.0:9092", observability.NewRegistry(nil, "", "test", "test"), slog.Default())
	if err == nil {
		_ = server.Close()
		t.Fatal("expected a non-loopback worker metrics bind to be rejected")
	}
}

func TestWorkerMetricsServerRejectsMalformedAddress(t *testing.T) {
	server, err := startMetricsServer("not-an-address", observability.NewRegistry(nil, "", "test", "test"), slog.Default())
	if err == nil {
		_ = server.Close()
		t.Fatal("expected malformed worker metrics address to be rejected")
	}
}
