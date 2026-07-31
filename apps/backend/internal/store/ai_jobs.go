package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const adminAIJobSelectColumns = `
	id, project_id, COALESCE(content_id, ''), COALESCE(revision_id, ''),
	task_type, COALESCE(article_type, ''), status,
	COALESCE(prompt_template_version, ''),
	COALESCE(voice_profile_id, ''), COALESCE(voice_profile_version, 0),
	COALESCE(evidence_packet_id, ''), COALESCE(evidence_packet_version, 0),
	COALESCE(input_hash, ''), COALESCE(source_revision_hash, ''),
	started_at, COALESCE(completed_at, started_at), COALESCE(output_json, '{}'),
	COALESCE(error_category, '')`

type AIJobContentContext struct {
	ID           string `json:"id"`
	RevisionID   string `json:"revisionId"`
	RevisionHash string `json:"revisionHash"`
}

type AIJobVoiceContext struct {
	ID      string               `json:"id"`
	Version int64                `json:"version"`
	Profile VoiceProfileDocument `json:"profile"`
}

type AIJobEvidenceContext struct {
	ID      string                 `json:"id"`
	Version int64                  `json:"version"`
	Packet  EvidencePacketDocument `json:"packet"`
}

type AIJobInputSnapshot struct {
	SchemaVersion         string               `json:"schemaVersion"`
	TaskType              string               `json:"taskType"`
	ArticleType           string               `json:"articleType"`
	PromptTemplateVersion string               `json:"promptTemplateVersion"`
	Content               AIJobContentContext  `json:"content"`
	Brief                 AIJobBrief           `json:"brief"`
	Voice                 AIJobVoiceContext    `json:"voice"`
	Evidence              AIJobEvidenceContext `json:"evidence"`
}

type aiJobQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func normalizeAIJobInput(input AIJobInput) AIJobInput {
	input.Type = strings.TrimSpace(input.Type)
	input.ContentID = strings.TrimSpace(input.ContentID)
	input.ArticleType = strings.TrimSpace(input.ArticleType)
	input.EvidencePacketID = strings.TrimSpace(input.EvidencePacketID)
	input.Brief.Title = strings.TrimSpace(input.Brief.Title)
	input.Brief.Purpose = strings.TrimSpace(input.Brief.Purpose)
	input.Brief.Audience = strings.TrimSpace(input.Brief.Audience)
	input.Brief.UniqueAngle = strings.TrimSpace(input.Brief.UniqueAngle)
	input.Brief.Evidence = strings.TrimSpace(input.Brief.Evidence)
	input.Brief.CTA = strings.TrimSpace(input.Brief.CTA)
	return input
}

func validateAIJobInput(input AIJobInput) error {
	if input.Type != "outline" && input.Type != "draft" && input.Type != "quality_check" {
		return fmt.Errorf("%w: unsupported AI job type", ErrValidation)
	}
	if input.ContentID == "" {
		return fmt.Errorf("%w: contentId is required", ErrValidation)
	}
	if input.EvidencePacketID == "" {
		return fmt.Errorf("%w: evidencePacketId is required", ErrValidation)
	}
	if input.VoiceProfileVersion < 0 {
		return fmt.Errorf("%w: voiceProfileVersion cannot be negative", ErrValidation)
	}
	required := map[string]struct {
		value string
		min   int
	}{
		"title":       {value: input.Brief.Title, min: 8},
		"purpose":     {value: input.Brief.Purpose, min: 20},
		"audience":    {value: input.Brief.Audience, min: 10},
		"uniqueAngle": {value: input.Brief.UniqueAngle, min: 15},
		"cta":         {value: input.Brief.CTA, min: 5},
	}
	for field, rule := range required {
		if utf8.RuneCountInString(rule.value) < rule.min {
			return fmt.Errorf("%w: AI brief %s must be at least %d characters", ErrValidation, field, rule.min)
		}
	}
	if utf8.RuneCountInString(input.Brief.Title) > 300 ||
		utf8.RuneCountInString(input.Brief.Purpose) > 4000 ||
		utf8.RuneCountInString(input.Brief.Audience) > 2000 ||
		utf8.RuneCountInString(input.Brief.UniqueAngle) > 4000 ||
		utf8.RuneCountInString(input.Brief.Evidence) > 8000 ||
		utf8.RuneCountInString(input.Brief.CTA) > 2000 {
		return fmt.Errorf("%w: AI brief fields exceed their size limits", ErrValidation)
	}
	return nil
}

