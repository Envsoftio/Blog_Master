package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"seoblog/apps/backend/internal/security"
)

type AdminMediaAsset struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Status      string `json:"status"`
	Width       int64  `json:"width,omitempty"`
	Height      int64  `json:"height,omitempty"`
	Bytes       int64  `json:"bytes"`
	AltText     string `json:"altText,omitempty"`
	Decorative  bool   `json:"decorative"`
	Caption     string `json:"caption,omitempty"`
	Credit      string `json:"credit,omitempty"`
	License     string `json:"license,omitempty"`
	ObjectKey   string `json:"objectKey"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type MediaUploadInput struct {
	Filename    string
	ContentType string
	Bytes       int64
}

type MediaPatch struct {
	AltText    *string
	Decorative *bool
	Caption    *string
	Credit     *string
	License    *string
}

type AdminAIJob struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	ContentID string `json:"contentId,omitempty"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Error     string `json:"error,omitempty"`
}

type AIJobInput struct {
	Type      string
	ContentID string
}

type WebhookEndpoint struct {
	ID              string   `json:"id"`
	ProjectID       string   `json:"projectId"`
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Events          []string `json:"events"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
	LastDeliveredAt string   `json:"lastDeliveredAt,omitempty"`
}

type WebhookWithSecret struct {
	WebhookEndpoint
	Secret string `json:"secret"`
}

type WebhookAttempt struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	EndpointID    string `json:"endpointId"`
	EndpointName  string `json:"endpointName"`
	OutboxEventID string `json:"outboxEventId"`
	EventType     string `json:"eventType"`
	AggregateType string `json:"aggregateType"`
	AggregateID   string `json:"aggregateId"`
	Status        string `json:"status"`
	StatusCode    int64  `json:"statusCode,omitempty"`
	ErrorCategory string `json:"errorCategory,omitempty"`
	AttemptedAt   string `json:"attemptedAt"`
}

type WebhookInput struct {
	Name   string
	URL    string
	Events []string
}

type DeliveryStatus struct {
	Endpoints int64 `json:"endpoints"`
	Active    int64 `json:"active"`
	Pending   int64 `json:"pending"`
	Failures  int64 `json:"failures"`
	Succeeded int64 `json:"succeeded"`
}

var allowedWebhookEvents = map[string]struct{}{
	"content.published":    {},
	"content.updated":      {},
	"content.unpublished":  {},
	"content.restored":     {},
	"content.slug_changed": {},
	"content.archived":     {},
}

func (s *Store) ListMediaAssets(ctx context.Context, userID, projectID string) ([]AdminMediaAsset, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, filename, mime_type, object_key, byte_size,
		       COALESCE(width, 0), COALESCE(height, 0), COALESCE(alt_text, ''),
		       decorative, COALESCE(caption, ''), COALESCE(creator_credit, ''),
		       COALESCE(license, ''), created_at, updated_at
		FROM assets
		WHERE project_id = ?
		ORDER BY created_at DESC, id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := []AdminMediaAsset{}
	for rows.Next() {
		asset, err := scanAdminMediaAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (s *Store) GetMediaAsset(ctx context.Context, userID, projectID, assetID string) (AdminMediaAsset, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	return scanAdminMediaAsset(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, filename, mime_type, object_key, byte_size,
		       COALESCE(width, 0), COALESCE(height, 0), COALESCE(alt_text, ''),
		       decorative, COALESCE(caption, ''), COALESCE(creator_credit, ''),
		       COALESCE(license, ''), created_at, updated_at
		FROM assets
		WHERE project_id = ? AND id = ?
	`, projectID, assetID))
}

func (s *Store) CreateMediaAsset(ctx context.Context, userID, projectID string, input MediaUploadInput) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	input.Filename = strings.TrimSpace(filepath.Base(input.Filename))
	input.ContentType = strings.ToLower(strings.TrimSpace(input.ContentType))
	if input.Filename == "" || input.Filename == "." {
		return AdminMediaAsset{}, fmt.Errorf("%w: filename is required", ErrValidation)
	}
	if input.ContentType == "" || input.Bytes <= 0 {
		return AdminMediaAsset{}, fmt.Errorf("%w: contentType and a positive byte size are required", ErrValidation)
	}
	if input.Bytes > 100*1024*1024 {
		return AdminMediaAsset{}, fmt.Errorf("%w: media files may not exceed 100 MB", ErrValidation)
	}
	assetID, err := security.RandomID("asset")
	if err != nil {
		return AdminMediaAsset{}, err
	}
	objectKey := "pending/" + projectID + "/" + assetID + "/" + safeObjectFilename(input.Filename)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	defer tx.Rollback()
	if status, err := projectStatus(ctx, tx, projectID); err != nil {
		return AdminMediaAsset{}, err
	} else if status != "active" {
		return AdminMediaAsset{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO assets(id, project_id, object_key, filename, mime_type, byte_size, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, assetID, projectID, objectKey, input.Filename, input.ContentType, input.Bytes, userID); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "media.register", "asset", assetID, "success", map[string]any{
		"filename": input.Filename,
		"bytes":    input.Bytes,
	}); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.GetMediaAsset(ctx, userID, projectID, assetID)
}

