package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"seoblog/apps/backend/internal/media"
	"seoblog/apps/backend/internal/security"
)

type AdminMediaAsset struct {
	ID             string              `json:"id"`
	ProjectID      string              `json:"projectId"`
	CreatedBy      string              `json:"-"`
	Filename       string              `json:"filename"`
	ContentType    string              `json:"contentType"`
	Status         string              `json:"status"`
	Width          int64               `json:"width,omitempty"`
	Height         int64               `json:"height,omitempty"`
	Bytes          int64               `json:"bytes"`
	AltText        string              `json:"altText,omitempty"`
	Decorative     bool                `json:"decorative"`
	Caption        string              `json:"caption,omitempty"`
	Credit         string              `json:"credit,omitempty"`
	License        string              `json:"license,omitempty"`
	ObjectKey      string              `json:"objectKey"`
	Bucket         string              `json:"bucket,omitempty"`
	SHA256         string              `json:"sha256,omitempty"`
	ScanStatus     string              `json:"scanStatus,omitempty"`
	ScanReason     string              `json:"scanReason,omitempty"`
	ExpectedSHA256 string              `json:"-"`
	Metadata       json.RawMessage     `json:"metadata,omitempty"`
	Variants       []AdminMediaVariant `json:"variants,omitempty"`
	Upload         *MediaUploadTarget  `json:"upload,omitempty"`
	URL            string              `json:"url,omitempty"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt"`
}

type AdminMediaVariant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	Width       int64  `json:"width,omitempty"`
	Height      int64  `json:"height,omitempty"`
	Bytes       int64  `json:"bytes"`
	URL         string `json:"url,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type MediaUploadTarget struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Fields    map[string]string `json:"fields,omitempty"`
	ExpiresAt string            `json:"expiresAt"`
	MaxBytes  int64             `json:"maxBytes"`
}

type MediaUploadInput struct {
	Filename        string
	ContentType     string
	Bytes           int64
	Bucket          string
	ObjectKeyPrefix string
	Status          string
	UploadExpiresAt time.Time
}

type MediaVariantInput struct {
	Name        string
	ObjectKey   string
	ContentType string
	Width       int64
	Height      int64
	Bytes       int64
}

type MediaCompletionInput struct {
	ObjectKey   string
	Bucket      string
	ContentType string
	SHA256      string
	Width       int64
	Height      int64
	Metadata    json.RawMessage
	Variants    []MediaVariantInput
}

type MediaPatch struct {
	AltText    *string
	Decorative *bool
	Caption    *string
	Credit     *string
	License    *string
}

type mediaUploadPolicy struct {
	Extensions map[string]struct{}
	MaxBytes   int64
}

var allowedMediaUploads = map[string]mediaUploadPolicy{
	"image/jpeg": {
		Extensions: extensionSet(".jpg", ".jpeg"),
		MaxBytes:   25 * 1024 * 1024,
	},
	"image/png": {
		Extensions: extensionSet(".png"),
		MaxBytes:   25 * 1024 * 1024,
	},
	"image/webp": {
		Extensions: extensionSet(".webp"),
		MaxBytes:   25 * 1024 * 1024,
	},
	"image/gif": {
		Extensions: extensionSet(".gif"),
		MaxBytes:   25 * 1024 * 1024,
	},
	"application/pdf": {
		Extensions: extensionSet(".pdf"),
		MaxBytes:   50 * 1024 * 1024,
	},
}

type AdminAIJob struct {
	ID                    string          `json:"id"`
	ProjectID             string          `json:"projectId"`
	ContentID             string          `json:"contentId,omitempty"`
	RevisionID            string          `json:"revisionId,omitempty"`
	Type                  string          `json:"type"`
	ArticleType           string          `json:"articleType,omitempty"`
	Status                string          `json:"status"`
	PromptTemplateVersion string          `json:"promptTemplateVersion,omitempty"`
	VoiceProfileID        string          `json:"voiceProfileId,omitempty"`
	VoiceProfileVersion   int64           `json:"voiceProfileVersion,omitempty"`
	EvidencePacketID      string          `json:"evidencePacketId,omitempty"`
	EvidencePacketVersion int64           `json:"evidencePacketVersion,omitempty"`
	InputHash             string          `json:"inputHash,omitempty"`
	SourceRevisionHash    string          `json:"sourceRevisionHash,omitempty"`
	CreatedAt             string          `json:"createdAt"`
	UpdatedAt             string          `json:"updatedAt"`
	Result                json.RawMessage `json:"result,omitempty"`
	Error                 string          `json:"error,omitempty"`
	Reused                bool            `json:"reused,omitempty"`
}

type AIJobBrief struct {
	Title       string `json:"title"`
	Purpose     string `json:"purpose"`
	Audience    string `json:"audience"`
	UniqueAngle string `json:"uniqueAngle"`
	Evidence    string `json:"evidence"`
	CTA         string `json:"cta"`
}

type AIJobInput struct {
	Type                string
	ContentID           string
	ArticleType         string
	EvidencePacketID    string
	VoiceProfileVersion int64
	Brief               AIJobBrief
}

type AIJobProgressInput struct {
	Type          string
	Status        string
	Progress      int64
	Message       string
	Metadata      any
	ErrorCategory string
}

type AIJobEvent struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"projectId"`
	JobID     string          `json:"jobId"`
	Sequence  int64           `json:"sequence"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	Progress  int64           `json:"progress"`
	Message   string          `json:"message,omitempty"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt string          `json:"createdAt"`
}

type AIRun struct {
	ID                    string   `json:"id"`
	ProjectID             string   `json:"projectId"`
	ContentID             string   `json:"contentId,omitempty"`
	RevisionID            string   `json:"revisionId,omitempty"`
	JobID                 string   `json:"jobId,omitempty"`
	Type                  string   `json:"type"`
	Provider              string   `json:"provider"`
	ModelIdentifier       string   `json:"modelIdentifier"`
	PromptTemplateVersion string   `json:"promptTemplateVersion"`
	VoiceProfileVersion   int64    `json:"voiceProfileVersion,omitempty"`
	EvidencePacketVersion int64    `json:"evidencePacketVersion,omitempty"`
	InputHash             string   `json:"inputHash"`
	OutputHash            string   `json:"outputHash,omitempty"`
	SourceIDs             []string `json:"sourceIds"`
	StartedBy             string   `json:"startedBy"`
	StartedAt             string   `json:"startedAt"`
	CompletedAt           string   `json:"completedAt,omitempty"`
	Status                string   `json:"status"`
	InputTokens           int64    `json:"inputTokens,omitempty"`
	OutputTokens          int64    `json:"outputTokens,omitempty"`
	EstimatedCostCents    int64    `json:"estimatedCostCents,omitempty"`
	ErrorCategory         string   `json:"errorCategory,omitempty"`
}

type AIRunFilter struct {
	ContentID  string
	RevisionID string
	JobID      string
	Status     string
}