func aiPromptTemplateVersion(taskType string) string {
	switch taskType {
	case "outline":
		return "outline-v1"
	case "draft":
		return "section-draft-v1"
	case "quality_check":
		return "quality-critique-v1"
	default:
		return ""
	}
}

func encodeAIJobSnapshot(snapshot AIJobInputSnapshot) (string, string, error) {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", "", fmt.Errorf("encode AI job input snapshot: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return string(encoded), fmt.Sprintf("%x", sum), nil
}

func (s *Store) GetAIJobInputSnapshot(ctx context.Context, projectID, jobID string) (AIJobInputSnapshot, error) {
	var inputJSON, inputHash string
	if err := s.db.QueryRowContext(ctx, `
		SELECT input_json, COALESCE(input_hash, '')
		FROM ai_jobs
		WHERE project_id = ? AND id = ?
	`, projectID, jobID).Scan(&inputJSON, &inputHash); err != nil {
		return AIJobInputSnapshot{}, err
	}
	var snapshot AIJobInputSnapshot
	if err := json.Unmarshal([]byte(inputJSON), &snapshot); err != nil {
		return AIJobInputSnapshot{}, fmt.Errorf("decode AI job input snapshot: %w", err)
	}
	_, actualHash, err := encodeAIJobSnapshot(snapshot)
	if err != nil {
		return AIJobInputSnapshot{}, err
	}
	if inputHash == "" || inputHash != actualHash {
		return AIJobInputSnapshot{}, fmt.Errorf("%w: AI job input snapshot hash does not match", ErrInvalidWorkflow)
	}
	return snapshot, nil
}

func findReusableAIJob(ctx context.Context, queryer aiJobQueryer, projectID, inputHash string) (AdminAIJob, error) {
	return scanAdminAIJob(queryer.QueryRowContext(ctx, `SELECT `+adminAIJobSelectColumns+`
		FROM ai_jobs
		WHERE project_id = ?
		  AND input_hash = ?
		  AND prompt_template_version <> ''
		  AND voice_profile_id IS NOT NULL
		  AND evidence_packet_id IS NOT NULL
		  AND status IN ('queued', 'running', 'needs_input', 'succeeded')
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`, projectID, inputHash))
}

func (s *Store) recordAIJobReuse(ctx context.Context, projectID, userID string, job AdminAIJob) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertAuditEventTx(ctx, tx, projectID, "user", userID, "ai_job.reuse", "ai_job", job.ID, "success", map[string]any{
		"input_hash": job.InputHash,
		"status":     job.Status,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func scanAdminAIJob(row rowScanner) (AdminAIJob, error) {
	var job AdminAIJob
	var resultJSON string
	if err := row.Scan(
		&job.ID,
		&job.ProjectID,
		&job.ContentID,
		&job.RevisionID,
		&job.Type,
		&job.ArticleType,
		&job.Status,
		&job.PromptTemplateVersion,
		&job.VoiceProfileID,
		&job.VoiceProfileVersion,
		&job.EvidencePacketID,
		&job.EvidencePacketVersion,
		&job.InputHash,
		&job.SourceRevisionHash,
		&job.CreatedAt,
		&job.UpdatedAt,
		&resultJSON,
		&job.Error,
	); err != nil {
		return AdminAIJob{}, err
	}
	job.Result = json.RawMessage(resultJSON)
	return job, nil
}