func (s *Store) UpdateMediaAsset(ctx context.Context, userID, projectID, assetID string, patch MediaPatch) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	current, err := s.GetMediaAsset(ctx, userID, projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	altText, decorative, caption, credit, license := current.AltText, current.Decorative, current.Caption, current.Credit, current.License
	if patch.AltText != nil {
		altText = strings.TrimSpace(*patch.AltText)
	}
	if patch.Decorative != nil {
		decorative = *patch.Decorative
	}
	if patch.Caption != nil {
		caption = strings.TrimSpace(*patch.Caption)
	}
	if patch.Credit != nil {
		credit = strings.TrimSpace(*patch.Credit)
	}
	if patch.License != nil {
		license = strings.TrimSpace(*patch.License)
	}
	if decorative {
		altText = ""
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET alt_text = ?, decorative = ?, caption = ?, creator_credit = ?, license = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, nullIfEmpty(altText), boolToInt(decorative), nullIfEmpty(caption), nullIfEmpty(credit), nullIfEmpty(license), projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminMediaAsset{}, err
	} else if changed != 1 {
		return AdminMediaAsset{}, sql.ErrNoRows
	}
	return s.GetMediaAsset(ctx, userID, projectID, assetID)
}

func (s *Store) DeleteMediaAsset(ctx context.Context, userID, projectID, assetID string) error {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM assets WHERE project_id = ? AND id = ?`, projectID, assetID)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return err
	} else if changed != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListAIJobs(ctx context.Context, userID, projectID string) ([]AdminAIJob, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, COALESCE(content_id, ''), task_type, status,
		       started_at, COALESCE(completed_at, started_at), COALESCE(error_category, '')
		FROM ai_jobs
		WHERE project_id = ?
		ORDER BY started_at DESC, id DESC
		LIMIT 100
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []AdminAIJob{}
	for rows.Next() {
		var job AdminAIJob
		if err := rows.Scan(&job.ID, &job.ProjectID, &job.ContentID, &job.Type, &job.Status, &job.CreatedAt, &job.UpdatedAt, &job.Error); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) CreateAIJob(ctx context.Context, userID, projectID string, input AIJobInput) (AdminAIJob, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminAIJob{}, err
	}
	input.Type = strings.TrimSpace(input.Type)
	if input.Type != "outline" && input.Type != "draft" && input.Type != "quality_check" {
		return AdminAIJob{}, fmt.Errorf("%w: unsupported AI job type", ErrValidation)
	}
	if input.ContentID != "" {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_items WHERE project_id = ? AND id = ?`, projectID, input.ContentID).Scan(&count); err != nil {
			return AdminAIJob{}, err
		}
		if count != 1 {
			return AdminAIJob{}, fmt.Errorf("%w: article does not belong to this project", ErrValidation)
		}
	}
	jobID, err := security.RandomID("aijob")
	if err != nil {
		return AdminAIJob{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminAIJob{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ai_jobs(id, project_id, content_id, task_type, status, started_by)
		VALUES (?, ?, ?, ?, 'queued', ?)
	`, jobID, projectID, nullIfEmpty(input.ContentID), input.Type, userID); err != nil {
		return AdminAIJob{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "ai_job.create", "ai_job", jobID, "success", map[string]string{"type": input.Type}); err != nil {
		return AdminAIJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminAIJob{}, err
	}
	return s.GetAIJob(ctx, userID, projectID, jobID)
}

func (s *Store) GetAIJob(ctx context.Context, userID, projectID, jobID string) (AdminAIJob, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return AdminAIJob{}, err
	}
	var job AdminAIJob
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, COALESCE(content_id, ''), task_type, status,
		       started_at, COALESCE(completed_at, started_at), COALESCE(error_category, '')
		FROM ai_jobs
		WHERE project_id = ? AND id = ?
	`, projectID, jobID).Scan(&job.ID, &job.ProjectID, &job.ContentID, &job.Type, &job.Status, &job.CreatedAt, &job.UpdatedAt, &job.Error)
	return job, err
}

func (s *Store) CancelAIJob(ctx context.Context, userID, projectID, jobID string) (AdminAIJob, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminAIJob{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = 'cancelled', completed_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND status IN ('queued', 'running')
	`, projectID, jobID)
	if err != nil {
		return AdminAIJob{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminAIJob{}, err
	} else if changed != 1 {
		return AdminAIJob{}, fmt.Errorf("%w: AI job is not cancellable", ErrInvalidWorkflow)
	}
	return s.GetAIJob(ctx, userID, projectID, jobID)
}

func (s *Store) ListWebhooks(ctx context.Context, userID, projectID string) ([]WebhookEndpoint, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT endpoint.id, endpoint.project_id, endpoint.name, endpoint.url,
		       endpoint.events_json, endpoint.status, endpoint.created_at,
		       COALESCE((
		         SELECT MAX(attempt.attempted_at)
		         FROM webhook_attempts attempt
		         WHERE attempt.project_id = endpoint.project_id
		           AND attempt.endpoint_id = endpoint.id
		           AND attempt.status = 'succeeded'
		       ), '')
		FROM webhook_endpoints endpoint
		WHERE endpoint.project_id = ?
		ORDER BY endpoint.created_at DESC, endpoint.id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	endpoints := []WebhookEndpoint{}
	for rows.Next() {
		endpoint, err := scanWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, rows.Err()
}

func (s *Store) CreateWebhook(ctx context.Context, userID, projectID string, input WebhookInput) (WebhookWithSecret, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return WebhookWithSecret{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	if input.Name == "" || len(input.Name) > 120 {
		return WebhookWithSecret{}, fmt.Errorf("%w: webhook name is required", ErrValidation)
	}
	parsed, err := url.ParseRequestURI(input.URL)
	if err != nil || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1"))) {
		return WebhookWithSecret{}, fmt.Errorf("%w: webhook URL must use HTTPS", ErrValidation)
	}
	events := uniqueStrings(input.Events)
	if len(events) == 0 {
		return WebhookWithSecret{}, fmt.Errorf("%w: at least one webhook event is required", ErrValidation)
	}
	for _, event := range events {
		if _, ok := allowedWebhookEvents[event]; !ok {
			return WebhookWithSecret{}, fmt.Errorf("%w: unsupported webhook event %q", ErrValidation, event)
		}
	}
	eventsJSON, err := jsonString(events)
	if err != nil {
		return WebhookWithSecret{}, err
	}
	endpointID, err := security.RandomID("wh")
	if err != nil {
		return WebhookWithSecret{}, err
	}
	secret, err := security.RandomToken(32)
	if err != nil {
		return WebhookWithSecret{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_endpoints(id, project_id, name, url, secret_hash, events_json, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, endpointID, projectID, input.Name, input.URL, security.TokenHash(secret), eventsJSON, userID); err != nil {
		return WebhookWithSecret{}, err
	}
	endpoint, err := s.getWebhook(ctx, projectID, endpointID)
	if err != nil {
		return WebhookWithSecret{}, err
	}
	return WebhookWithSecret{WebhookEndpoint: endpoint, Secret: secret}, nil
}

func (s *Store) RevokeWebhook(ctx context.Context, userID, projectID, endpointID string) (WebhookEndpoint, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return WebhookEndpoint{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE webhook_endpoints
		SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND revoked_at IS NULL
	`, projectID, endpointID)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return WebhookEndpoint{}, err
	} else if changed != 1 {
		return WebhookEndpoint{}, sql.ErrNoRows
	}
	return s.getWebhook(ctx, projectID, endpointID)
}

