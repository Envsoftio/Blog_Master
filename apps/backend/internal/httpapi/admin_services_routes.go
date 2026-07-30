package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"seoblog/apps/backend/internal/media"
	"seoblog/apps/backend/internal/store"
)

type mediaUploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Bytes       int64  `json:"bytes"`
}

type mediaPatchRequest struct {
	AltText    *string `json:"altText"`
	Decorative *bool   `json:"decorative"`
	Caption    *string `json:"caption"`
	Credit     *string `json:"credit"`
	License    *string `json:"license"`
}

type mediaCompleteRequest struct {
	SHA256 string `json:"sha256"`
}

type aiJobRequest struct {
	Type                string           `json:"type"`
	ContentID           string           `json:"contentId"`
	ArticleType         string           `json:"articleType"`
	EvidencePacketID    string           `json:"evidencePacketId"`
	VoiceProfileVersion int64            `json:"voiceProfileVersion"`
	Brief               store.AIJobBrief `json:"brief"`
}

type webhookRequest struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

func (s *Server) listMediaAssets(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	assets, err := s.store.ListMediaAssets(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list media")
	}
	for index := range assets {
		s.enrichMediaAsset(&assets[index])
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AdminMediaAsset]{
		Data: assets,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(assets)},
	})
}

func (s *Server) createMediaAsset(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input mediaUploadRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	now := time.Now().UTC()
	uploadStatus := "registered"
	uploadBucket := ""
	var uploadExpiresAt time.Time
	if s.mediaStorage != nil {
		uploadStatus = "uploading"
		uploadBucket = s.mediaStorage.Bucket()
		uploadExpiresAt = now.Add(mediaPresignTTL(s.cfg.B2MediaPresignTTL))
	}
	asset, err := s.store.CreateMediaAsset(c.UserContext(), user.ID, c.Params("projectID"), store.MediaUploadInput{
		Filename:        input.Filename,
		ContentType:     input.ContentType,
		Bytes:           input.Bytes,
		Bucket:          uploadBucket,
		Status:          uploadStatus,
		UploadExpiresAt: uploadExpiresAt,
	})
	if err != nil {
		return s.adminMutationError(c, err, "Could not register media")
	}
	if s.mediaStorage != nil {
		signed, err := s.mediaStorage.PresignPut(asset.ObjectKey, asset.ContentType, asset.Bytes, now)
		if err != nil {
			_, _ = s.store.FailMediaAsset(c.UserContext(), user.ID, c.Params("projectID"), asset.ID, "could not sign B2 upload")
			return problem(c, fiber.StatusInternalServerError, "Could not sign media upload", "")
		}
		asset.Upload = &store.MediaUploadTarget{
			URL:       signed.URL,
			Method:    signed.Method,
			Headers:   signed.Headers,
			Fields:    signed.Fields,
			ExpiresAt: signed.ExpiresAt,
			MaxBytes:  signed.MaxBytes,
		}
	}
	s.enrichMediaAsset(&asset)
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminMediaAsset]{Data: asset})
}

func (s *Server) getMediaAsset(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	asset, err := s.store.GetMediaAsset(c.UserContext(), user.ID, c.Params("projectID"), c.Params("assetID"))
	if err != nil {
		return s.adminReadError(c, err, "Media asset not found", "Could not load media")
	}
	s.enrichMediaAsset(&asset)
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminMediaAsset]{Data: asset})
}

func (s *Server) completeMediaUpload(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if s.mediaStorage == nil {
		return problem(c, fiber.StatusConflict, "Media storage is not configured", "Configure Backblaze B2 media storage before completing uploads")
	}
	var input mediaCompleteRequest
	if len(strings.TrimSpace(string(c.Body()))) > 0 {
		if err := decodeStrictRequestBody(c, &input); err != nil {
			return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
		}
	}
	if !validOptionalSHA256(input.SHA256) {
		return problem(c, fiber.StatusBadRequest, "Invalid media checksum", "sha256 must be a 64-character hexadecimal digest")
	}
	projectID := c.Params("projectID")
	assetID := c.Params("assetID")
	asset, err := s.store.MarkMediaAssetProcessing(c.UserContext(), user.ID, projectID, assetID, input.SHA256)
	if err != nil {
		return s.adminMutationError(c, err, "Could not process media")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminMediaAsset]{Data: asset})
}

