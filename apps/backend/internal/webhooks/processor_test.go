package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
)

type recordedWebhookRequest struct {
	Destination string
	Headers     http.Header
	Body        []byte
}

type senderOutcome struct {
	result SendResult
	err    error
}

type recordingWebhookSender struct {
	requests []recordedWebhookRequest
	outcomes []senderOutcome
}

func (s *recordingWebhookSender) Send(
	_ context.Context,
	destination string,
	headers http.Header,
	body []byte,
) (SendResult, error) {
	s.requests = append(s.requests, recordedWebhookRequest{
		Destination: destination,
		Headers:     headers.Clone(),
		Body:        append([]byte(nil), body...),
	})
	if len(s.outcomes) == 0 {
		return SendResult{StatusCode: http.StatusNoContent, DurationMillis: 4}, nil
	}
	outcome := s.outcomes[0]
	s.outcomes = s.outcomes[1:]
	return outcome.result, outcome.err
}

func TestProcessorFansOutSignsAndDeliversIdempotently(t *testing.T) {
	webhookStore, db, secret := newWebhookTestStore(t)
	insertWebhookEvent(t, db, "event-published", "content.published", `{"content_id":"article"}`)
	sender := &recordingWebhookSender{}
	processor := Processor{Store: webhookStore, Sender: sender, WorkerID: "worker-test"}

	result, err := processor.Process(context.Background(), 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.EventsFannedOut != 1 ||
		result.DeliveriesQueued != 1 ||
		result.Succeeded != 1 ||
		result.Failed != 0 {
		t.Fatalf("unexpected processor result %#v", result)
	}
	if len(sender.requests) != 1 {
		t.Fatalf("expected one webhook request, got %d", len(sender.requests))
	}
	request := sender.requests[0]
	if request.Destination != "https://hooks.example.test/revalidate" {
		t.Fatalf("unexpected destination %q", request.Destination)
	}
	timestamp := request.Headers.Get("X-SEOBlog-Timestamp")
	expectedSignature := "v1=" + signWebhook(secret, timestamp, "event-published", request.Body)
	if request.Headers.Get("X-SEOBlog-Signature") != expectedSignature {
		t.Fatalf("signature did not cover timestamp, event ID and raw body")
	}
	if request.Headers.Get("Idempotency-Key") != "event-published" {
		t.Fatalf("expected event ID idempotency key, got %q", request.Headers.Get("Idempotency-Key"))
	}
	var envelope eventEnvelope
	if err := json.Unmarshal(request.Body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ID != "event-published" ||
		envelope.Type != "content.published" ||
		envelope.ProjectID != "project" ||
		envelope.AggregateID != "article" ||
		string(envelope.Data) != `{"content_id":"article"}` {
		t.Fatalf("unexpected event envelope %#v", envelope)
	}

	var status string
	var attemptCount int
	if err := db.QueryRow(`
		SELECT status, attempt_count
		FROM webhook_attempts
		WHERE outbox_event_id = 'event-published'
	`).Scan(&status, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || attemptCount != 1 {
		t.Fatalf("unexpected durable delivery status=%q attempts=%d", status, attemptCount)
	}
	var webhookFannedOutAt, processedAt string
	if err := db.QueryRow(`
		SELECT COALESCE(webhook_fanned_out_at, ''), COALESCE(processed_at, '')
		FROM outbox_events
		WHERE id = 'event-published'
	`).Scan(&webhookFannedOutAt, &processedAt); err != nil {
		t.Fatal(err)
	}
	if webhookFannedOutAt == "" || processedAt != "" {
		t.Fatalf("webhook fan-out must not consume shared outbox state, fanout=%q processed=%q", webhookFannedOutAt, processedAt)
	}

	second, err := processor.Process(context.Background(), 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if second.EventsFannedOut != 0 ||
		second.DeliveriesQueued != 0 ||
		second.Succeeded != 0 ||
		len(sender.requests) != 1 {
		t.Fatalf("expected completed event to remain idempotent, result=%#v requests=%d", second, len(sender.requests))
	}
}

func TestProcessorRetriesTransientFailuresAndStopsOnPermanentFailures(t *testing.T) {
	webhookStore, db, _ := newWebhookTestStore(t)
	insertWebhookEvent(t, db, "event-retry", "content.updated", `{"content_id":"article"}`)
	sender := &recordingWebhookSender{outcomes: []senderOutcome{
		{err: &DeliveryError{
			Category:       "http_503",
			SafeMessage:    "webhook receiver returned HTTP 503",
			StatusCode:     http.StatusServiceUnavailable,
			DurationMillis: 8,
			Retryable:      true,
		}},
		{result: SendResult{StatusCode: http.StatusAccepted, DurationMillis: 3}},
	}}
	processor := Processor{Store: webhookStore, Sender: sender, WorkerID: "worker-retry"}
	first, err := processor.Process(context.Background(), 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if first.Failed != 1 {
		t.Fatalf("expected transient failure, got %#v", first)
	}
	var deliveryID, status, nextAttemptAt string
	var attemptCount int
	if err := db.QueryRow(`
		SELECT id, status, attempt_count, next_attempt_at
		FROM webhook_attempts
		WHERE outbox_event_id = 'event-retry'
	`).Scan(&deliveryID, &status, &attemptCount, &nextAttemptAt); err != nil {
		t.Fatal(err)
	}
	if status != "retrying" || attemptCount != 1 || nextAttemptAt == "" {
		t.Fatalf("unexpected retry state status=%q attempts=%d next=%q", status, attemptCount, nextAttemptAt)
	}
	if _, err := db.Exec(`
		UPDATE webhook_attempts
		SET next_attempt_at = datetime(CURRENT_TIMESTAMP, '-1 second')
		WHERE id = ?
	`, deliveryID); err != nil {
		t.Fatal(err)
	}
	second, err := processor.Process(context.Background(), 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if second.Succeeded != 1 {
		t.Fatalf("expected retry success, got %#v", second)
	}
	if err := db.QueryRow(`
		SELECT status, attempt_count
		FROM webhook_attempts
		WHERE id = ?
	`, deliveryID).Scan(&status, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || attemptCount != 2 {
		t.Fatalf("unexpected retry completion status=%q attempts=%d", status, attemptCount)
	}

	insertWebhookEvent(t, db, "event-permanent", "content.unpublished", `{"content_id":"article"}`)
	sender.outcomes = []senderOutcome{{err: &DeliveryError{
		Category:    "http_400",
		SafeMessage: "webhook receiver returned HTTP 400",
		StatusCode:  http.StatusBadRequest,
		Retryable:   false,
	}}}
	permanent, err := processor.Process(context.Background(), 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if permanent.Failed != 1 {
		t.Fatalf("expected permanent failure, got %#v", permanent)
	}
	var category string
	if err := db.QueryRow(`
		SELECT status, error_category, attempt_count
		FROM webhook_attempts
		WHERE outbox_event_id = 'event-permanent'
	`).Scan(&status, &category, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || category != "http_400" || attemptCount != 1 {
		t.Fatalf("unexpected permanent state status=%q category=%q attempts=%d", status, category, attemptCount)
	}
}

func TestRevokedEndpointSuppressesQueuedAndExpiredDeliveries(t *testing.T) {
	webhookStore, db, _ := newWebhookTestStore(t)
	insertWebhookEvent(t, db, "event-revoked", "content.published", `{}`)
	if events, queued, err := webhookStore.FanOutWebhookEvents(context.Background(), time.Now().UTC(), 10); err != nil {
		t.Fatal(err)
	} else if events != 1 || queued != 1 {
		t.Fatalf("unexpected fan-out events=%d queued=%d", events, queued)
	}
	if _, err := db.Exec(`
		UPDATE webhook_attempts
		SET status = 'processing',
		    attempt_count = 1,
		    locked_by = 'crashed-worker',
		    locked_until = datetime(CURRENT_TIMESTAMP, '-1 minute')
		WHERE outbox_event_id = 'event-revoked'
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := webhookStore.RevokeWebhook(context.Background(), "owner", "project", "endpoint"); err != nil {
		t.Fatal(err)
	}
	deliveries, err := webhookStore.ClaimWebhookDeliveries(context.Background(), "new-worker", time.Now().UTC(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 0 {
		t.Fatalf("expected revoked endpoint delivery to be suppressed, got %#v", deliveries)
	}
	var status string
	if err := db.QueryRow(`
		SELECT status
		FROM webhook_attempts
		WHERE outbox_event_id = 'event-revoked'
	`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "suppressed" {
		t.Fatalf("expected suppressed delivery, got %q", status)
	}
}

func newWebhookTestStore(t *testing.T) (*store.Store, *sql.DB, string) {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "webhooks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO projects(id, slug, name, public_project_key)
		VALUES ('project', 'project', 'Project', 'public-project');
		INSERT INTO users(id, email_normalized, status)
		VALUES ('owner', 'owner@example.test', 'active');
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES ('project', 'owner', 'project_owner', 'active', CURRENT_TIMESTAMP);
	`); err != nil {
		t.Fatal(err)
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	webhookStore := store.New(db, store.WithWebhookEncryptionKey(key))
	created, err := webhookStore.CreateWebhook(context.Background(), "owner", "project", store.WebhookInput{
		Name:   "Production",
		URL:    "https://hooks.example.test/revalidate",
		Events: []string{"content.published", "content.updated", "content.unpublished"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE webhook_endpoints SET id = 'endpoint' WHERE id = ?`, created.ID); err != nil {
		t.Fatal(err)
	}
	return webhookStore, db, created.Secret
}

func insertWebhookEvent(t *testing.T, db *sql.DB, eventID, eventType, payload string) {
	t.Helper()
	if !json.Valid([]byte(payload)) {
		t.Fatalf("invalid test payload %q", payload)
	}
	if _, err := db.Exec(`
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (?, 'project', ?, 'content', 'article', ?, ?)
	`, eventID, eventType, payload, "key-"+eventID); err != nil {
		t.Fatal(err)
	}
}

func TestProcessorRejectsMissingDependencies(t *testing.T) {
	_, err := (&Processor{}).Process(context.Background(), 1, 1)
	if err == nil {
		t.Fatal("expected missing store to fail")
	}
	_, err = (&Processor{Store: &store.Store{}}).Process(context.Background(), 1, 1)
	if err == nil {
		t.Fatal("expected missing sender to fail")
	}
}
