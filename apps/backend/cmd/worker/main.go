package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"seoblog/apps/backend/internal/aijobs"
	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/mailer"
	"seoblog/apps/backend/internal/mediajobs"
	"seoblog/apps/backend/internal/notifications"
	"seoblog/apps/backend/internal/observability"
	"seoblog/apps/backend/internal/platform/b2"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
	"seoblog/apps/backend/internal/webhooks"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "seoblog-worker", "environment", cfg.Env)

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

	workerStore := store.New(db, store.WithWebhookEncryptionKey(cfg.WebhookEncryptionKey))
	metrics := observability.NewRegistry(db, cfg.DBPath, "seoblog-worker", cfg.Env)
	workerID := "worker-" + fmt.Sprint(os.Getpid())
	notificationProcessor := notifications.Processor{
		Store: workerStore,
		Sender: mailer.NewSMTP(mailer.SMTPConfig{
			Address:         cfg.SMTPAddress,
			Username:        cfg.SMTPUsername,
			Password:        cfg.SMTPPassword,
			From:            cfg.SMTPFrom,
			FromName:        cfg.SMTPFromName,
			RequireStartTLS: cfg.SMTPRequireTLS,
		}),
		Logger:         logger,
		WorkerID:       workerID,
		AdminPublicURL: cfg.AdminPublicURL,
	}
	webhookProcessor := webhooks.Processor{
		Store: workerStore,
		Sender: webhooks.NewHTTPSender(webhooks.DestinationPolicy{
			Environment:  cfg.Env,
			AllowedHosts: cfg.WebhookAllowedHosts,
		}),
		Logger:   logger,
		WorkerID: workerID,
	}
	var mediaStorage mediajobs.Storage
	if cfg.B2MediaEnabled() {
		client, err := b2.New(b2.Config{
			Endpoint:             cfg.B2MediaEndpoint,
			Region:               cfg.B2MediaRegion,
			Bucket:               cfg.B2MediaBucket,
			KeyID:                cfg.B2MediaKeyID,
			ApplicationKey:       cfg.B2MediaApplicationKey,
			PublicBaseURL:        cfg.B2MediaPublicBaseURL,
			PresignTTL:           cfg.B2MediaPresignTTL,
			ServerSideEncryption: cfg.B2MediaSSE,
		})
		if err != nil {
			logger.Error("B2 media storage disabled", "error", err)
		} else {
			mediaStorage = client
		}
	}
	mediaProcessor := mediajobs.Processor{
		Store:   workerStore,
		Storage: mediaStorage,
		Logger:  logger,
	}
	var aiProcessor *aijobs.Processor
	if cfg.AIEnabled() {
		generator, err := aijobs.NewOpenAICompatibleClient(aijobs.ClientConfig{
			BaseURL:         cfg.AIBaseURL,
			APIKey:          cfg.AIAPIKey,
			Model:           cfg.AIModel,
			MaxOutputTokens: cfg.AIMaxOutputTokens,
			Timeout:         cfg.AITimeout,
		})
		if err != nil {
			logger.Error("AI execution disabled", "error", err)
		} else {
			aiProcessor = &aijobs.Processor{
				Store:         workerStore,
				Generator:     generator,
				Logger:        logger,
				WorkerID:      workerID,
				Provider:      cfg.AIProvider,
				Model:         cfg.AIModel,
				MaxInputBytes: cfg.AIMaxInputBytes,
				LeaseDuration: cfg.AITimeout + 30*time.Second,
			}
		}
	} else {
		logger.Info("AI execution disabled; configure SEOBLOG_AI_BASE_URL, SEOBLOG_AI_API_KEY, and SEOBLOG_AI_MODEL to enable it")
	}
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metricsServer, err := startMetricsServer(cfg.WorkerMetricsAddr, metrics, logger)
	if err != nil {
		logger.Error("start worker metrics server", "addr", cfg.WorkerMetricsAddr, "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsServer.Shutdown(shutdownContext); err != nil {
			logger.Error("stop worker metrics server", "error", err)
		}
	}()
	var background sync.WaitGroup
	aiReports := make(chan aiCycleReport, 1)
	if aiProcessor != nil {
		background.Add(1)
		go runAIProcessor(stopContext, &background, aiProcessor, cfg.AITimeout, metrics, aiReports)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Info("worker ready", "poll_interval", "5s")
	for {
		select {
		case <-stopContext.Done():
			background.Wait()
			logger.Info("worker stopped")
			return
		case report := <-aiReports:
			if report.Err != nil {
				logger.Error("AI processing cycle failed", "error", report.Err)
			}
			if report.Result.Claimed > 0 {
				logger.Info("AI jobs processed", "claimed", report.Result.Claimed, "succeeded", report.Result.Succeeded, "retried", report.Result.Retried, "failed", report.Result.Failed)
			}
		case <-ticker.C:
			cycleContext, cancel := context.WithTimeout(stopContext, 30*time.Second)
			publishStartedAt := time.Now()
			published, publishErr := workerStore.PublishDueSchedules(cycleContext, 50)
			metrics.RecordWorkerCycle("scheduler", cycleOutcome(publishErr), time.Since(publishStartedAt), published)
			webhookStartedAt := time.Now()
			webhookResult, webhookErr := webhookProcessor.Process(cycleContext, 50, 2)
			metrics.RecordWorkerCycle("webhook", cycleOutcome(webhookErr), time.Since(webhookStartedAt), webhookResult.Succeeded+webhookResult.Failed)
			emailStartedAt := time.Now()
			delivered, failed, notificationErr := notificationProcessor.Process(cycleContext, 2)
			metrics.RecordWorkerCycle("email", cycleOutcome(notificationErr), time.Since(emailStartedAt), delivered+failed)
			cancel()
			mediaContext, mediaCancel := context.WithTimeout(stopContext, 2*time.Minute)
			mediaStartedAt := time.Now()
			mediaProcessed, mediaErr := mediaProcessor.Process(mediaContext, 10)
			metrics.RecordWorkerCycle("media", cycleOutcome(mediaErr), time.Since(mediaStartedAt), mediaProcessed)
			mediaCancel()
			if publishErr != nil {
				logger.Error("scheduled publication cycle failed", "error", publishErr)
			}
			if notificationErr != nil {
				logger.Error("notification cycle failed", "error", notificationErr)
			}
			if webhookErr != nil {
				logger.Error("webhook cycle failed", "error", webhookErr)
			}
			if mediaErr != nil {
				logger.Error("media processing cycle failed", "error", mediaErr)
			}
			if published > 0 {
				logger.Info("scheduled publications completed", "count", published)
			}
			if delivered > 0 || failed > 0 {
				logger.Info("review assignment notifications processed", "delivered", delivered, "failed", failed)
			}
			if webhookResult.EventsFannedOut > 0 ||
				webhookResult.Succeeded > 0 ||
				webhookResult.Failed > 0 {
				logger.Info("webhook deliveries processed",
					"events", webhookResult.EventsFannedOut,
					"queued", webhookResult.DeliveriesQueued,
					"succeeded", webhookResult.Succeeded,
					"failed", webhookResult.Failed,
				)
			}
			if mediaProcessed > 0 {
				logger.Info("media assets processed", "count", mediaProcessed)
			}
		}
	}
}

type aiCycleReport struct {
	Result aijobs.ProcessResult
	Err    error
}

func runAIProcessor(ctx context.Context, waitGroup *sync.WaitGroup, processor *aijobs.Processor, timeout time.Duration, metrics *observability.Registry, reports chan<- aiCycleReport) {
	defer waitGroup.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cycleContext, cancel := context.WithTimeout(ctx, 2*timeout+30*time.Second)
			startedAt := time.Now()
			result, err := processor.Process(cycleContext, 2)
			metrics.RecordWorkerCycle("ai", cycleOutcome(err), time.Since(startedAt), result.Claimed)
			cancel()
			select {
			case reports <- aiCycleReport{Result: result, Err: err}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func cycleOutcome(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func startMetricsServer(addr string, registry *observability.Registry, logger *slog.Logger) (*http.Server, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid metrics address: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, fmt.Errorf("worker metrics must bind to a loopback address")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if err := registry.Render(request.Context(), writer); err != nil {
			logger.Error("collect worker metrics", "error_category", "metrics_collection", "error", err)
			http.Error(writer, "metrics unavailable", http.StatusServiceUnavailable)
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("worker metrics server stopped", "error", err)
		}
	}()
	return server, nil
}
