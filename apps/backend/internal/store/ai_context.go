package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"seoblog/apps/backend/internal/security"
)

type VoiceWritingExample struct {
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

type VoiceProfileDocument struct {
	Audience             string                `json:"audience"`
	AssumedKnowledge     string                `json:"assumedKnowledge"`
	BrandPurpose         string                `json:"brandPurpose"`
	PointOfView          string                `json:"pointOfView"`
	Tone                 string                `json:"tone"`
	Formality            string                `json:"formality"`
	Humor                string                `json:"humor"`
	PreferredVocabulary  []string              `json:"preferredVocabulary"`
	ProductTerminology   map[string]string     `json:"productTerminology"`
	ApprovedProductFacts []string              `json:"approvedProductFacts"`
	SentencePreferences  string                `json:"sentencePreferences"`
	ParagraphPreferences string                `json:"paragraphPreferences"`
	AvoidPhrases         []string              `json:"avoidPhrases"`
	ProhibitedClaims     []string              `json:"prohibitedClaims"`
	ContentTypeStyles    map[string]string     `json:"contentTypeStyles"`
	WritingExamples      []VoiceWritingExample `json:"writingExamples"`
	IntroductionRules    string                `json:"introductionRules"`
	ConclusionRules      string                `json:"conclusionRules"`
	CallToActionRules    string                `json:"callToActionRules"`
	RegionalSpelling     string                `json:"regionalSpelling"`
}

type VoiceProfile struct {
	ID        string               `json:"id"`
	ProjectID string               `json:"projectId"`
	Version   int64                `json:"version"`
	Profile   VoiceProfileDocument `json:"profile"`
	CreatedBy string               `json:"createdBy"`
	CreatedAt string               `json:"createdAt"`
}

type EvidenceFact struct {
	Statement string   `json:"statement"`
	SourceIDs []string `json:"sourceIds"`
}

type EvidencePacketDocument struct {
	HumanBrief                string         `json:"humanBrief"`
	SearchIntent              string         `json:"searchIntent"`
	Thesis                    string         `json:"thesis"`
	ProductFacts              []EvidenceFact `json:"productFacts"`
	SubjectMatterNotes        []string       `json:"subjectMatterNotes"`
	FirsthandObservations     []string       `json:"firsthandObservations"`
	SourceIDs                 []string       `json:"sourceIds"`
	CustomerEvidence          []string       `json:"customerEvidence"`
	Measurements              []string       `json:"measurements"`
	AllowedClaims             []string       `json:"allowedClaims"`
	ProhibitedClaims          []string       `json:"prohibitedClaims"`
	Limitations               []string       `json:"limitations"`
	RequiredInternalLinks     []string       `json:"requiredInternalLinks"`
	CallToAction              string         `json:"callToAction"`
	PublicationRecommendation string         `json:"publicationRecommendation"`
}

type EvidencePacket struct {
	ID         string                 `json:"id"`
	ProjectID  string                 `json:"projectId"`
	ContentID  string                 `json:"contentId,omitempty"`
	Version    int64                  `json:"version"`
	Packet     EvidencePacketDocument `json:"packet"`
	ApprovedBy string                 `json:"approvedBy,omitempty"`
	ApprovedAt string                 `json:"approvedAt,omitempty"`
	CreatedBy  string                 `json:"createdBy"`
	CreatedAt  string                 `json:"createdAt"`
}

type EvidencePacketInput struct {
	ContentID string
	Packet    EvidencePacketDocument
}

type EvidencePacketFilter struct {
	ContentID     string
	ApprovalState string
}

func (s *Store) GetLatestVoiceProfile(ctx context.Context, userID, projectID string) (VoiceProfile, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return VoiceProfile{}, err
	}
	return scanVoiceProfile(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, version, profile_json, created_by, created_at
		FROM voice_profiles
		WHERE project_id = ?
		ORDER BY version DESC
		LIMIT 1
	`, projectID))
}

func (s *Store) CreateVoiceProfile(ctx context.Context, userID, projectID string, document VoiceProfileDocument) (VoiceProfile, error) {
	if err := s.requireProjectManagement(ctx, userID, projectID); err != nil {
		return VoiceProfile{}, err
	}
	document = normalizeVoiceProfile(document)
	if err := validateVoiceProfile(document); err != nil {
		return VoiceProfile{}, err
	}
	profileJSON, err := jsonString(document)
	if err != nil {
		return VoiceProfile{}, err
	}
	if len(profileJSON) > 128*1024 {
		return VoiceProfile{}, fmt.Errorf("%w: voice profile exceeds 128 KB", ErrValidation)
	}
	profileID, err := security.RandomID("voice")
	if err != nil {
		return VoiceProfile{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return VoiceProfile{}, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM projects
		WHERE id = ?
	`, projectID).Scan(&status); err != nil {
		return VoiceProfile{}, err
	}
	if status != "active" {
		return VoiceProfile{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	var version int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM voice_profiles
		WHERE project_id = ?
	`, projectID).Scan(&version); err != nil {
		return VoiceProfile{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO voice_profiles(id, project_id, version, profile_json, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, profileID, projectID, version, profileJSON, userID); err != nil {
		return VoiceProfile{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "voice_profile.create", "voice_profile", profileID, "success", map[string]any{
		"version": version,
	}); err != nil {
		return VoiceProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return VoiceProfile{}, err
	}
	return s.GetLatestVoiceProfile(ctx, userID, projectID)
}

func (s *Store) ListEvidencePackets(
	ctx context.Context,
	userID, projectID, cursor string,
	limit int,
	filter EvidencePacketFilter,
) ([]EvidencePacket, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	cursor = strings.TrimSpace(cursor)
	filter.ContentID = strings.TrimSpace(filter.ContentID)
	filter.ApprovalState = strings.TrimSpace(filter.ApprovalState)
	if filter.ApprovalState != "" && filter.ApprovalState != "draft" && filter.ApprovalState != "approved" {
		return nil, fmt.Errorf("%w: unsupported evidence approval state", ErrValidation)
	}
	if cursor != "" {
		var exists int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM evidence_packets
			WHERE project_id = ?
			  AND id = ?
			  AND (? = '' OR content_id = ?)
			  AND (
			    ? = ''
			    OR (? = 'approved' AND approved_at IS NOT NULL)
			    OR (? = 'draft' AND approved_at IS NULL)
			  )
		`, projectID, cursor,
			filter.ContentID, filter.ContentID,
			filter.ApprovalState, filter.ApprovalState, filter.ApprovalState,
		).Scan(&exists); err != nil {
			return nil, err
		}
		if exists != 1 {
			return nil, fmt.Errorf("%w: evidence packet cursor is not valid for these filters", ErrValidation)
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT packet.id, packet.project_id, COALESCE(packet.content_id, ''),
		       packet.version, packet.packet_json, COALESCE(packet.approved_by, ''),
		       COALESCE(packet.approved_at, ''), packet.created_by, packet.created_at
		FROM evidence_packets packet
		WHERE packet.project_id = ?
		  AND (? = '' OR packet.content_id = ?)
		  AND (
		    ? = ''
		    OR (? = 'approved' AND packet.approved_at IS NOT NULL)
		    OR (? = 'draft' AND packet.approved_at IS NULL)
		  )
		  AND (
		    ? = ''
		    OR packet.created_at < (
		      SELECT cursor_packet.created_at
		      FROM evidence_packets cursor_packet
		      WHERE cursor_packet.project_id = ? AND cursor_packet.id = ?
		    )
		    OR (
		      packet.created_at = (
		        SELECT cursor_packet.created_at
		        FROM evidence_packets cursor_packet
		        WHERE cursor_packet.project_id = ? AND cursor_packet.id = ?
		      )
		      AND packet.rowid < (
		        SELECT cursor_packet.rowid
		        FROM evidence_packets cursor_packet
		        WHERE cursor_packet.project_id = ? AND cursor_packet.id = ?
		      )
		    )
		  )
		ORDER BY packet.created_at DESC, packet.rowid DESC
		LIMIT ?
	`, projectID,
		filter.ContentID, filter.ContentID,
		filter.ApprovalState, filter.ApprovalState, filter.ApprovalState,
		cursor, projectID, cursor, projectID, cursor, projectID, cursor,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packets := []EvidencePacket{}
	for rows.Next() {
		packet, err := scanEvidencePacket(rows)
		if err != nil {
			return nil, err
		}
		packets = append(packets, packet)
	}
	return packets, rows.Err()
}

func (s *Store) CreateEvidencePacket(ctx context.Context, userID, projectID string, input EvidencePacketInput) (EvidencePacket, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return EvidencePacket{}, err
	}
	input.ContentID = strings.TrimSpace(input.ContentID)
	input.Packet = normalizeEvidencePacket(input.Packet)
	if err := validateEvidencePacket(input.Packet); err != nil {
		return EvidencePacket{}, err
	}
	packetJSON, err := jsonString(input.Packet)
	if err != nil {
		return EvidencePacket{}, err
	}
	if len(packetJSON) > 256*1024 {
		return EvidencePacket{}, fmt.Errorf("%w: evidence packet exceeds 256 KB", ErrValidation)
	}
	packetID, err := security.RandomID("evidence")
	if err != nil {
		return EvidencePacket{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EvidencePacket{}, err
	}
	defer tx.Rollback()
	if status, err := projectStatus(ctx, tx, projectID); err != nil {
		return EvidencePacket{}, err
	} else if status != "active" {
		return EvidencePacket{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if input.ContentID != "" {
		var exists int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM content_items
			WHERE project_id = ? AND id = ?
		`, projectID, input.ContentID).Scan(&exists); err != nil {
			return EvidencePacket{}, err
		}
		if exists != 1 {
			return EvidencePacket{}, fmt.Errorf("%w: evidence content does not belong to this project", ErrValidation)
		}
	}
	sourceIDs := evidenceSourceIDs(input.Packet)
	if err := ensureEvidenceSources(ctx, tx, projectID, sourceIDs); err != nil {
		return EvidencePacket{}, err
	}
	var version int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM evidence_packets
		WHERE project_id = ? AND content_id IS ?
	`, projectID, nullIfEmpty(input.ContentID)).Scan(&version); err != nil {
		return EvidencePacket{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO evidence_packets(
		  id, project_id, content_id, version, packet_json, created_by
		) VALUES (?, ?, ?, ?, ?, ?)
	`, packetID, projectID, nullIfEmpty(input.ContentID), version, packetJSON, userID); err != nil {
		return EvidencePacket{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "evidence_packet.create", "evidence_packet", packetID, "success", map[string]any{
		"content_id":   input.ContentID,
		"version":      version,
		"source_count": len(sourceIDs),
	}); err != nil {
		return EvidencePacket{}, err
	}
	if err := tx.Commit(); err != nil {
		return EvidencePacket{}, err
	}
	return s.getEvidencePacket(ctx, projectID, packetID)
}

