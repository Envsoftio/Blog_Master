package httpapi

import (
	"github.com/gofiber/fiber/v3"

	"seoblog/apps/backend/internal/store"
)

type voiceProfileRequest struct {
	Profile store.VoiceProfileDocument `json:"profile"`
}

type evidencePacketRequest struct {
	ContentID string                       `json:"contentId"`
	Packet    store.EvidencePacketDocument `json:"packet"`
}

func (s *Server) getVoiceProfile(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	profile, err := s.store.GetLatestVoiceProfile(c.Context(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Voice profile not found", "Could not load voice profile")
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, Envelope[store.VoiceProfile]{Data: profile})
}

func (s *Server) createVoiceProfile(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input voiceProfileRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	profile, err := s.store.CreateVoiceProfile(c.Context(), user.ID, c.Params("projectID"), input.Profile)
	if err != nil {
		return s.adminMutationError(c, err, "Could not create voice profile")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.VoiceProfile]{Data: profile})
}

func (s *Server) listEvidencePackets(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	packets, err := s.store.ListEvidencePackets(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Query("cursor"),
		limit+1,
		store.EvidencePacketFilter{
			ContentID:     c.Query("contentId"),
			ApprovalState: c.Query("approvalState"),
		},
	)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list evidence packets")
	}
	nextCursor := ""
	if len(packets) > limit {
		packets = packets[:limit]
		nextCursor = packets[len(packets)-1].ID
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.EvidencePacket]{
		Data: packets,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) createEvidencePacket(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input evidencePacketRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	packet, err := s.store.CreateEvidencePacket(c.Context(), user.ID, c.Params("projectID"), store.EvidencePacketInput{
		ContentID: input.ContentID,
		Packet:    input.Packet,
	})
	if err != nil {
		return s.adminMutationError(c, err, "Could not create evidence packet")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.EvidencePacket]{Data: packet})
}

func (s *Server) approveEvidencePacket(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	packet, err := s.store.ApproveEvidencePacket(c.Context(), user.ID, c.Params("projectID"), c.Params("packetID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not approve evidence packet")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.EvidencePacket]{Data: packet})
}