func (s *Store) ListWebhookAttempts(ctx context.Context, userID, projectID, cursor string, limit int) ([]WebhookAttempt, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT attempt.id, attempt.project_id, attempt.endpoint_id, endpoint.name,
		       attempt.outbox_event_id, event.event_type, event.aggregate_type,
		       event.aggregate_id, attempt.status, COALESCE(attempt.status_code, 0),
		       COALESCE(attempt.error_category, ''), attempt.attempted_at
		FROM webhook_attempts attempt
		JOIN webhook_endpoints endpoint
		  ON endpoint.project_id = attempt.project_id
		 AND endpoint.id = attempt.endpoint_id
		JOIN outbox_events event
		  ON event.project_id = attempt.project_id
		 AND event.id = attempt.outbox_event_id
		WHERE attempt.project_id = ?
		  AND (
		    ? = ''
		    OR attempt.attempted_at < (
		      SELECT cursor_attempt.attempted_at
		      FROM webhook_attempts cursor_attempt
		      WHERE cursor_attempt.project_id = ? AND cursor_attempt.id = ?
		    )
		    OR (
		      attempt.attempted_at = (
		        SELECT cursor_attempt.attempted_at
		        FROM webhook_attempts cursor_attempt
		        WHERE cursor_attempt.project_id = ? AND cursor_attempt.id = ?
		      )
		      AND attempt.id < ?
		    )
		  )
		ORDER BY attempt.attempted_at DESC, attempt.id DESC
		LIMIT ?
	`, projectID, cursor, projectID, cursor, projectID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []WebhookAttempt{}
	for rows.Next() {
		attempt, err := scanWebhookAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func (s *Store) ReplayWebhookAttempt(ctx context.Context, userID, projectID, attemptID string) (WebhookAttempt, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return WebhookAttempt{}, err
	}
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return WebhookAttempt{}, fmt.Errorf("%w: attempt id is required", ErrValidation)
	}
	replayID, err := security.RandomID("wha")
	if err != nil {
		return WebhookAttempt{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WebhookAttempt{}, err
	}
	defer tx.Rollback()

	source, endpointStatus, err := getWebhookAttemptForReplay(ctx, tx, projectID, attemptID)
	if err != nil {
		return WebhookAttempt{}, err
	}
	if endpointStatus != "active" {
		return WebhookAttempt{}, fmt.Errorf("%w: webhook endpoint is not active", ErrInvalidWorkflow)
	}
	if source.Status != "failed" && source.Status != "dead_letter" {
		return WebhookAttempt{}, fmt.Errorf("%w: only failed webhook attempts can be replayed", ErrInvalidWorkflow)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE outbox_events
		SET processed_at = NULL,
		    available_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, projectID, source.OutboxEventID); err != nil {
		return WebhookAttempt{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO webhook_attempts(id, project_id, endpoint_id, outbox_event_id, status, error_category)
		VALUES (?, ?, ?, ?, 'queued', ?)
	`, replayID, projectID, source.EndpointID, source.OutboxEventID, "manual_replay:"+attemptID); err != nil {
		return WebhookAttempt{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "webhook_attempt.replay", "webhook_attempt", replayID, "success", map[string]string{
		"source_attempt_id": attemptID,
		"endpoint_id":       source.EndpointID,
		"outbox_event_id":   source.OutboxEventID,
	}); err != nil {
		return WebhookAttempt{}, err
	}
	if err := tx.Commit(); err != nil {
		return WebhookAttempt{}, err
	}
	return s.getWebhookAttempt(ctx, projectID, replayID)
}