func (s *Store) ApproveEvidencePacket(ctx context.Context, userID, projectID, packetID string) (EvidencePacket, error) {
	if err := s.requireContentReview(ctx, userID, projectID); err != nil {
		return EvidencePacket{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EvidencePacket{}, err
	}
	defer tx.Rollback()
	if status, err := projectStatus(ctx, tx, projectID); err != nil {
		return EvidencePacket{}, err
	} else if status != "active" {
		return EvidencePacket{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	var approvedAt, packetJSON string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(approved_at, ''), packet_json
		FROM evidence_packets
		WHERE project_id = ? AND id = ?
	`, projectID, packetID).Scan(&approvedAt, &packetJSON); err != nil {
		return EvidencePacket{}, err
	}
	if approvedAt != "" {
		return EvidencePacket{}, fmt.Errorf("%w: evidence packet is already approved", ErrInvalidWorkflow)
	}
	var document EvidencePacketDocument
	if err := json.Unmarshal([]byte(packetJSON), &document); err != nil {
		return EvidencePacket{}, fmt.Errorf("decode evidence packet before approval: %w", err)
	}
	document = normalizeEvidencePacket(document)
	if err := validateEvidencePacket(document); err != nil {
		return EvidencePacket{}, err
	}
	if document.PublicationRecommendation != "ready" {
		return EvidencePacket{}, fmt.Errorf("%w: evidence packet is not ready for approval", ErrInvalidWorkflow)
	}
	if err := ensureEvidenceSources(ctx, tx, projectID, evidenceSourceIDs(document)); err != nil {
		return EvidencePacket{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE evidence_packets
		SET approved_by = ?, approved_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, userID, projectID, packetID); err != nil {
		return EvidencePacket{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "evidence_packet.approve", "evidence_packet", packetID, "success", nil); err != nil {
		return EvidencePacket{}, err
	}
	if err := tx.Commit(); err != nil {
		return EvidencePacket{}, err
	}
	return s.getEvidencePacket(ctx, projectID, packetID)
}

func (s *Store) getEvidencePacket(ctx context.Context, projectID, packetID string) (EvidencePacket, error) {
	return scanEvidencePacket(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, COALESCE(content_id, ''), version, packet_json,
		       COALESCE(approved_by, ''), COALESCE(approved_at, ''),
		       created_by, created_at
		FROM evidence_packets
		WHERE project_id = ? AND id = ?
	`, projectID, packetID))
}

func scanVoiceProfile(row rowScanner) (VoiceProfile, error) {
	var profile VoiceProfile
	var profileJSON string
	if err := row.Scan(
		&profile.ID,
		&profile.ProjectID,
		&profile.Version,
		&profileJSON,
		&profile.CreatedBy,
		&profile.CreatedAt,
	); err != nil {
		return VoiceProfile{}, err
	}
	if err := json.Unmarshal([]byte(profileJSON), &profile.Profile); err != nil {
		return VoiceProfile{}, fmt.Errorf("decode voice profile: %w", err)
	}
	return profile, nil
}

func scanEvidencePacket(row rowScanner) (EvidencePacket, error) {
	var packet EvidencePacket
	var packetJSON string
	if err := row.Scan(
		&packet.ID,
		&packet.ProjectID,
		&packet.ContentID,
		&packet.Version,
		&packetJSON,
		&packet.ApprovedBy,
		&packet.ApprovedAt,
		&packet.CreatedBy,
		&packet.CreatedAt,
	); err != nil {
		return EvidencePacket{}, err
	}
	if err := json.Unmarshal([]byte(packetJSON), &packet.Packet); err != nil {
		return EvidencePacket{}, fmt.Errorf("decode evidence packet: %w", err)
	}
	return packet, nil
}

func normalizeVoiceProfile(document VoiceProfileDocument) VoiceProfileDocument {
	document.Audience = strings.TrimSpace(document.Audience)
	document.AssumedKnowledge = strings.TrimSpace(document.AssumedKnowledge)
	document.BrandPurpose = strings.TrimSpace(document.BrandPurpose)
	document.PointOfView = strings.TrimSpace(document.PointOfView)
	document.Tone = strings.TrimSpace(document.Tone)
	document.Formality = strings.TrimSpace(document.Formality)
	document.Humor = strings.TrimSpace(document.Humor)
	document.PreferredVocabulary = uniqueStrings(document.PreferredVocabulary)
	document.ApprovedProductFacts = uniqueStrings(document.ApprovedProductFacts)
	document.SentencePreferences = strings.TrimSpace(document.SentencePreferences)
	document.ParagraphPreferences = strings.TrimSpace(document.ParagraphPreferences)
	document.AvoidPhrases = uniqueStrings(document.AvoidPhrases)
	document.ProhibitedClaims = uniqueStrings(document.ProhibitedClaims)
	document.IntroductionRules = strings.TrimSpace(document.IntroductionRules)
	document.ConclusionRules = strings.TrimSpace(document.ConclusionRules)
	document.CallToActionRules = strings.TrimSpace(document.CallToActionRules)
	document.RegionalSpelling = strings.TrimSpace(document.RegionalSpelling)
	document.ProductTerminology = normalizeStringMap(document.ProductTerminology)
	document.ContentTypeStyles = normalizeStringMap(document.ContentTypeStyles)
	for index := range document.WritingExamples {
		document.WritingExamples[index].Title = strings.TrimSpace(document.WritingExamples[index].Title)
		document.WritingExamples[index].Excerpt = strings.TrimSpace(document.WritingExamples[index].Excerpt)
	}
	return document
}

func validateVoiceProfile(document VoiceProfileDocument) error {
	required := map[string]string{
		"audience":             document.Audience,
		"assumedKnowledge":     document.AssumedKnowledge,
		"brandPurpose":         document.BrandPurpose,
		"pointOfView":          document.PointOfView,
		"tone":                 document.Tone,
		"formality":            document.Formality,
		"sentencePreferences":  document.SentencePreferences,
		"paragraphPreferences": document.ParagraphPreferences,
		"introductionRules":    document.IntroductionRules,
		"conclusionRules":      document.ConclusionRules,
		"callToActionRules":    document.CallToActionRules,
		"regionalSpelling":     document.RegionalSpelling,
	}
	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%w: voice profile %s is required", ErrValidation, field)
		}
	}
	if len(document.WritingExamples) < 3 {
		return fmt.Errorf("%w: voice profile requires at least three approved writing examples", ErrValidation)
	}
	for _, example := range document.WritingExamples {
		if example.Title == "" || len(example.Excerpt) < 40 {
			return fmt.Errorf("%w: each writing example needs a title and an excerpt of at least 40 characters", ErrValidation)
		}
	}
	if len(document.WritingExamples) > 20 ||
		len(document.PreferredVocabulary) > 200 ||
		len(document.AvoidPhrases) > 200 ||
		len(document.ProhibitedClaims) > 200 ||
		len(document.ProductTerminology) > 200 ||
		len(document.ContentTypeStyles) > 20 {
		return fmt.Errorf("%w: voice profile contains too many list entries", ErrValidation)
	}
	return nil
}