type QualityCheckResult struct {
	ID             string          `json:"id"`
	ProjectID      string          `json:"projectId"`
	ContentID      string          `json:"contentId,omitempty"`
	RevisionID     string          `json:"revisionId,omitempty"`
	CheckType      string          `json:"checkType"`
	Severity       string          `json:"severity"`
	Status         string          `json:"status"`
	Message        string          `json:"message"`
	Evidence       json.RawMessage `json:"evidence"`
	OverrideReason string          `json:"overrideReason,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

type QualityCheckFilter struct {
	ContentID  string
	RevisionID string
	Severity   string
	Status     string
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
	ID                     string `json:"id"`
	ProjectID              string `json:"projectId"`
	EndpointID             string `json:"endpointId"`
	EndpointName           string `json:"endpointName"`
	OutboxEventID          string `json:"outboxEventId"`
	EventType              string `json:"eventType"`
	AggregateType          string `json:"aggregateType"`
	AggregateID            string `json:"aggregateId"`
	Status                 string `json:"status"`
	StatusCode             int64  `json:"statusCode,omitempty"`
	ErrorCategory          string `json:"errorCategory,omitempty"`
	AttemptCount           int64  `json:"attemptCount"`
	MaxAttempts            int64  `json:"maxAttempts"`
	NextAttemptAt          string `json:"nextAttemptAt,omitempty"`
	ResponseDurationMillis int64  `json:"responseDurationMillis,omitempty"`
	LastErrorSafeMessage   string `json:"lastErrorSafeMessage,omitempty"`
	CompletedAt            string `json:"completedAt,omitempty"`
	ReplayOfAttemptID      string `json:"replayOfAttemptId,omitempty"`
	AttemptedAt            string `json:"attemptedAt"`
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

var allowedAIStatuses = map[string]struct{}{
	"queued":         {},
	"running":        {},
	"needs_input":    {},
	"succeeded":      {},
	"failed":         {},
	"cancelled":      {},
	"budget_blocked": {},
	"safety_blocked": {},
}

var allowedQualitySeverities = map[string]struct{}{
	"info":     {},
	"warning":  {},
	"blocking": {},
	"critical": {},
}

var allowedQualityStatuses = map[string]struct{}{
	"passed":     {},
	"failed":     {},
	"overridden": {},
}

var allowedMediaStatuses = map[string]struct{}{
	"registered": {},
	"uploading":  {},
	"processing": {},
	"ready":      {},
	"rejected":   {},
	"failed":     {},
}

const adminMediaAssetSelectColumns = `
	id, project_id, filename, mime_type, object_key, COALESCE(bucket, ''),
	byte_size, COALESCE(width, 0), COALESCE(height, 0), COALESCE(alt_text, ''),
	decorative, COALESCE(caption, ''), COALESCE(creator_credit, ''),
	COALESCE(license, ''), status, COALESCE(checksum_sha256, ''),
	COALESCE(expected_checksum_sha256, ''), scan_status, COALESCE(scan_reason, ''),
	provenance_json, created_by, created_at, updated_at
`

func (s *Store) ListMediaAssets(ctx context.Context, userID, projectID string) ([]AdminMediaAsset, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+adminMediaAssetSelectColumns+`
		FROM assets
		WHERE project_id = ?
		ORDER BY created_at DESC, id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}

	assets := []AdminMediaAsset{}
	for rows.Next() {
		asset, err := scanAdminMediaAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range assets {
		variants, err := s.listMediaVariants(ctx, projectID, assets[index].ID)
		if err != nil {
			return nil, err
		}
		assets[index].Variants = variants
	}
	return assets, nil
}

func (s *Store) GetMediaAsset(ctx context.Context, userID, projectID, assetID string) (AdminMediaAsset, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.getMediaAsset(ctx, projectID, assetID)
}

func (s *Store) getMediaAsset(ctx context.Context, projectID, assetID string) (AdminMediaAsset, error) {
	asset, err := scanAdminMediaAsset(s.db.QueryRowContext(ctx, `SELECT `+adminMediaAssetSelectColumns+`
		FROM assets
		WHERE project_id = ? AND id = ?
	`, projectID, assetID))
	if err != nil {
		return AdminMediaAsset{}, err
	}
	asset.Variants, err = s.listMediaVariants(ctx, projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	return asset, nil
}

func (s *Store) ListMediaAssetsForProcessing(ctx context.Context, limit int) ([]AdminMediaAsset, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+adminMediaAssetSelectColumns+`
		FROM assets
		WHERE status = 'processing'
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	var assets []AdminMediaAsset
	for rows.Next() {
		asset, err := scanAdminMediaAsset(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range assets {
		variants, err := s.listMediaVariants(ctx, assets[index].ProjectID, assets[index].ID)
		if err != nil {
			return nil, err
		}
		assets[index].Variants = variants
	}
	return assets, nil
}

func (s *Store) ListReadyMediaAssetsWithPendingOriginals(ctx context.Context, limit int) ([]AdminMediaAsset, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+adminMediaAssetSelectColumns+`
		FROM assets
		WHERE status = 'ready'
		  AND object_key LIKE ?
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, media.ObjectRootPrefix+"/pending/%", limit)
	if err != nil {
		return nil, err
	}
	var assets []AdminMediaAsset
	for rows.Next() {
		asset, err := scanAdminMediaAsset(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range assets {
		variants, err := s.listMediaVariants(ctx, assets[index].ProjectID, assets[index].ID)
		if err != nil {
			return nil, err
		}
		assets[index].Variants = variants
	}
	return assets, nil
}

func (s *Store) SetMediaAssetObjectKeySystem(ctx context.Context, projectID, assetID, objectKey string) (AdminMediaAsset, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return AdminMediaAsset{}, fmt.Errorf("%w: media object key is required", ErrValidation)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET object_key = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, objectKey, projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminMediaAsset{}, err
	} else if changed != 1 {
		return AdminMediaAsset{}, sql.ErrNoRows
	}
	return s.getMediaAsset(ctx, projectID, assetID)
}

func (s *Store) CreateMediaAsset(ctx context.Context, userID, projectID string, input MediaUploadInput) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	input, err := validateMediaUpload(input)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	assetID, err := security.RandomID("asset")
	if err != nil {
		return AdminMediaAsset{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "registered"
	}
	if _, ok := allowedMediaStatuses[status]; !ok {
		return AdminMediaAsset{}, fmt.Errorf("%w: unsupported media status", ErrValidation)
	}
	objectKeyPrefix := strings.Trim(input.ObjectKeyPrefix, "/")
	if objectKeyPrefix == "" {
		objectKeyPrefix = media.OriginalObjectKeyPrefix(projectID, assetID)
	}
	objectKey := objectKeyPrefix + "/" + safeObjectFilename(input.Filename)
	var uploadExpiresAt any
	if !input.UploadExpiresAt.IsZero() {
		uploadExpiresAt = input.UploadExpiresAt.UTC().Format(time.RFC3339)
	}

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
		INSERT INTO assets(
			id, project_id, storage_provider, bucket, object_key, filename, mime_type,
			byte_size, status, upload_expires_at, scan_status, created_by
		) VALUES (?, ?, 'b2', ?, ?, ?, ?, ?, ?, ?, 'pending', ?)
	`, assetID, projectID, nullIfEmpty(input.Bucket), objectKey, input.Filename, input.ContentType, input.Bytes, status, uploadExpiresAt, userID); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "media.register", "asset", assetID, "success", map[string]any{
		"filename": input.Filename,
		"bytes":    input.Bytes,
		"status":   status,
	}); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.getMediaAsset(ctx, projectID, assetID)
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

// AssertMediaAssetDeletable checks every currently supported asset reference
// before an object-storage delete is attempted. This is deliberately separate
// from DeleteMediaAsset because B2 and SQLite cannot share a transaction: the
// HTTP layer must run this preflight before removing any stored objects.
func (s *Store) AssertMediaAssetDeletable(ctx context.Context, userID, projectID, assetID string) error {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return err
	}
	if _, err := s.getMediaAsset(ctx, projectID, assetID); err != nil {
		return err
	}

	var authorName string
	err := s.db.QueryRowContext(ctx, `
		SELECT display_name
		FROM authors
		WHERE project_id = ? AND photo_asset_id = ?
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`, projectID, assetID).Scan(&authorName)
	if err == nil {
		return fmt.Errorf("%w: media asset is used by author %q; choose a replacement before deleting it", ErrInvalidWorkflow, authorName)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var projectReference string
	err = s.db.QueryRowContext(ctx, `
		SELECT CASE
			WHEN publisher_logo_asset_id = ? THEN 'publisher logo'
			WHEN default_social_image_id = ? THEN 'default social image'
		END
		FROM projects
		WHERE id = ?
		  AND (publisher_logo_asset_id = ? OR default_social_image_id = ?)
	`, assetID, assetID, projectID, assetID, assetID).Scan(&projectReference)
	if err == nil {
		return fmt.Errorf("%w: media asset is used as the project %s; choose a replacement before deleting it", ErrInvalidWorkflow, projectReference)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, body_document_json, media_snapshot_json
		FROM content_revisions
		WHERE project_id = ?
		  AND (instr(body_document_json, ?) > 0 OR instr(media_snapshot_json, ?) > 0)
		ORDER BY created_at ASC, id ASC
	`, projectID, assetID, assetID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var revisionID, bodyJSON, mediaJSON string
		if err := rows.Scan(&revisionID, &bodyJSON, &mediaJSON); err != nil {
			return err
		}
		if structuredDocumentReferencesAsset(bodyJSON, assetID) || jsonContainsString(mediaJSON, assetID) {
			return fmt.Errorf("%w: media asset is retained by revision %q; replace the reference with a new revision before deleting it", ErrInvalidWorkflow, revisionID)
		}
	}
	return rows.Err()
}

