package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"seoblog/apps/backend/internal/security"
)

type AIExecutionJob struct {
	Job       AdminAIJob
	Snapshot  AIJobInputSnapshot
	StartedBy string
	Attempts  int
	Provider  string
	Model     string
}

type AIExecutionResult struct {
	Output             json.RawMessage
	InputTokens        int
	OutputTokens       int
	EstimatedCostCents int
}

type aiQualityOutput struct {
	Checks []struct {
		Type     string          `json:"type"`
		Severity string          `json:"severity"`
		Status   string          `json:"status"`
		Message  string          `json:"message"`
		Evidence json.RawMessage `json:"evidence"`
	} `json:"checks"`
}

func (s *Store) ClaimAIJobs(ctx context.Context, workerID, provider, model string, leaseDuration time.Duration, limit int) ([]AIExecutionJob, error) {
	workerID = strings.TrimSpace(workerID)
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if workerID == "" || provider == "" || model == "" {
		return nil, fmt.Errorf("%w: AI worker, provider, and model are required", ErrValidation)
	}
	if limit <= 0 || limit > 20 {
		return nil, fmt.Errorf("%w: AI claim limit must be between 1 and 20", ErrValidation)
	}
	leaseSeconds := int64((leaseDuration + time.Second - 1) / time.Second)
	if leaseSeconds < 30 || leaseSeconds > 3600 {
		return nil, fmt.Errorf("%w: AI lease duration must be between 30 seconds and 1 hour", ErrValidation)
	}
	leaseModifier := fmt.Sprintf("+%d seconds", leaseSeconds)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT job.id, job.project_id
		FROM ai_jobs job
		JOIN projects project ON project.id = job.project_id
		WHERE project.status = 'active'
		  AND (
		    job.status = 'queued'
		    OR (job.status = 'running' AND (job.locked_until IS NULL OR job.locked_until < CURRENT_TIMESTAMP))
		  )
		ORDER BY job.started_at ASC, job.id ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	type candidate struct{ jobID, projectID string }
	candidates := make([]candidate, 0, limit)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.jobID, &item.projectID); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	claimed := make([]AIExecutionJob, 0, len(candidates))
	for _, candidate := range candidates {
		result, err := tx.ExecContext(ctx, `
			UPDATE ai_jobs
			SET status = 'running', provider = ?, model_identifier = ?,
			    locked_by = ?, locked_until = datetime(CURRENT_TIMESTAMP, ?),
			    attempt_count = attempt_count + 1, completed_at = NULL,
			    error_category = NULL
			WHERE project_id = ? AND id = ?
			  AND (
			    status = 'queued'
		    OR (status = 'running' AND (locked_until IS NULL OR locked_until < CURRENT_TIMESTAMP))
			  )
		`, provider, model, workerID, leaseModifier, candidate.projectID, candidate.jobID)
		if err != nil {
			return nil, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if changed == 0 {
			continue
		}

		var inputJSON, inputHash, startedBy string
		var attempts int
		if err := tx.QueryRowContext(ctx, `
			SELECT input_json, input_hash, started_by, attempt_count
			FROM ai_jobs
			WHERE project_id = ? AND id = ?
		`, candidate.projectID, candidate.jobID).Scan(&inputJSON, &inputHash, &startedBy, &attempts); err != nil {
			return nil, err
		}
		snapshot, err := decodeAIJobSnapshot(inputJSON, inputHash)
		if err != nil {
			if err := rejectInvalidAIJobTx(ctx, tx, workerID, candidate.projectID, candidate.jobID); err != nil {
				return nil, err
			}
			continue
		}
		job, err := scanAdminAIJob(tx.QueryRowContext(ctx, `SELECT `+adminAIJobSelectColumns+`
			FROM ai_jobs WHERE project_id = ? AND id = ?
		`, candidate.projectID, candidate.jobID))
		if err != nil {
			return nil, err
		}
		if err := appendAIJobEventTx(ctx, tx, candidate.projectID, candidate.jobID, "started", "running", 5, "AI generation started", map[string]any{
			"attempt":  attempts,
			"provider": provider,
			"model":    model,
		}); err != nil {
			return nil, err
		}
		claimed = append(claimed, AIExecutionJob{
			Job: job, Snapshot: snapshot, StartedBy: startedBy, Attempts: attempts,
			Provider: provider, Model: model,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return claimed, nil
}

func rejectInvalidAIJobTx(ctx context.Context, tx *sql.Tx, workerID, projectID, jobID string) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = 'failed', error_category = 'invalid_input', completed_at = CURRENT_TIMESTAMP,
		    locked_by = NULL, locked_until = NULL
		WHERE project_id = ? AND id = ? AND status = 'running' AND locked_by = ?
	`, projectID, jobID, workerID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}
	if err := appendAIJobEventTx(ctx, tx, projectID, jobID, "rejected", "failed", 0, "AI input snapshot failed its integrity check", map[string]any{
		"category": "invalid_input",
	}); err != nil {
		return err
	}
	return insertAuditEventTx(ctx, tx, projectID, "system", workerID, "ai_job.reject", "ai_job", jobID, "failed", map[string]any{
		"category": "invalid_input",
	})
}

func (s *Store) CompleteAIJob(ctx context.Context, workerID string, execution AIExecutionJob, result AIExecutionResult) error {
	if !json.Valid(result.Output) || len(result.Output) == 0 || len(result.Output) > 1024*1024 {
		return fmt.Errorf("%w: AI output must be valid JSON no larger than 1 MB", ErrValidation)
	}
	if err := validateAIExecutionOutput(execution.Job.Type, result.Output); err != nil {
		return err
	}
	if result.InputTokens < 0 || result.OutputTokens < 0 || result.EstimatedCostCents < 0 {
		return fmt.Errorf("%w: AI usage cannot be negative", ErrValidation)
	}
	outputHashBytes := sha256.Sum256(result.Output)
	outputHash := hex.EncodeToString(outputHashBytes[:])
	runID, err := security.RandomID("airun")
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updated, err := tx.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = 'succeeded', output_json = ?, output_hash = ?,
		    input_tokens = ?, output_tokens = ?, estimated_cost_cents = ?,
		    completed_at = CURRENT_TIMESTAMP, error_category = NULL,
		    locked_by = NULL, locked_until = NULL
		WHERE project_id = ? AND id = ? AND status = 'running' AND locked_by = ?
	`, string(result.Output), outputHash, result.InputTokens, result.OutputTokens, result.EstimatedCostCents,
		execution.Job.ProjectID, execution.Job.ID, workerID)
	if err != nil {
		return err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}

	sourceIDsJSON, err := json.Marshal(evidenceSourceIDs(execution.Snapshot.Evidence.Packet))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ai_runs(
		  id, project_id, content_id, revision_id, job_id, task_type,
		  provider, model_identifier, prompt_template_version,
		  voice_profile_version, evidence_packet_version, input_hash,
		  output_hash, source_ids, started_by, completed_at, status,
		  input_tokens, output_tokens, estimated_cost_cents
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP,
		          'succeeded', ?, ?, ?)
	`, runID, execution.Job.ProjectID, execution.Job.ContentID, execution.Job.RevisionID,
		execution.Job.ID, execution.Job.Type, execution.Provider, execution.Model,
		execution.Job.PromptTemplateVersion, execution.Job.VoiceProfileVersion,
		execution.Job.EvidencePacketVersion, execution.Job.InputHash, outputHash,
		string(sourceIDsJSON), execution.StartedBy, result.InputTokens, result.OutputTokens,
		result.EstimatedCostCents); err != nil {
		return err
	}
	if execution.Job.Type == "quality_check" {
		if err := insertAIQualityResults(ctx, tx, execution, result.Output); err != nil {
			return err
		}
	}
	if err := appendAIJobEventTx(ctx, tx, execution.Job.ProjectID, execution.Job.ID, "completed", "succeeded", 100, "AI proposal ready for human review", map[string]any{
		"outputHash":         outputHash,
		"inputTokens":        result.InputTokens,
		"outputTokens":       result.OutputTokens,
		"estimatedCostCents": result.EstimatedCostCents,
	}); err != nil {
		return err
	}
	if err := insertAuditEventTx(ctx, tx, execution.Job.ProjectID, "system", workerID, "ai_job.complete", "ai_job", execution.Job.ID, "success", map[string]any{
		"run_id":      runID,
		"output_hash": outputHash,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func validateAIExecutionOutput(taskType string, output json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(output, &object); err != nil || object == nil {
		return fmt.Errorf("%w: AI output must be a JSON object", ErrValidation)
	}
	requireString := func(key string) bool {
		var value string
		return json.Unmarshal(object[key], &value) == nil && strings.TrimSpace(value) != ""
	}
	requireArray := func(key string) bool {
		var value []json.RawMessage
		return json.Unmarshal(object[key], &value) == nil && len(value) > 0
	}
	switch taskType {
	case "outline":
		if !requireString("title") || !requireString("thesis") || !requireArray("sections") {
			return fmt.Errorf("%w: outline output requires title, thesis, and sections", ErrValidation)
		}
	case "draft":
		if !requireString("title") || !requireString("html") || !requireString("markdown") {
			return fmt.Errorf("%w: draft output requires title, HTML, and Markdown", ErrValidation)
		}
	case "quality_check":
		if !requireString("summary") || !requireArray("checks") {
			return fmt.Errorf("%w: quality-check output requires a summary and checks", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: unsupported AI task output", ErrValidation)
	}
	return nil
}

func (s *Store) FailAIJob(ctx context.Context, workerID string, execution AIExecutionJob, category, safeMessage string, retryable bool) error {
	category = strings.TrimSpace(category)
	safeMessage = strings.TrimSpace(safeMessage)
	if category == "" {
		category = "provider_error"
	}
	if len(category) > 120 {
		category = category[:120]
	}
	if len(safeMessage) > 2000 {
		safeMessage = safeMessage[:2000]
	}
	nextStatus := "failed"
	progress := int64(0)
	message := safeMessage
	if retryable && execution.Attempts < 3 {
		nextStatus = "queued"
		message = "AI provider request will be retried"
	}
	if category == "budget_blocked" {
		nextStatus = "budget_blocked"
	}
	if category == "safety_blocked" {
		nextStatus = "safety_blocked"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	completedExpression := "CURRENT_TIMESTAMP"
	if nextStatus == "queued" {
		completedExpression = "NULL"
	}
	query := `
		UPDATE ai_jobs
		SET status = ?, error_category = ?, completed_at = ` + completedExpression + `,
		    locked_by = NULL, locked_until = NULL
		WHERE project_id = ? AND id = ? AND status = 'running' AND locked_by = ?
	`
	updated, err := tx.ExecContext(ctx, query, nextStatus, category, execution.Job.ProjectID, execution.Job.ID, workerID)
	if err != nil {
		return err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}
	if err := appendAIJobEventTx(ctx, tx, execution.Job.ProjectID, execution.Job.ID, "provider_error", nextStatus, progress, message, map[string]any{
		"category":  category,
		"attempt":   execution.Attempts,
		"retryable": nextStatus == "queued",
	}); err != nil {
		return err
	}
	if nextStatus != "queued" {
		runID, err := security.RandomID("airun")
		if err != nil {
			return err
		}
		sourceIDs, err := json.Marshal(evidenceSourceIDs(execution.Snapshot.Evidence.Packet))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ai_runs(
			  id, project_id, content_id, revision_id, job_id, task_type,
			  provider, model_identifier, prompt_template_version,
			  voice_profile_version, evidence_packet_version, input_hash,
			  source_ids, started_by, completed_at, status, error_category
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?)
		`, runID, execution.Job.ProjectID, execution.Job.ContentID, execution.Job.RevisionID,
			execution.Job.ID, execution.Job.Type, execution.Provider, execution.Model,
			execution.Job.PromptTemplateVersion, execution.Job.VoiceProfileVersion,
			execution.Job.EvidencePacketVersion, execution.Job.InputHash, string(sourceIDs),
			execution.StartedBy, nextStatus, category); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func decodeAIJobSnapshot(inputJSON, expectedHash string) (AIJobInputSnapshot, error) {
	var snapshot AIJobInputSnapshot
	if err := json.Unmarshal([]byte(inputJSON), &snapshot); err != nil {
		return AIJobInputSnapshot{}, fmt.Errorf("decode AI job input snapshot: %w", err)
	}
	_, actualHash, err := encodeAIJobSnapshot(snapshot)
	if err != nil {
		return AIJobInputSnapshot{}, err
	}
	if expectedHash == "" || actualHash != expectedHash {
		return AIJobInputSnapshot{}, fmt.Errorf("%w: AI job input snapshot hash does not match", ErrInvalidWorkflow)
	}
	return snapshot, nil
}

func insertAIQualityResults(ctx context.Context, tx *sql.Tx, execution AIExecutionJob, output json.RawMessage) error {
	var quality aiQualityOutput
	if err := json.Unmarshal(output, &quality); err != nil {
		return fmt.Errorf("%w: quality-check output is not valid", ErrValidation)
	}
	if len(quality.Checks) > 100 {
		return fmt.Errorf("%w: quality-check output exceeds 100 checks", ErrValidation)
	}
	for _, check := range quality.Checks {
		check.Type = strings.TrimSpace(check.Type)
		check.Severity = strings.TrimSpace(check.Severity)
		check.Status = strings.TrimSpace(check.Status)
		check.Message = strings.TrimSpace(check.Message)
		if check.Type == "" || len(check.Type) > 120 || check.Message == "" || len(check.Message) > 2000 {
			return fmt.Errorf("%w: quality-check fields are incomplete", ErrValidation)
		}
		if _, ok := allowedQualitySeverities[check.Severity]; !ok {
			return fmt.Errorf("%w: unsupported AI quality-check severity", ErrValidation)
		}
		if check.Status != "passed" && check.Status != "failed" {
			return fmt.Errorf("%w: AI quality-check status must be passed or failed", ErrValidation)
		}
		if len(check.Evidence) == 0 {
			check.Evidence = json.RawMessage(`{}`)
		}
		if !json.Valid(check.Evidence) || len(check.Evidence) > 64*1024 {
			return fmt.Errorf("%w: quality-check evidence is invalid", ErrValidation)
		}
		resultID, err := security.RandomID("quality")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO quality_check_results(
			  id, project_id, content_id, revision_id, check_type,
			  severity, status, message, evidence_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, resultID, execution.Job.ProjectID, execution.Job.ContentID, execution.Job.RevisionID,
			check.Type, check.Severity, check.Status, check.Message, string(check.Evidence)); err != nil {
			return err
		}
	}
	return nil
}

func IsAIExecutionObsolete(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