func (s *Server) updateMediaAsset(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input mediaPatchRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	asset, err := s.store.UpdateMediaAsset(c.UserContext(), user.ID, c.Params("projectID"), c.Params("assetID"), store.MediaPatch{
		AltText:    input.AltText,
		Decorative: input.Decorative,
		Caption:    input.Caption,
		Credit:     input.Credit,
		License:    input.License,
	})
	if err != nil {
		return s.adminMutationError(c, err, "Could not update media")
	}
	s.enrichMediaAsset(&asset)
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminMediaAsset]{Data: asset})
}

func (s *Server) deleteMediaAsset(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	projectID := c.Params("projectID")
	assetID := c.Params("assetID")
	asset, err := s.store.GetMediaAsset(c.UserContext(), user.ID, projectID, assetID)
	if err != nil {
		return s.adminReadError(c, err, "Media asset not found", "Could not delete media")
	}
	if err := s.deleteMediaObjects(c.UserContext(), asset); err != nil {
		s.logger.Error("media object deletion failed", "asset_id", asset.ID, "error", err)
		return problem(c, fiber.StatusBadGateway, "Could not delete media objects", "")
	}
	if err := s.store.DeleteMediaAsset(c.UserContext(), user.ID, projectID, assetID); err != nil {
		return s.adminMutationError(c, err, "Could not delete media")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) deleteMediaObjects(ctx context.Context, asset store.AdminMediaAsset) error {
	if s.mediaStorage == nil {
		return nil
	}
	keys := []string{asset.ObjectKey}
	for _, variant := range asset.Variants {
		keys = append(keys, variant.ObjectKey)
	}
	keys = uniqueMediaObjectKeys(keys)
	for _, key := range keys {
		if !media.DeletableObjectKeyForAsset(asset.ProjectID, asset.ID, key) {
			return fmt.Errorf("refusing to delete media object outside asset scope: %q", key)
		}
	}
	var errs []error
	for _, key := range keys {
		if err := s.mediaStorage.DeleteObject(ctx, key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func uniqueMediaObjectKeys(keys []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func (s *Server) enrichMediaAsset(asset *store.AdminMediaAsset) {
	if s.mediaStorage == nil || asset == nil || asset.Status != "ready" {
		return
	}
	for index := range asset.Variants {
		asset.Variants[index].URL = s.mediaStorage.PublicURL(asset.Variants[index].ObjectKey)
	}
	if len(asset.Variants) > 0 && asset.Variants[0].URL != "" {
		asset.URL = asset.Variants[0].URL
		return
	}
	asset.URL = s.mediaStorage.PublicURL(asset.ObjectKey)
}

func mediaPresignTTL(configured time.Duration) time.Duration {
	if configured <= 0 {
		return 15 * time.Minute
	}
	if configured > 7*24*time.Hour {
		return 7 * 24 * time.Hour
	}
	return configured
}

func validOptionalSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') &&
			(character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}

func (s *Server) listAIJobs(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	jobs, err := s.store.ListAIJobs(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list AI jobs")
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AdminAIJob]{
		Data: jobs,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(jobs)},
	})
}

func (s *Server) createAIJob(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input aiJobRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	job, err := s.store.CreateAIJob(c.UserContext(), user.ID, c.Params("projectID"), store.AIJobInput{
		Type:                input.Type,
		ContentID:           strings.TrimSpace(input.ContentID),
		ArticleType:         input.ArticleType,
		EvidencePacketID:    input.EvidencePacketID,
		VoiceProfileVersion: input.VoiceProfileVersion,
		Brief:               input.Brief,
	})
	if err != nil {
		return s.adminMutationError(c, err, "Could not create AI job")
	}
	return writeJSON(c, fiber.StatusAccepted, Envelope[store.AdminAIJob]{Data: job})
}

func (s *Server) getAIJob(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	job, err := s.store.GetAIJob(c.UserContext(), user.ID, c.Params("projectID"), c.Params("jobID"))
	if err != nil {
		return s.adminReadError(c, err, "AI job not found", "Could not load AI job")
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminAIJob]{Data: job})
}

func (s *Server) cancelAIJob(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	job, err := s.store.CancelAIJob(c.UserContext(), user.ID, c.Params("projectID"), c.Params("jobID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not cancel AI job")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminAIJob]{Data: job})
}

func (s *Server) listAIJobEvents(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	after, err := strconv.ParseInt(c.Query("after", "0"), 10, 64)
	if err != nil || after < 0 {
		return problem(c, fiber.StatusBadRequest, "Invalid event cursor", "after must be a non-negative event sequence")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	events, err := s.store.ListAIJobEvents(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		c.Params("jobID"),
		after,
		limit+1,
	)
	if err != nil {
		return s.adminReadError(c, err, "AI job not found", "Could not load AI job events")
	}
	nextCursor := ""
	if len(events) > limit {
		events = events[:limit]
		nextCursor = strconv.FormatInt(events[len(events)-1].Sequence, 10)
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AIJobEvent]{
		Data: events,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) listAIRuns(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	runs, err := s.store.ListAIRuns(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		c.Query("cursor"),
		limit+1,
		store.AIRunFilter{
			ContentID:  c.Query("contentId"),
			RevisionID: c.Query("revisionId"),
			JobID:      c.Query("jobId"),
			Status:     c.Query("status"),
		},
	)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list AI runs")
	}
	nextCursor := ""
	if len(runs) > limit {
		runs = runs[:limit]
		nextCursor = runs[len(runs)-1].ID
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AIRun]{
		Data: runs,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) listQualityCheckResults(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	results, err := s.store.ListQualityCheckResults(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		c.Query("cursor"),
		limit+1,
		store.QualityCheckFilter{
			ContentID:  c.Query("contentId"),
			RevisionID: c.Query("revisionId"),
			Severity:   c.Query("severity"),
			Status:     c.Query("status"),
		},
	)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list quality checks")
	}
	nextCursor := ""
	if len(results) > limit {
		results = results[:limit]
		nextCursor = results[len(results)-1].ID
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.QualityCheckResult]{
		Data: results,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) listWebhooks(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	endpoints, err := s.store.ListWebhooks(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list webhooks")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.WebhookEndpoint]{
		Data: endpoints,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(endpoints)},
	})
}

func (s *Server) createWebhook(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input webhookRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	endpoint, err := s.store.CreateWebhook(c.UserContext(), user.ID, c.Params("projectID"), store.WebhookInput{
		Name:   input.Name,
		URL:    input.URL,
		Events: input.Events,
	})
	if err != nil {
		return s.adminMutationError(c, err, "Could not create webhook")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.WebhookWithSecret]{Data: endpoint})
}

func (s *Server) revokeWebhook(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	endpoint, err := s.store.RevokeWebhook(c.UserContext(), user.ID, c.Params("projectID"), c.Params("endpointID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not revoke webhook")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.WebhookEndpoint]{Data: endpoint})
}

func (s *Server) listWebhookAttempts(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	attempts, err := s.store.ListWebhookAttempts(c.UserContext(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list webhook attempts")
	}
	nextCursor := ""
	if len(attempts) > limit {
		attempts = attempts[:limit]
		nextCursor = attempts[len(attempts)-1].ID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.WebhookAttempt]{
		Data: attempts,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) replayWebhookAttempt(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	attempt, err := s.store.ReplayWebhookAttempt(c.UserContext(), user.ID, c.Params("projectID"), c.Params("attemptID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not replay webhook attempt")
	}
	return writeJSON(c, fiber.StatusAccepted, Envelope[store.WebhookAttempt]{Data: attempt})
}

func (s *Server) deliveryStatus(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	status, err := s.store.DeliveryStatus(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not load delivery status")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.DeliveryStatus]{Data: status})
}