func structuredDocumentReferencesAsset(raw, assetID string) bool {
	var document any
	if json.Unmarshal([]byte(raw), &document) != nil {
		return false
	}
	return findAssetReference(document, assetID)
}

func findAssetReference(value any, assetID string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalizedKey := strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(key))
			switch normalizedKey {
			case "assetid", "assetids", "mediaid", "mediaids", "imageassetid", "heroassetid":
				if jsonValueContainsString(child, assetID) {
					return true
				}
			}
			if findAssetReference(child, assetID) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if findAssetReference(child, assetID) {
				return true
			}
		}
	}
	return false
}

func jsonContainsString(raw, expected string) bool {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return false
	}
	return jsonValueContainsString(value, expected)
}

func jsonValueContainsString(value any, expected string) bool {
	switch typed := value.(type) {
	case string:
		return typed == expected
	case map[string]any:
		for _, child := range typed {
			if jsonValueContainsString(child, expected) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if jsonValueContainsString(child, expected) {
				return true
			}
		}
	}
	return false
}

func (s *Store) MarkMediaAssetProcessing(ctx context.Context, userID, projectID, assetID, expectedSHA256 string) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	expectedSHA256 = normalizeSHA256(expectedSHA256)
	result, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET status = 'processing',
		    expected_checksum_sha256 = NULLIF(?, ''),
		    scan_status = 'pending',
		    scan_reason = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND status IN ('registered','uploading','failed')
	`, expectedSHA256, projectID, assetID)
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

func (s *Store) CompleteMediaAsset(ctx context.Context, userID, projectID, assetID string, input MediaCompletionInput) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.completeMediaAsset(ctx, projectID, assetID, input, "user", userID)
}

func (s *Store) CompleteMediaAssetSystem(ctx context.Context, projectID, assetID string, input MediaCompletionInput) (AdminMediaAsset, error) {
	return s.completeMediaAsset(ctx, projectID, assetID, input, "system", "")
}

func (s *Store) completeMediaAsset(ctx context.Context, projectID, assetID string, input MediaCompletionInput, actorType, actorID string) (AdminMediaAsset, error) {
	metadataJSON, err := mediaMetadataJSON(input.Metadata)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM asset_variants
		WHERE project_id = ? AND asset_id = ?
	`, projectID, assetID); err != nil {
		return AdminMediaAsset{}, err
	}
	for _, variant := range input.Variants {
		variant.Name = strings.TrimSpace(variant.Name)
		variant.ObjectKey = strings.TrimSpace(variant.ObjectKey)
		variant.ContentType = normalizeMediaContentType(variant.ContentType)
		if variant.Name == "" || variant.ObjectKey == "" || variant.ContentType == "" || variant.Bytes <= 0 {
			return AdminMediaAsset{}, fmt.Errorf("%w: media variant is incomplete", ErrValidation)
		}
		variantID, err := security.RandomID("variant")
		if err != nil {
			return AdminMediaAsset{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO asset_variants(
				id, project_id, asset_id, variant_name, object_key,
				mime_type, width, height, byte_size
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, variantID, projectID, assetID, variant.Name, variant.ObjectKey, variant.ContentType, nullZeroInt(variant.Width), nullZeroInt(variant.Height), variant.Bytes); err != nil {
			return AdminMediaAsset{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE assets
		SET status = 'ready',
		    object_key = COALESCE(NULLIF(?, ''), object_key),
		    bucket = COALESCE(NULLIF(?, ''), bucket),
		    mime_type = COALESCE(NULLIF(?, ''), mime_type),
		    width = ?,
		    height = ?,
		    checksum_sha256 = NULLIF(?, ''),
		    scan_status = 'passed',
		    scan_reason = NULL,
		    provenance_json = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND status IN ('registered','uploading','processing','failed')
	`, input.ObjectKey, input.Bucket, input.ContentType, nullZeroInt(input.Width), nullZeroInt(input.Height), input.SHA256, metadataJSON, projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminMediaAsset{}, err
	} else if changed != 1 {
		return AdminMediaAsset{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, actorType, actorID, "media.ready", "asset", assetID, "success", map[string]any{
		"variantCount": len(input.Variants),
		"sha256":       input.SHA256,
	}); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.getMediaAsset(ctx, projectID, assetID)
}

func (s *Store) RejectMediaAsset(ctx context.Context, userID, projectID, assetID, reason string) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.rejectMediaAsset(ctx, projectID, assetID, reason, "user", userID)
}

func (s *Store) RejectMediaAssetSystem(ctx context.Context, projectID, assetID, reason string) (AdminMediaAsset, error) {
	return s.rejectMediaAsset(ctx, projectID, assetID, reason, "system", "")
}

func (s *Store) rejectMediaAsset(ctx context.Context, projectID, assetID, reason, actorType, actorID string) (AdminMediaAsset, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "media processing failed"
	}
	if len(reason) > 500 {
		reason = reason[:500]
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE assets
		SET status = 'rejected', scan_status = 'failed', scan_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, reason, projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminMediaAsset{}, err
	} else if changed != 1 {
		return AdminMediaAsset{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, actorType, actorID, "media.reject", "asset", assetID, "success", map[string]any{
		"reason": reason,
	}); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.getMediaAsset(ctx, projectID, assetID)
}

func (s *Store) FailMediaAsset(ctx context.Context, userID, projectID, assetID, reason string) (AdminMediaAsset, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.failMediaAsset(ctx, projectID, assetID, reason, "user", userID)
}

func (s *Store) FailMediaAssetSystem(ctx context.Context, projectID, assetID, reason string) (AdminMediaAsset, error) {
	return s.failMediaAsset(ctx, projectID, assetID, reason, "system", "")
}

func (s *Store) failMediaAsset(ctx context.Context, projectID, assetID, reason, actorType, actorID string) (AdminMediaAsset, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "media processing failed"
	}
	if len(reason) > 500 {
		reason = reason[:500]
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE assets
		SET status = 'failed', scan_status = 'skipped', scan_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, reason, projectID, assetID)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminMediaAsset{}, err
	} else if changed != 1 {
		return AdminMediaAsset{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, actorType, actorID, "media.fail", "asset", assetID, "error", map[string]any{
		"reason": reason,
	}); err != nil {
		return AdminMediaAsset{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminMediaAsset{}, err
	}
	return s.getMediaAsset(ctx, projectID, assetID)
}

func (s *Store) ListAIJobs(ctx context.Context, userID, projectID string) ([]AdminAIJob, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+adminAIJobSelectColumns+`
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
		job, err := scanAdminAIJob(rows)
		if err != nil {
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
	input = normalizeAIJobInput(input)
	if input.Type != "outline" && input.Type != "draft" && input.Type != "quality_check" {
		return AdminAIJob{}, fmt.Errorf("%w: unsupported AI job type", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminAIJob{}, err
	}
	defer tx.Rollback()
	if status, err := projectStatus(ctx, tx, projectID); err != nil {
		return AdminAIJob{}, err
	} else if status != "active" {
		return AdminAIJob{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if err := validateAIJobInput(input); err != nil {
		return AdminAIJob{}, err
	}

	var articleType, revisionID, revisionHash string
	if err := tx.QueryRowContext(ctx, `
		SELECT content.article_type, revision.id, revision.content_hash
		FROM content_items content
		JOIN content_revisions revision
		  ON revision.project_id = content.project_id
		 AND revision.content_id = content.id
		WHERE content.project_id = ? AND content.id = ?
		ORDER BY revision.revision_number DESC
		LIMIT 1
	`, projectID, input.ContentID).Scan(&articleType, &revisionID, &revisionHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminAIJob{}, fmt.Errorf("%w: article does not belong to this project", ErrValidation)
		}
		return AdminAIJob{}, err
	}
	if input.ArticleType != "" && input.ArticleType != articleType {
		return AdminAIJob{}, fmt.Errorf("%w: articleType does not match the selected article", ErrValidation)
	}

	var voiceProfile VoiceProfile
	if input.VoiceProfileVersion > 0 {
		voiceProfile, err = scanVoiceProfile(tx.QueryRowContext(ctx, `
			SELECT id, project_id, version, profile_json, created_by, created_at
			FROM voice_profiles
			WHERE project_id = ? AND version = ?
		`, projectID, input.VoiceProfileVersion))
	} else {
		voiceProfile, err = scanVoiceProfile(tx.QueryRowContext(ctx, `
			SELECT id, project_id, version, profile_json, created_by, created_at
			FROM voice_profiles
			WHERE project_id = ?
			ORDER BY version DESC
			LIMIT 1
		`, projectID))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return AdminAIJob{}, fmt.Errorf("%w: a project voice profile is required before creating an AI job", ErrInvalidWorkflow)
	}
	if err != nil {
		return AdminAIJob{}, err
	}
	voiceProfile.Profile = normalizeVoiceProfile(voiceProfile.Profile)
	if err := validateVoiceProfile(voiceProfile.Profile); err != nil {
		return AdminAIJob{}, err
	}

	evidencePacket, err := scanEvidencePacket(tx.QueryRowContext(ctx, `
		SELECT id, project_id, COALESCE(content_id, ''), version, packet_json,
		       COALESCE(approved_by, ''), COALESCE(approved_at, ''),
		       created_by, created_at
		FROM evidence_packets
		WHERE project_id = ? AND id = ?
	`, projectID, input.EvidencePacketID))
	if errors.Is(err, sql.ErrNoRows) {
		return AdminAIJob{}, fmt.Errorf("%w: evidence packet does not belong to this project", ErrValidation)
	}
	if err != nil {
		return AdminAIJob{}, err
	}
	if evidencePacket.ContentID != input.ContentID {
		return AdminAIJob{}, fmt.Errorf("%w: evidence packet does not belong to the selected article", ErrValidation)
	}
	evidencePacket.Packet = normalizeEvidencePacket(evidencePacket.Packet)
	if err := validateEvidencePacket(evidencePacket.Packet); err != nil {
		return AdminAIJob{}, err
	}
	if evidencePacket.ApprovedAt == "" || evidencePacket.Packet.PublicationRecommendation != "ready" {
		return AdminAIJob{}, fmt.Errorf("%w: an approved, ready evidence packet is required before creating an AI job", ErrInvalidWorkflow)
	}
	if err := ensureEvidenceSources(ctx, tx, projectID, evidenceSourceIDs(evidencePacket.Packet)); err != nil {
		return AdminAIJob{}, err
	}

	promptTemplateVersion := aiPromptTemplateVersion(input.Type)
	inputJSON, inputHash, err := encodeAIJobSnapshot(AIJobInputSnapshot{
		SchemaVersion:         "ai-job-input-v1",
		TaskType:              input.Type,
		ArticleType:           articleType,
		PromptTemplateVersion: promptTemplateVersion,
		Content: AIJobContentContext{
			ID:           input.ContentID,
			RevisionID:   revisionID,
			RevisionHash: revisionHash,
		},
		Brief: input.Brief,
		Voice: AIJobVoiceContext{
			ID:      voiceProfile.ID,
			Version: voiceProfile.Version,
			Profile: voiceProfile.Profile,
		},
		Evidence: AIJobEvidenceContext{
			ID:      evidencePacket.ID,
			Version: evidencePacket.Version,
			Packet:  evidencePacket.Packet,
		},
	})
	if err != nil {
		return AdminAIJob{}, err
	}
	if len(inputJSON) > 512*1024 {
		return AdminAIJob{}, fmt.Errorf("%w: AI job input snapshot exceeds 512 KB", ErrValidation)
	}

	existing, err := findReusableAIJob(ctx, tx, projectID, inputHash)
	if err == nil {
		if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "ai_job.reuse", "ai_job", existing.ID, "success", map[string]any{
			"input_hash": inputHash,
			"status":     existing.Status,
		}); err != nil {
			return AdminAIJob{}, err
		}
		if err := tx.Commit(); err != nil {
			return AdminAIJob{}, err
		}
		existing.Reused = true
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return AdminAIJob{}, err
	}

	jobID, err := security.RandomID("aijob")
	if err != nil {
		return AdminAIJob{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ai_jobs(
		  id, project_id, content_id, revision_id, task_type, article_type, status,
		  prompt_template_version, voice_profile_id, voice_profile_version,
		  evidence_packet_id, evidence_packet_version, input_hash, input_json,
		  source_revision_hash, started_by
		) VALUES (?, ?, ?, ?, ?, ?, 'queued', ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, jobID, projectID, input.ContentID, revisionID, input.Type, articleType,
		promptTemplateVersion, voiceProfile.ID, voiceProfile.Version,
		evidencePacket.ID, evidencePacket.Version, inputHash, inputJSON,
		revisionHash, userID); err != nil {
		insertErr := err
		_ = tx.Rollback()
		existing, reuseErr := findReusableAIJob(ctx, s.db, projectID, inputHash)
		if reuseErr == nil {
			if auditErr := s.recordAIJobReuse(ctx, projectID, userID, existing); auditErr != nil {
				return AdminAIJob{}, auditErr
			}
			existing.Reused = true
			return existing, nil
		}
		return AdminAIJob{}, insertErr
	}
	if err := appendAIJobEventTx(ctx, tx, projectID, jobID, "queued", "queued", 0, "AI job queued", map[string]any{
		"inputHash":             inputHash,
		"revisionId":            revisionID,
		"voiceProfileVersion":   voiceProfile.Version,
		"evidencePacketVersion": evidencePacket.Version,
	}); err != nil {
		return AdminAIJob{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "ai_job.create", "ai_job", jobID, "success", map[string]any{
		"type":                    input.Type,
		"input_hash":              inputHash,
		"revision_id":             revisionID,
		"voice_profile_version":   voiceProfile.Version,
		"evidence_packet_version": evidencePacket.Version,
	}); err != nil {
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
	return scanAdminAIJob(s.db.QueryRowContext(ctx, `SELECT `+adminAIJobSelectColumns+`
		FROM ai_jobs
		WHERE project_id = ? AND id = ?
	`, projectID, jobID))
}

func (s *Store) CancelAIJob(ctx context.Context, userID, projectID, jobID string) (AdminAIJob, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return AdminAIJob{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminAIJob{}, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM ai_jobs
		WHERE project_id = ? AND id = ?
	`, projectID, jobID).Scan(&status); err != nil {
		return AdminAIJob{}, err
	}
	if status != "queued" && status != "running" {
		return AdminAIJob{}, fmt.Errorf("%w: AI job is not cancellable", ErrInvalidWorkflow)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = 'cancelled', completed_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, projectID, jobID); err != nil {
		return AdminAIJob{}, err
	}
	if err := appendAIJobEventTx(ctx, tx, projectID, jobID, "cancelled", "cancelled", 0, "AI job cancelled", nil); err != nil {
		return AdminAIJob{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "ai_job.cancel", "ai_job", jobID, "success", nil); err != nil {
		return AdminAIJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminAIJob{}, err
	}
	return s.GetAIJob(ctx, userID, projectID, jobID)
}

func (s *Store) ListAIJobEvents(ctx context.Context, userID, projectID, jobID string, after int64, limit int) ([]AIJobEvent, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if after < 0 {
		return nil, fmt.Errorf("%w: event cursor cannot be negative", ErrValidation)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: event limit must be positive", ErrValidation)
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM ai_jobs
		WHERE project_id = ? AND id = ?
	`, projectID, jobID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists != 1 {
		return nil, sql.ErrNoRows
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, job_id, sequence, event_type, status,
		       progress, message, metadata_json, created_at
		FROM ai_job_events
		WHERE project_id = ? AND job_id = ? AND sequence > ?
		ORDER BY sequence ASC
		LIMIT ?
	`, projectID, jobID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []AIJobEvent{}
	for rows.Next() {
		event, err := scanAIJobEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) RecordAIJobProgress(ctx context.Context, projectID, jobID string, input AIJobProgressInput) (AIJobEvent, error) {
	projectID = strings.TrimSpace(projectID)
	jobID = strings.TrimSpace(jobID)
	input.Type = strings.TrimSpace(input.Type)
	input.Status = strings.TrimSpace(input.Status)
	input.Message = strings.TrimSpace(input.Message)
	input.ErrorCategory = strings.TrimSpace(input.ErrorCategory)
	if projectID == "" || jobID == "" {
		return AIJobEvent{}, fmt.Errorf("%w: project and AI job IDs are required", ErrValidation)
	}
	if input.Type == "" {
		input.Type = "progress"
	}
	if len(input.Type) > 80 || len(input.Message) > 2000 || len(input.ErrorCategory) > 120 {
		return AIJobEvent{}, fmt.Errorf("%w: AI job progress fields exceed their size limits", ErrValidation)
	}
	if _, ok := allowedAIStatuses[input.Status]; !ok {
		return AIJobEvent{}, fmt.Errorf("%w: unsupported AI job status", ErrValidation)
	}
	if input.Progress < 0 || input.Progress > 100 {
		return AIJobEvent{}, fmt.Errorf("%w: AI job progress must be between 0 and 100", ErrValidation)
	}
	if input.Status == "succeeded" && input.Progress != 100 {
		return AIJobEvent{}, fmt.Errorf("%w: succeeded AI jobs must report 100 percent progress", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AIJobEvent{}, err
	}
	defer tx.Rollback()

	var currentStatus, currentProjectStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT job.status, project.status
		FROM ai_jobs job
		JOIN projects project ON project.id = job.project_id
		WHERE job.project_id = ? AND job.id = ?
	`, projectID, jobID).Scan(&currentStatus, &currentProjectStatus); err != nil {
		return AIJobEvent{}, err
	}
	if currentProjectStatus != "active" && !isTerminalAIJobStatus(input.Status) {
		return AIJobEvent{}, fmt.Errorf("%w: inactive projects may only terminate AI jobs", ErrInvalidWorkflow)
	}
	if !canTransitionAIJob(currentStatus, input.Status) {
		return AIJobEvent{}, fmt.Errorf("%w: AI job cannot transition from %s to %s", ErrInvalidWorkflow, currentStatus, input.Status)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = ?,
		    completed_at = CASE
		      WHEN ? IN ('succeeded', 'failed', 'cancelled', 'budget_blocked', 'safety_blocked')
		      THEN CURRENT_TIMESTAMP
		      ELSE NULL
		    END,
		    error_category = ?
		WHERE project_id = ? AND id = ?
	`, input.Status, input.Status, nullIfEmpty(input.ErrorCategory), projectID, jobID); err != nil {
		return AIJobEvent{}, err
	}
	if err := appendAIJobEventTx(
		ctx,
		tx,
		projectID,
		jobID,
		input.Type,
		input.Status,
		input.Progress,
		input.Message,
		input.Metadata,
	); err != nil {
		return AIJobEvent{}, err
	}
	event, err := scanAIJobEvent(tx.QueryRowContext(ctx, `
		SELECT id, project_id, job_id, sequence, event_type, status,
		       progress, message, metadata_json, created_at
		FROM ai_job_events
		WHERE project_id = ? AND job_id = ?
		ORDER BY sequence DESC
		LIMIT 1
	`, projectID, jobID))
	if err != nil {
		return AIJobEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return AIJobEvent{}, err
	}
	return event, nil
}

func (s *Store) ListAIRuns(ctx context.Context, userID, projectID, cursor string, limit int, filter AIRunFilter) ([]AIRun, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	cursor = strings.TrimSpace(cursor)
	filter.ContentID = strings.TrimSpace(filter.ContentID)
	filter.RevisionID = strings.TrimSpace(filter.RevisionID)
	filter.JobID = strings.TrimSpace(filter.JobID)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" {
		if _, ok := allowedAIStatuses[filter.Status]; !ok {
			return nil, fmt.Errorf("%w: unsupported AI run status", ErrValidation)
		}
	}
	if cursor != "" {
		var exists int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM ai_runs
			WHERE project_id = ? AND id = ?
		`, projectID, cursor).Scan(&exists); err != nil {
			return nil, err
		}
		if exists != 1 {
			return nil, fmt.Errorf("%w: AI run cursor is not valid for this project", ErrValidation)
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT run.id, run.project_id, COALESCE(run.content_id, ''),
		       COALESCE(run.revision_id, ''), COALESCE(run.job_id, ''),
		       run.task_type, run.provider, run.model_identifier,
		       run.prompt_template_version, COALESCE(run.voice_profile_version, 0),
		       COALESCE(run.evidence_packet_version, 0), run.input_hash,
		       COALESCE(run.output_hash, ''), run.source_ids, run.started_by,
		       run.started_at, COALESCE(run.completed_at, ''), run.status,
		       COALESCE(run.input_tokens, 0), COALESCE(run.output_tokens, 0),
		       COALESCE(run.estimated_cost_cents, 0), COALESCE(run.error_category, '')
		FROM ai_runs run
		WHERE run.project_id = ?
		  AND (? = '' OR run.content_id = ?)
		  AND (? = '' OR run.revision_id = ?)
		  AND (? = '' OR run.job_id = ?)
		  AND (? = '' OR run.status = ?)
		  AND (
		    ? = ''
		    OR run.started_at < (
		      SELECT cursor_run.started_at
		      FROM ai_runs cursor_run
		      WHERE cursor_run.project_id = ? AND cursor_run.id = ?
		    )
		    OR (
		      run.started_at = (
		        SELECT cursor_run.started_at
		        FROM ai_runs cursor_run
		        WHERE cursor_run.project_id = ? AND cursor_run.id = ?
		      )
		      AND run.id < ?
		    )
		  )
		ORDER BY run.started_at DESC, run.id DESC
		LIMIT ?
	`, projectID,
		filter.ContentID, filter.ContentID,
		filter.RevisionID, filter.RevisionID,
		filter.JobID, filter.JobID,
		filter.Status, filter.Status,
		cursor, projectID, cursor, projectID, cursor, cursor,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []AIRun{}
	for rows.Next() {
		run, err := scanAIRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) ListQualityCheckResults(ctx context.Context, userID, projectID, cursor string, limit int, filter QualityCheckFilter) ([]QualityCheckResult, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	cursor = strings.TrimSpace(cursor)
	filter.ContentID = strings.TrimSpace(filter.ContentID)
	filter.RevisionID = strings.TrimSpace(filter.RevisionID)
	filter.Severity = strings.TrimSpace(filter.Severity)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Severity != "" {
		if _, ok := allowedQualitySeverities[filter.Severity]; !ok {
			return nil, fmt.Errorf("%w: unsupported quality-check severity", ErrValidation)
		}
	}
	if filter.Status != "" {
		if _, ok := allowedQualityStatuses[filter.Status]; !ok {
			return nil, fmt.Errorf("%w: unsupported quality-check status", ErrValidation)
		}
	}
	if cursor != "" {
		var exists int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM quality_check_results
			WHERE project_id = ? AND id = ?
		`, projectID, cursor).Scan(&exists); err != nil {
			return nil, err
		}
		if exists != 1 {
			return nil, fmt.Errorf("%w: quality-check cursor is not valid for this project", ErrValidation)
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT result.id, result.project_id, COALESCE(result.content_id, ''),
		       COALESCE(result.revision_id, ''), result.check_type,
		       result.severity, result.status, result.message,
		       result.evidence_json, COALESCE(result.override_reason, ''),
		       result.created_at
		FROM quality_check_results result
		WHERE result.project_id = ?
		  AND (? = '' OR result.content_id = ?)
		  AND (? = '' OR result.revision_id = ?)
		  AND (? = '' OR result.severity = ?)
		  AND (? = '' OR result.status = ?)
		  AND (
		    ? = ''
		    OR result.created_at < (
		      SELECT cursor_result.created_at
		      FROM quality_check_results cursor_result
		      WHERE cursor_result.project_id = ? AND cursor_result.id = ?
		    )
		    OR (
		      result.created_at = (
		        SELECT cursor_result.created_at
		        FROM quality_check_results cursor_result
		        WHERE cursor_result.project_id = ? AND cursor_result.id = ?
		      )
		      AND result.rowid < (
		        SELECT cursor_result.rowid
		        FROM quality_check_results cursor_result
		        WHERE cursor_result.project_id = ? AND cursor_result.id = ?
		      )
		    )
		  )
		ORDER BY result.created_at DESC, result.rowid DESC
		LIMIT ?
	`, projectID,
		filter.ContentID, filter.ContentID,
		filter.RevisionID, filter.RevisionID,
		filter.Severity, filter.Severity,
		filter.Status, filter.Status,
		cursor, projectID, cursor, projectID, cursor, projectID, cursor,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []QualityCheckResult{}
	for rows.Next() {
		result, err := scanQualityCheckResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func ensureRevisionQualityApproved(ctx context.Context, tx *sql.Tx, projectID, revisionID string) error {
	var checkType string
	err := tx.QueryRowContext(ctx, `
		SELECT result.check_type
		FROM quality_check_results result
		WHERE result.project_id = ?
		  AND result.revision_id = ?
		  AND result.severity IN ('blocking', 'critical')
		  AND result.status = 'failed'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM quality_check_results newer
		    WHERE newer.project_id = result.project_id
		      AND newer.revision_id = result.revision_id
		      AND newer.check_type = result.check_type
		      AND (
		        newer.created_at > result.created_at
		        OR (newer.created_at = result.created_at AND newer.rowid > result.rowid)
		      )
		  )
		ORDER BY CASE result.severity WHEN 'critical' THEN 0 ELSE 1 END,
		         result.created_at DESC,
		         result.id DESC
		LIMIT 1
	`, projectID, revisionID).Scan(&checkType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: latest %s quality check must pass or be overridden before approval", ErrInvalidWorkflow, checkType)
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
	if err != nil ||
		parsed.Scheme != "https" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.Fragment != "" {
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
	secretCiphertext, err := security.EncryptSecret(s.webhookEncryptionKey, secret)
	if err != nil {
		return WebhookWithSecret{}, fmt.Errorf("encrypt webhook signing secret: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WebhookWithSecret{}, err
	}
	defer tx.Rollback()
	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return WebhookWithSecret{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO webhook_endpoints(
		  id, project_id, name, url, secret_hash, secret_ciphertext,
		  events_json, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, endpointID, projectID, input.Name, input.URL, security.TokenHash(secret), secretCiphertext, eventsJSON, userID); err != nil {
		return WebhookWithSecret{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "webhook.create", "webhook_endpoint", endpointID, "success", map[string]string{
		"name": input.Name,
	}); err != nil {
		return WebhookWithSecret{}, err
	}
	if err := tx.Commit(); err != nil {
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
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
	if _, err := tx.ExecContext(ctx, `
		UPDATE webhook_attempts
		SET status = 'suppressed',
		    completed_at = CURRENT_TIMESTAMP,
		    locked_by = NULL,
		    locked_until = NULL
		WHERE project_id = ?
		  AND endpoint_id = ?
		  AND (
		    status IN ('queued','retrying')
		    OR (
		      status = 'processing'
		      AND locked_until <= CURRENT_TIMESTAMP
		    )
		  )
	`, projectID, endpointID); err != nil {
		return WebhookEndpoint{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "webhook.revoke", "webhook_endpoint", endpointID, "success", nil); err != nil {
		return WebhookEndpoint{}, err
	}
	if err := tx.Commit(); err != nil {
		return WebhookEndpoint{}, err
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
		       COALESCE(attempt.error_category, ''),
		       attempt.attempt_count, attempt.max_attempts,
		       COALESCE(attempt.next_attempt_at, ''),
		       COALESCE(attempt.response_duration_ms, 0),
		       COALESCE(attempt.last_error_safe_message, ''),
		       COALESCE(attempt.completed_at, ''),
		       COALESCE(attempt.replay_of_attempt_id, ''),
		       attempt.attempted_at
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
	replayRootID := attemptID
	if source.ReplayOfAttemptID != "" {
		replayRootID = source.ReplayOfAttemptID
	}
	var activeReplayCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM webhook_attempts
		WHERE replay_of_attempt_id = ?
		  AND status IN ('queued','processing','retrying')
	`, replayRootID).Scan(&activeReplayCount); err != nil {
		return WebhookAttempt{}, err
	}
	if activeReplayCount > 0 {
		return WebhookAttempt{}, fmt.Errorf("%w: a replay is already pending for this delivery", ErrInvalidWorkflow)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status,
		  error_category, replay_of_attempt_id, next_attempt_at
		) VALUES (?, ?, ?, ?, 'queued', ?, ?, CURRENT_TIMESTAMP)
	`, replayID, projectID, source.EndpointID, source.OutboxEventID, "manual_replay", replayRootID); err != nil {
		return WebhookAttempt{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "webhook_attempt.replay", "webhook_attempt", replayID, "success", map[string]string{
		"source_attempt_id": attemptID,
		"root_attempt_id":   replayRootID,
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
		  (SELECT COUNT(*) FROM webhook_attempts WHERE project_id = ? AND status IN ('queued', 'processing', 'retrying')),
		  (
		    SELECT COUNT(*)
		    FROM webhook_attempts failed
		    WHERE failed.project_id = ?
		      AND failed.replay_of_attempt_id IS NULL
		      AND failed.status IN ('failed', 'dead_letter')
		      AND NOT EXISTS (
		        SELECT 1
		        FROM webhook_attempts replay
		        WHERE replay.replay_of_attempt_id = failed.id
		          AND replay.status = 'succeeded'
		      )
		  ),
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
		       COALESCE(attempt.error_category, ''),
		       attempt.attempt_count, attempt.max_attempts,
		       COALESCE(attempt.next_attempt_at, ''),
		       COALESCE(attempt.response_duration_ms, 0),
		       COALESCE(attempt.last_error_safe_message, ''),
		       COALESCE(attempt.completed_at, ''),
		       COALESCE(attempt.replay_of_attempt_id, ''),
		       attempt.attempted_at
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

func appendAIJobEventTx(
	ctx context.Context,
	tx *sql.Tx,
	projectID, jobID, eventType, status string,
	progress int64,
	message string,
	metadata any,
) error {
	eventID, err := security.RandomID("aije")
	if err != nil {
		return err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := jsonString(metadata)
	if err != nil {
		return err
	}
	if eventType == "" || status == "" ||
		len(eventType) > 80 ||
		len(status) > 40 ||
		len(message) > 2000 ||
		len(metadataJSON) > 64*1024 {
		return fmt.Errorf("%w: AI job event exceeds its field limits", ErrValidation)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ai_job_events(
		  id, project_id, job_id, sequence, event_type, status,
		  progress, message, metadata_json
		)
		SELECT ?, ?, ?, COALESCE(MAX(sequence), 0) + 1, ?, ?, ?, ?, ?
		FROM ai_job_events
		WHERE project_id = ? AND job_id = ?
	`, eventID, projectID, jobID, eventType, status, progress, message, metadataJSON, projectID, jobID)
	return err
}

func canTransitionAIJob(current, next string) bool {
	if current == next {
		return current == "queued" || current == "running" || current == "needs_input"
	}
	switch current {
	case "queued":
		return next == "running" ||
			next == "succeeded" ||
			next == "failed" ||
			next == "cancelled" ||
			next == "budget_blocked" ||
			next == "safety_blocked"
	case "running":
		return next == "needs_input" ||
			next == "succeeded" ||
			next == "failed" ||
			next == "cancelled" ||
			next == "budget_blocked" ||
			next == "safety_blocked"
	case "needs_input":
		return next == "queued" ||
			next == "running" ||
			next == "failed" ||
			next == "cancelled" ||
			next == "budget_blocked" ||
			next == "safety_blocked"
	default:
		return false
	}
}

func isTerminalAIJobStatus(status string) bool {
	return status == "succeeded" ||
		status == "failed" ||
		status == "cancelled" ||
		status == "budget_blocked" ||
		status == "safety_blocked"
}

func scanAIJobEvent(row rowScanner) (AIJobEvent, error) {
	var event AIJobEvent
	var metadataJSON string
	if err := row.Scan(
		&event.ID,
		&event.ProjectID,
		&event.JobID,
		&event.Sequence,
		&event.Type,
		&event.Status,
		&event.Progress,
		&event.Message,
		&metadataJSON,
		&event.CreatedAt,
	); err != nil {
		return AIJobEvent{}, err
	}
	if !json.Valid([]byte(metadataJSON)) {
		return AIJobEvent{}, fmt.Errorf("decode AI job event metadata: invalid JSON")
	}
	event.Metadata = json.RawMessage(metadataJSON)
	return event, nil
}

func scanAIRun(row rowScanner) (AIRun, error) {
	var run AIRun
	var sourceIDsJSON string
	if err := row.Scan(
		&run.ID,
		&run.ProjectID,
		&run.ContentID,
		&run.RevisionID,
		&run.JobID,
		&run.Type,
		&run.Provider,
		&run.ModelIdentifier,
		&run.PromptTemplateVersion,
		&run.VoiceProfileVersion,
		&run.EvidencePacketVersion,
		&run.InputHash,
		&run.OutputHash,
		&sourceIDsJSON,
		&run.StartedBy,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.InputTokens,
		&run.OutputTokens,
		&run.EstimatedCostCents,
		&run.ErrorCategory,
	); err != nil {
		return AIRun{}, err
	}
	if err := json.Unmarshal([]byte(sourceIDsJSON), &run.SourceIDs); err != nil {
		return AIRun{}, fmt.Errorf("decode AI run source IDs: %w", err)
	}
	if run.SourceIDs == nil {
		run.SourceIDs = []string{}
	}
	return run, nil
}

func scanQualityCheckResult(row rowScanner) (QualityCheckResult, error) {
	var result QualityCheckResult
	var evidenceJSON string
	if err := row.Scan(
		&result.ID,
		&result.ProjectID,
		&result.ContentID,
		&result.RevisionID,
		&result.CheckType,
		&result.Severity,
		&result.Status,
		&result.Message,
		&evidenceJSON,
		&result.OverrideReason,
		&result.CreatedAt,
	); err != nil {
		return QualityCheckResult{}, err
	}
	if !json.Valid([]byte(evidenceJSON)) {
		return QualityCheckResult{}, fmt.Errorf("decode quality-check evidence: invalid JSON")
	}
	result.Evidence = json.RawMessage(evidenceJSON)
	return result, nil
}

func scanAdminMediaAsset(row rowScanner) (AdminMediaAsset, error) {
	var asset AdminMediaAsset
	var decorative int
	var metadataJSON string
	err := row.Scan(
		&asset.ID,
		&asset.ProjectID,
		&asset.Filename,
		&asset.ContentType,
		&asset.ObjectKey,
		&asset.Bucket,
		&asset.Bytes,
		&asset.Width,
		&asset.Height,
		&asset.AltText,
		&decorative,
		&asset.Caption,
		&asset.Credit,
		&asset.License,
		&asset.Status,
		&asset.SHA256,
		&asset.ExpectedSHA256,
		&asset.ScanStatus,
		&asset.ScanReason,
		&metadataJSON,
		&asset.CreatedBy,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	)
	if err != nil {
		return AdminMediaAsset{}, err
	}
	asset.Decorative = decorative == 1
	if json.Valid([]byte(metadataJSON)) && metadataJSON != "{}" {
		asset.Metadata = json.RawMessage(metadataJSON)
	}
	return asset, nil
}

func (s *Store) listMediaVariants(ctx context.Context, projectID, assetID string) ([]AdminMediaVariant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, variant_name, object_key, mime_type,
		       COALESCE(width, 0), COALESCE(height, 0), byte_size, created_at
		FROM asset_variants
		WHERE project_id = ? AND asset_id = ?
		ORDER BY
			CASE variant_name
				WHEN 'square_1x1' THEN 1
				WHEN 'landscape_4x3' THEN 2
				WHEN 'widescreen_16x9' THEN 3
				ELSE 4
			END,
			variant_name
	`, projectID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var variants []AdminMediaVariant
	for rows.Next() {
		var variant AdminMediaVariant
		if err := rows.Scan(
			&variant.ID,
			&variant.Name,
			&variant.ObjectKey,
			&variant.ContentType,
			&variant.Width,
			&variant.Height,
			&variant.Bytes,
			&variant.CreatedAt,
		); err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}
	return variants, rows.Err()
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
		&attempt.AttemptCount,
		&attempt.MaxAttempts,
		&attempt.NextAttemptAt,
		&attempt.ResponseDurationMillis,
		&attempt.LastErrorSafeMessage,
		&attempt.CompletedAt,
		&attempt.ReplayOfAttemptID,
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
		       COALESCE(attempt.error_category, ''),
		       attempt.attempt_count, attempt.max_attempts,
		       COALESCE(attempt.next_attempt_at, ''),
		       COALESCE(attempt.response_duration_ms, 0),
		       COALESCE(attempt.last_error_safe_message, ''),
		       COALESCE(attempt.completed_at, ''),
		       COALESCE(attempt.replay_of_attempt_id, ''),
		       attempt.attempted_at,
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
		&attempt.AttemptCount,
		&attempt.MaxAttempts,
		&attempt.NextAttemptAt,
		&attempt.ResponseDurationMillis,
		&attempt.LastErrorSafeMessage,
		&attempt.CompletedAt,
		&attempt.ReplayOfAttemptID,
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

func validateMediaUpload(input MediaUploadInput) (MediaUploadInput, error) {
	input.Filename = strings.TrimSpace(filepath.Base(strings.ReplaceAll(input.Filename, "\\", "/")))
	input.ContentType = normalizeMediaContentType(input.ContentType)
	if input.Filename == "" || input.Filename == "." {
		return input, fmt.Errorf("%w: filename is required", ErrValidation)
	}
	if input.ContentType == "" || input.Bytes <= 0 {
		return input, fmt.Errorf("%w: contentType and a positive byte size are required", ErrValidation)
	}

	extension := strings.ToLower(filepath.Ext(input.Filename))
	if extension == "" {
		return input, fmt.Errorf("%w: filename must include an allowed extension", ErrValidation)
	}
	if extension == ".svg" || input.ContentType == "image/svg+xml" {
		return input, fmt.Errorf("%w: SVG media requires a dedicated sanitizer and is disabled by default", ErrValidation)
	}

	policy, ok := allowedMediaUploads[input.ContentType]
	if !ok {
		return input, fmt.Errorf("%w: unsupported media content type", ErrValidation)
	}
	if _, ok := policy.Extensions[extension]; !ok {
		return input, fmt.Errorf("%w: filename extension does not match content type", ErrValidation)
	}
	if input.Bytes > policy.MaxBytes {
		return input, fmt.Errorf("%w: media file exceeds the %s limit", ErrValidation, humanByteLimit(policy.MaxBytes))
	}
	return input, nil
}

func mediaMetadataJSON(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	if len(raw) > 64*1024 {
		return "", fmt.Errorf("%w: media metadata is too large", ErrValidation)
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("%w: media metadata must be valid JSON", ErrValidation)
	}
	if string(raw) == "null" {
		return "{}", nil
	}
	return string(raw), nil
}

func normalizeSHA256(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return ""
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return ""
		}
	}
	return value
}

func normalizeMediaContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	mediaType = strings.ToLower(mediaType)
	if mediaType == "image/jpg" {
		return "image/jpeg"
	}
	return mediaType
}

func extensionSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[strings.ToLower(value)] = struct{}{}
	}
	return result
}

func humanByteLimit(bytes int64) string {
	if bytes%(1024*1024) == 0 {
		return fmt.Sprintf("%d MB", bytes/(1024*1024))
	}
	if bytes%1024 == 0 {
		return fmt.Sprintf("%d KB", bytes/1024)
	}
	if bytes == 1 {
		return "1 byte"
	}
	return fmt.Sprintf("%d bytes", bytes)
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

func nullZeroInt(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