func (s *Store) DeliveryStatus(ctx context.Context, userID, projectID string) (DeliveryStatus, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return DeliveryStatus{}, err
	}
	var status DeliveryStatus
	err := s.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM webhook_endpoints WHERE project_id = ?),
		  (SELECT COUNT(*) FROM webhook_endpoints WHERE project_id = ? AND status = 'active'),
		  (SELECT COUNT(*) FROM webhook_attempts WHERE project_id = ? AND status IN ('pending', 'queued', 'retrying')),
		  (SELECT COUNT(*) FROM webhook_attempts WHERE project_id = ? AND status IN ('failed', 'dead_letter')),
		  (SELECT COUNT(*) FROM webhook_attempts WHERE project_id = ? AND status = 'succeeded')
	`, projectID, projectID, projectID, projectID, projectID).Scan(
		&status.Endpoints,
		&status.Active,
		&status.Pending,
		&status.Failures,
		&status.Succeeded,
	)
	return status, err
}

func (s *Store) getWebhookAttempt(ctx context.Context, projectID, attemptID string) (WebhookAttempt, error) {
	return scanWebhookAttempt(s.db.QueryRowContext(ctx, `
		SELECT attempt.id, attempt.project_id, attempt.endpoint_id, endpoint.name,
		       attempt.outbox_event_id, event.event_type, event.aggregate_type,
		       event.aggregate_id, attempt.status, COALESCE(attempt.status_code, 0),
		       COALESCE(attempt.error_category, ''), attempt.attempted_at
		FROM webhook_attempts attempt
		JOIN webhook_endpoints endpoint
		  ON endpoint.project_id = attempt.project_id
		 AND endpoint.id = attempt.endpoint_id
		JOIN outbox_events event
		  ON event.project_id = attempt.project_id
		 AND event.id = attempt.outbox_event_id
		WHERE attempt.project_id = ? AND attempt.id = ?
	`, projectID, attemptID))
}

func (s *Store) getWebhook(ctx context.Context, projectID, endpointID string) (WebhookEndpoint, error) {
	return scanWebhookEndpoint(s.db.QueryRowContext(ctx, `
		SELECT endpoint.id, endpoint.project_id, endpoint.name, endpoint.url,
		       endpoint.events_json, endpoint.status, endpoint.created_at,
		       COALESCE((
		         SELECT MAX(attempt.attempted_at)
		         FROM webhook_attempts attempt
		         WHERE attempt.project_id = endpoint.project_id
		           AND attempt.endpoint_id = endpoint.id
		           AND attempt.status = 'succeeded'
		       ), '')
		FROM webhook_endpoints endpoint
		WHERE endpoint.project_id = ? AND endpoint.id = ?
	`, projectID, endpointID))
}

func scanAdminMediaAsset(row rowScanner) (AdminMediaAsset, error) {
	var asset AdminMediaAsset
	var decorative int
	err := row.Scan(
		&asset.ID,
		&asset.ProjectID,
		&asset.Filename,
		&asset.ContentType,
		&asset.ObjectKey,
		&asset.Bytes,
		&asset.Width,
		&asset.Height,
		&asset.AltText,
		&decorative,
		&asset.Caption,
		&asset.Credit,
		&asset.License,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	)
	asset.Decorative = decorative == 1
	asset.Status = "registered"
	return asset, err
}

func scanWebhookEndpoint(row rowScanner) (WebhookEndpoint, error) {
	var endpoint WebhookEndpoint
	var eventsJSON string
	err := row.Scan(
		&endpoint.ID,
		&endpoint.ProjectID,
		&endpoint.Name,
		&endpoint.URL,
		&eventsJSON,
		&endpoint.Status,
		&endpoint.CreatedAt,
		&endpoint.LastDeliveredAt,
	)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	decodeInto(eventsJSON, &endpoint.Events)
	if endpoint.Events == nil {
		endpoint.Events = []string{}
	}
	return endpoint, nil
}

func scanWebhookAttempt(row rowScanner) (WebhookAttempt, error) {
	var attempt WebhookAttempt
	err := row.Scan(
		&attempt.ID,
		&attempt.ProjectID,
		&attempt.EndpointID,
		&attempt.EndpointName,
		&attempt.OutboxEventID,
		&attempt.EventType,
		&attempt.AggregateType,
		&attempt.AggregateID,
		&attempt.Status,
		&attempt.StatusCode,
		&attempt.ErrorCategory,
		&attempt.AttemptedAt,
	)
	return attempt, err
}

func getWebhookAttemptForReplay(ctx context.Context, tx *sql.Tx, projectID, attemptID string) (WebhookAttempt, string, error) {
	var attempt WebhookAttempt
	var endpointStatus string
	err := tx.QueryRowContext(ctx, `
		SELECT attempt.id, attempt.project_id, attempt.endpoint_id, endpoint.name,
		       attempt.outbox_event_id, event.event_type, event.aggregate_type,
		       event.aggregate_id, attempt.status, COALESCE(attempt.status_code, 0),
		       COALESCE(attempt.error_category, ''), attempt.attempted_at,
		       endpoint.status
		FROM webhook_attempts attempt
		JOIN webhook_endpoints endpoint
		  ON endpoint.project_id = attempt.project_id
		 AND endpoint.id = attempt.endpoint_id
		JOIN outbox_events event
		  ON event.project_id = attempt.project_id
		 AND event.id = attempt.outbox_event_id
		WHERE attempt.project_id = ? AND attempt.id = ?
	`, projectID, attemptID).Scan(
		&attempt.ID,
		&attempt.ProjectID,
		&attempt.EndpointID,
		&attempt.EndpointName,
		&attempt.OutboxEventID,
		&attempt.EventType,
		&attempt.AggregateType,
		&attempt.AggregateID,
		&attempt.Status,
		&attempt.StatusCode,
		&attempt.ErrorCategory,
		&attempt.AttemptedAt,
		&endpointStatus,
	)
	return attempt, endpointStatus, err
}

func safeObjectFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character
		case character >= '0' && character <= '9':
			return character
		case character == '.', character == '-', character == '_':
			return character
		default:
			return '-'
		}
	}, value)
	value = strings.Trim(value, ".-_")
	if value == "" {
		return "asset"
	}
	return value
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