func normalizeEvidencePacket(document EvidencePacketDocument) EvidencePacketDocument {
	document.HumanBrief = strings.TrimSpace(document.HumanBrief)
	document.SearchIntent = strings.TrimSpace(document.SearchIntent)
	document.Thesis = strings.TrimSpace(document.Thesis)
	document.SubjectMatterNotes = uniqueStrings(document.SubjectMatterNotes)
	document.FirsthandObservations = uniqueStrings(document.FirsthandObservations)
	document.SourceIDs = uniqueStrings(document.SourceIDs)
	document.CustomerEvidence = uniqueStrings(document.CustomerEvidence)
	document.Measurements = uniqueStrings(document.Measurements)
	document.AllowedClaims = uniqueStrings(document.AllowedClaims)
	document.ProhibitedClaims = uniqueStrings(document.ProhibitedClaims)
	document.Limitations = uniqueStrings(document.Limitations)
	document.RequiredInternalLinks = uniqueStrings(document.RequiredInternalLinks)
	document.CallToAction = strings.TrimSpace(document.CallToAction)
	document.PublicationRecommendation = strings.TrimSpace(document.PublicationRecommendation)
	for index := range document.ProductFacts {
		document.ProductFacts[index].Statement = strings.TrimSpace(document.ProductFacts[index].Statement)
		document.ProductFacts[index].SourceIDs = uniqueStrings(document.ProductFacts[index].SourceIDs)
	}
	return document
}

