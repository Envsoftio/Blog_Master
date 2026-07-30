package httpapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"

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

type aiJobRequest struct {
	Type      string `json:"type"`
	ContentID string `json:"contentId"`
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
	asset, err := s.store.CreateMediaAsset(c.UserContext(), user.ID, c.Params("projectID"), store.MediaUploadInput{
		Filename:    input.Filename,
		ContentType: input.ContentType,
		Bytes:       input.Bytes,
	})
	if err != nil {
		return s.adminMutationError(c, err, "Could not register media")
	}
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
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminMediaAsset]{Data: asset})
}

func (s *Server) deleteMediaAsset(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.DeleteMediaAsset(c.UserContext(), user.ID, c.Params("projectID"), c.Params("assetID")); err != nil {
		return s.adminMutationError(c, err, "Could not delete media")
	}
	return c.SendStatus(fiber.StatusNoContent)
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
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	job, err := s.store.CreateAIJob(c.UserContext(), user.ID, c.Params("projectID"), store.AIJobInput{
		Type:      input.Type,
		ContentID: strings.TrimSpace(input.ContentID),
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
