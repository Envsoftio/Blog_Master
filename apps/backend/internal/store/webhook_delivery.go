package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"seoblog/apps/backend/internal/security"
)

const webhookDeliveryLockDuration = 2 * time.Minute

type WebhookDelivery struct {
	ID                     string
	ProjectID              string
	EndpointID             string
	EndpointURL            string
	OutboxEventID          string
	EventType              string
	AggregateType          string
	AggregateID            string
	Payload                json.RawMessage
	EventCreatedAt         string
	AttemptCount           int
	MaxAttempts            int
	SigningSecret          string
	SigningSecretErrorSafe string
}

type WebhookDeliveryOutcome struct {
	StatusCode             int
	ResponseDurationMillis int64
	ErrorCategory          string
	SafeMessage            string
	Retryable              bool
}

type webhookOutboxEvent struct {
	ID        string
	ProjectID string
	EventType string
}

func (s *Store) FanOutWebhookEvents(ctx context.Context, now time.Time, limit int) (int, int, error) {
	if limit <= 0 {
		return 0, 0, fmt.Errorf("%w: webhook outbox batch size must be positive", ErrValidation)
	}
	nowValue := now.UTC().Format(timeFormat)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, project_id, event_type
		FROM outbox_events
		WHERE webhook_fanned_out_at IS NULL
		  AND available_at <= ?
		ORDER BY available_at, created_at, id
		LIMIT ?
	`, nowValue, limit)
	if err != nil {
		return 0, 0, err
	}
	events := make([]webhookOutboxEvent, 0, limit)
	for rows.Next() {
		var event webhookOutboxEvent
		if err := rows.Scan(&event.ID, &event.ProjectID, &event.EventType); err != nil {
			rows.Close()
			return 0, 0, err
		}
		events = append(events, event)
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	created := 0
	for _, event := range events {
		endpointRows, err := tx.QueryContext(ctx, `
			SELECT id
			FROM webhook_endpoints
			WHERE project_id = ?
			  AND status = 'active'
			  AND EXISTS (
			    SELECT 1
			    FROM json_each(webhook_endpoints.events_json)
			    WHERE json_each.value = ?
			  )
			ORDER BY id
		`, event.ProjectID, event.EventType)
		if err != nil {
			return 0, 0, err
		}
		var endpointIDs []string
		for endpointRows.Next() {
			var endpointID string
			if err := endpointRows.Scan(&endpointID); err != nil {
				endpointRows.Close()
				return 0, 0, err
			}
			endpointIDs = append(endpointIDs, endpointID)
		}
		if err := endpointRows.Close(); err != nil {
			return 0, 0, err
		}
		if err := endpointRows.Err(); err != nil {
			return 0, 0, err
		}
		for _, endpointID := range endpointIDs {
			attemptID, err := security.RandomID("wha")
			if err != nil {
				return 0, 0, err
			}
			result, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO webhook_attempts(
				  id, project_id, endpoint_id, outbox_event_id, status,
				  next_attempt_at
				) VALUES (?, ?, ?, ?, 'queued', ?)
			`, attemptID, event.ProjectID, endpointID, event.ID, nowValue)
			if err != nil {
				return 0, 0, err
			}
			inserted, err := result.RowsAffected()
			if err != nil {
				return 0, 0, err
			}
			created += int(inserted)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE outbox_events
			SET webhook_fanned_out_at = ?
			WHERE project_id = ? AND id = ? AND webhook_fanned_out_at IS NULL
		`, nowValue, event.ProjectID, event.ID)
		if err != nil {
			return 0, 0, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return 0, 0, err
		}
		if changed != 1 {
			return 0, 0, fmt.Errorf("webhook outbox event %s was concurrently processed", event.ID)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return len(events), created, nil
}

func (s *Store) ClaimWebhookDeliveries(ctx context.Context, workerID string, now time.Time, limit int) ([]WebhookDelivery, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, fmt.Errorf("%w: webhook worker ID is required", ErrValidation)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: webhook delivery batch size must be positive", ErrValidation)
	}
	now = now.UTC()
	nowValue := now.Format(timeFormat)
	lockUntil := now.Add(webhookDeliveryLockDuration).Format(timeFormat)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE webhook_attempts
		SET status = 'suppressed',
		    locked_by = NULL,
		    locked_until = NULL,
		    completed_at = ?
		WHERE (
		    status IN ('queued','retrying')
		    OR (status = 'processing' AND locked_until <= ?)
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM webhook_endpoints endpoint
		    WHERE endpoint.project_id = webhook_attempts.project_id
		      AND endpoint.id = webhook_attempts.endpoint_id
		      AND endpoint.status = 'active'
		  )
	`, nowValue, nowValue); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `
		UPDATE webhook_attempts
		SET status = 'processing',
		    attempt_count = attempt_count + 1,
		    attempted_at = ?,
		    locked_by = ?,
		    locked_until = ?,
		    status_code = NULL,
		    error_category = NULL,
		    response_duration_ms = NULL,
		    last_error_safe_message = NULL
		WHERE id IN (
		  SELECT attempt.id
		  FROM webhook_attempts attempt
		  JOIN webhook_endpoints endpoint
		    ON endpoint.project_id = attempt.project_id
		   AND endpoint.id = attempt.endpoint_id
		  WHERE endpoint.status = 'active'
		    AND attempt.attempt_count < attempt.max_attempts
		    AND (
		      (attempt.status IN ('queued','retrying') AND attempt.next_attempt_at <= ?)
		      OR (attempt.status = 'processing' AND attempt.locked_until <= ?)
		    )
		  ORDER BY attempt.next_attempt_at, attempt.id
		  LIMIT ?
		)
		RETURNING id
	`, nowValue, workerID, lockUntil, nowValue, nowValue, limit)
	if err != nil {
		return nil, err
	}
	var deliveryIDs []string
	for rows.Next() {
		var deliveryID string
		if err := rows.Scan(&deliveryID); err != nil {
			rows.Close()
			return nil, err
		}
		deliveryIDs = append(deliveryIDs, deliveryID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	deliveries := make([]WebhookDelivery, 0, len(deliveryIDs))
	for _, deliveryID := range deliveryIDs {
		delivery, err := s.getWebhookDeliveryTx(ctx, tx, deliveryID, workerID)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (s *Store) MarkWebhookDeliverySucceeded(
	ctx context.Context,
	deliveryID, workerID string,
	now time.Time,
	statusCode int,
	durationMillis int64,
) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE webhook_attempts
		SET status = 'succeeded',
		    status_code = ?,
		    response_duration_ms = ?,
		    completed_at = ?,
		    locked_by = NULL,
		    locked_until = NULL
		WHERE id = ? AND status = 'processing' AND locked_by = ?
	`, statusCode, durationMillis, now.UTC().Format(timeFormat), deliveryID, workerID)
	if err != nil {
		return err
	}
	return requireOneRowAffected(result)
}

func (s *Store) MarkWebhookDeliveryFailed(
	ctx context.Context,
	delivery WebhookDelivery,
	workerID string,
	now time.Time,
	outcome WebhookDeliveryOutcome,
) error {
	status := "failed"
	nextAttemptAt := now.UTC()
	completedAt := now.UTC().Format(timeFormat)
	if outcome.Retryable && delivery.AttemptCount < delivery.MaxAttempts {
		status = "retrying"
		nextAttemptAt = now.UTC().Add(webhookRetryDelay(delivery.AttemptCount))
		completedAt = ""
	} else if outcome.Retryable {
		status = "dead_letter"
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE webhook_attempts
		SET status = ?,
		    status_code = ?,
		    error_category = ?,
		    response_duration_ms = ?,
		    last_error_safe_message = ?,
		    next_attempt_at = ?,
		    completed_at = ?,
		    locked_by = NULL,
		    locked_until = NULL
		WHERE id = ? AND status = 'processing' AND locked_by = ?
	`, status, nullInt(outcome.StatusCode), nullIfEmpty(outcome.ErrorCategory),
		nullInt64(outcome.ResponseDurationMillis), safeWebhookError(outcome.SafeMessage),
		nextAttemptAt.Format(timeFormat), nullIfEmpty(completedAt), delivery.ID, workerID)
	if err != nil {
		return err
	}
	return requireOneRowAffected(result)
}

func (s *Store) getWebhookDeliveryTx(ctx context.Context, tx *sql.Tx, deliveryID, workerID string) (WebhookDelivery, error) {
	var delivery WebhookDelivery
	var payloadJSON, secretCiphertext string
	err := tx.QueryRowContext(ctx, `
		SELECT attempt.id, attempt.project_id, attempt.endpoint_id, endpoint.url,
		       attempt.outbox_event_id, event.event_type, event.aggregate_type,
		       event.aggregate_id, event.payload_json, event.created_at,
		       attempt.attempt_count, attempt.max_attempts,
		       COALESCE(endpoint.secret_ciphertext, '')
		FROM webhook_attempts attempt
		JOIN webhook_endpoints endpoint
		  ON endpoint.project_id = attempt.project_id
		 AND endpoint.id = attempt.endpoint_id
		JOIN outbox_events event
		  ON event.project_id = attempt.project_id
		 AND event.id = attempt.outbox_event_id
		WHERE attempt.id = ?
		  AND attempt.status = 'processing'
		  AND attempt.locked_by = ?
	`, deliveryID, workerID).Scan(
		&delivery.ID,
		&delivery.ProjectID,
		&delivery.EndpointID,
		&delivery.EndpointURL,
		&delivery.OutboxEventID,
		&delivery.EventType,
		&delivery.AggregateType,
		&delivery.AggregateID,
		&payloadJSON,
		&delivery.EventCreatedAt,
		&delivery.AttemptCount,
		&delivery.MaxAttempts,
		&secretCiphertext,
	)
	if err != nil {
		return WebhookDelivery{}, err
	}
	if !json.Valid([]byte(payloadJSON)) {
		delivery.SigningSecretErrorSafe = "webhook event payload is invalid"
	} else {
		delivery.Payload = json.RawMessage(payloadJSON)
	}
	if secretCiphertext == "" {
		delivery.SigningSecretErrorSafe = "webhook endpoint must be recreated to provision its signing secret"
		return delivery, nil
	}
	signingSecret, err := security.DecryptSecret(s.webhookEncryptionKey, secretCiphertext)
	if err != nil {
		delivery.SigningSecretErrorSafe = "webhook signing secret is unavailable"
		return delivery, nil
	}
	delivery.SigningSecret = signingSecret
	return delivery, nil
}

func webhookRetryDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 15 * time.Second
	case attempt == 2:
		return time.Minute
	case attempt == 3:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func safeWebhookError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "webhook delivery failed"
	}
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
