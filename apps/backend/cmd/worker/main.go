package main

import (
	"context"
	"fmt"
	"log/slog"
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
	"seoblog/apps/backend/internal/platform/b2"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
	"seoblog/apps/backend/internal/webhooks"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

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
	var background sync.WaitGroup
	aiReports := make(chan aiCycleReport, 1)
	if aiProcessor != nil {
		background.Add(1)
		go runAIProcessor(stopContext, &background, aiProcessor, cfg.AITimeout, aiReports)
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
			published, publishErr := workerStore.PublishDueSchedules(cycleContext, 50)
			webhookResult, webhookErr := webhookProcessor.Process(cycleContext, 50, 2)
			delivered, failed, notificationErr := notificationProcessor.Process(cycleContext, 2)
			cancel()
			mediaContext, mediaCancel := context.WithTimeout(stopContext, 2*time.Minute)
			mediaProcessed, mediaErr := mediaProcessor.Process(mediaContext, 10)
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

func runAIProcessor(ctx context.Context, waitGroup *sync.WaitGroup, processor *aijobs.Processor, timeout time.Duration, reports chan<- aiCycleReport) {
	defer waitGroup.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cycleContext, cancel := context.WithTimeout(ctx, 2*timeout+30*time.Second)
			result, err := processor.Process(cycleContext, 2)
			cancel()
			select {
			case reports <- aiCycleReport{Result: result, Err: err}:
			case <-ctx.Done():
				return
			}
		}
	}
}
