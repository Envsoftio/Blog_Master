package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"seoblog/apps/backend/internal/store"
)

type Processor struct {
	Store    *store.Store
	Sender   Sender
	Logger   *slog.Logger
	WorkerID string
}

type CycleResult struct {
	EventsFannedOut  int
	DeliveriesQueued int
	Succeeded        int
	Failed           int
}

type eventEnvelope struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	ProjectID     string          `json:"projectId"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	Data          json.RawMessage `json:"data"`
	CreatedAt     string          `json:"createdAt"`
}

func (p *Processor) Process(ctx context.Context, eventLimit, deliveryLimit int) (CycleResult, error) {
	if p.Store == nil {
		return CycleResult{}, fmt.Errorf("webhook processor store is required")
	}
	if p.Sender == nil {
		return CycleResult{}, fmt.Errorf("webhook processor sender is required")
	}
	now := time.Now().UTC()
	events, queued, err := p.Store.FanOutWebhookEvents(ctx, now, eventLimit)
	if err != nil {
		return CycleResult{}, err
	}
	result := CycleResult{EventsFannedOut: events, DeliveriesQueued: queued}
	deliveries, err := p.Store.ClaimWebhookDeliveries(ctx, p.WorkerID, now, deliveryLimit)
	if err != nil {
		return result, err
	}
	for _, delivery := range deliveries {
		outcome, sendErr := p.deliver(ctx, delivery)
		if sendErr == nil {
			if err := p.Store.MarkWebhookDeliverySucceeded(
				ctx,
				delivery.ID,
				p.WorkerID,
				time.Now().UTC(),
				outcome.StatusCode,
				outcome.ResponseDurationMillis,
			); err != nil {
				return result, err
			}
			result.Succeeded++
			continue
		}
		result.Failed++
		if err := p.Store.MarkWebhookDeliveryFailed(
			ctx,
			delivery,
			p.WorkerID,
			time.Now().UTC(),
			outcome,
		); err != nil {
			return result, err
		}
		if p.Logger != nil {
			p.Logger.Warn("webhook delivery failed",
				"delivery_id", delivery.ID,
				"event_id", delivery.OutboxEventID,
				"endpoint_id", delivery.EndpointID,
				"attempt", delivery.AttemptCount,
				"category", outcome.ErrorCategory,
				"retryable", outcome.Retryable,
			)
		}
	}
	return result, nil
}

func (p *Processor) deliver(ctx context.Context, delivery store.WebhookDelivery) (store.WebhookDeliveryOutcome, error) {
	if delivery.SigningSecretErrorSafe != "" {
		return store.WebhookDeliveryOutcome{
			ErrorCategory: "configuration",
			SafeMessage:   delivery.SigningSecretErrorSafe,
			Retryable:     false,
		}, errors.New(delivery.SigningSecretErrorSafe)
	}
	body, err := json.Marshal(eventEnvelope{
		ID:            delivery.OutboxEventID,
		Type:          delivery.EventType,
		ProjectID:     delivery.ProjectID,
		AggregateType: delivery.AggregateType,
		AggregateID:   delivery.AggregateID,
		Data:          delivery.Payload,
		CreatedAt:     delivery.EventCreatedAt,
	})
	if err != nil {
		return store.WebhookDeliveryOutcome{
			ErrorCategory: "payload",
			SafeMessage:   "webhook event could not be encoded",
			Retryable:     false,
		}, err
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	signature := signWebhook(delivery.SigningSecret, timestamp, delivery.OutboxEventID, body)
	headers := make(http.Header)
	headers.Set("X-SEOBlog-Event-ID", delivery.OutboxEventID)
	headers.Set("X-SEOBlog-Event-Type", delivery.EventType)
	headers.Set("X-SEOBlog-Timestamp", timestamp)
	headers.Set("X-SEOBlog-Signature", "v1="+signature)
	headers.Set("Idempotency-Key", delivery.OutboxEventID)
	sendResult, err := p.Sender.Send(ctx, delivery.EndpointURL, headers, body)
	if err == nil {
		return store.WebhookDeliveryOutcome{
			StatusCode:             sendResult.StatusCode,
			ResponseDurationMillis: sendResult.DurationMillis,
		}, nil
	}
	var deliveryErr *DeliveryError
	if errors.As(err, &deliveryErr) {
		return store.WebhookDeliveryOutcome{
			StatusCode:             deliveryErr.StatusCode,
			ResponseDurationMillis: deliveryErr.DurationMillis,
			ErrorCategory:          deliveryErr.Category,
			SafeMessage:            deliveryErr.SafeMessage,
			Retryable:              deliveryErr.Retryable,
		}, err
	}
	return store.WebhookDeliveryOutcome{
		ErrorCategory: "transport",
		SafeMessage:   "webhook receiver could not be reached",
		Retryable:     true,
	}, err
}

func signWebhook(secret, timestamp, eventID string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(eventID))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