func validateEvidencePacket(document EvidencePacketDocument) error {
	if len(document.HumanBrief) < 20 {
		return fmt.Errorf("%w: evidence human brief must be at least 20 characters", ErrValidation)
	}
	if len(document.SearchIntent) < 10 {
		return fmt.Errorf("%w: evidence search intent must be at least 10 characters", ErrValidation)
	}
	if len(document.Thesis) < 20 {
		return fmt.Errorf("%w: evidence thesis must be at least 20 characters", ErrValidation)
	}
	if len(document.CallToAction) < 5 {
		return fmt.Errorf("%w: evidence call to action must be at least 5 characters", ErrValidation)
	}
	switch document.PublicationRecommendation {
	case "ready", "request_unique_evidence", "do_not_publish":
	default:
		return fmt.Errorf("%w: unsupported evidence publication recommendation", ErrValidation)
	}
	if len(document.SourceIDs) > 100 ||
		len(document.ProductFacts) > 100 ||
		len(document.SubjectMatterNotes) > 100 ||
		len(document.FirsthandObservations) > 100 ||
		len(document.CustomerEvidence) > 100 ||
		len(document.Measurements) > 100 ||
		len(document.AllowedClaims) > 100 ||
		len(document.ProhibitedClaims) > 100 ||
		len(document.Limitations) > 100 ||
		len(document.RequiredInternalLinks) > 100 {
		return fmt.Errorf("%w: evidence packet contains too many entries", ErrValidation)
	}
	for _, fact := range document.ProductFacts {
		if fact.Statement == "" {
			return fmt.Errorf("%w: product facts cannot be empty", ErrValidation)
		}
	}
	if len(evidenceSourceIDs(document)) == 0 &&
		len(document.ProductFacts) == 0 &&
		len(document.SubjectMatterNotes) == 0 &&
		len(document.FirsthandObservations) == 0 &&
		len(document.CustomerEvidence) == 0 &&
		len(document.Measurements) == 0 {
		return fmt.Errorf("%w: evidence packet requires sources or original evidence", ErrValidation)
	}
	if !evidenceHasUniqueMaterial(document) && document.PublicationRecommendation == "ready" {
		return fmt.Errorf("%w: evidence without unique material must request more evidence or recommend not publishing", ErrValidation)
	}
	return nil
}

func evidenceHasUniqueMaterial(document EvidencePacketDocument) bool {
	return len(document.ProductFacts) > 0 ||
		len(document.SubjectMatterNotes) > 0 ||
		len(document.FirsthandObservations) > 0 ||
		len(document.CustomerEvidence) > 0 ||
		len(document.Measurements) > 0
}

func evidenceSourceIDs(document EvidencePacketDocument) []string {
	sourceIDs := append([]string{}, document.SourceIDs...)
	for _, fact := range document.ProductFacts {
		sourceIDs = append(sourceIDs, fact.SourceIDs...)
	}
	return uniqueStrings(sourceIDs)
}

func ensureEvidenceSources(ctx context.Context, tx *sql.Tx, projectID string, sourceIDs []string) error {
	for _, sourceID := range sourceIDs {
		var exists int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM sources
			WHERE project_id = ? AND id = ?
		`, projectID, sourceID).Scan(&exists); err != nil {
			return err
		}
		if exists != 1 {
			return fmt.Errorf("%w: evidence source %q does not belong to this project", ErrValidation, sourceID)
		}
	}
	return nil
}

func normalizeStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			normalized[key] = value
		}
	}
	return normalized
}
