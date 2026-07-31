package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
)

func TestAIExecutionLeaseCompletionAndQualityIngestion(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "ai-execution.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	snapshot := store.AIJobInputSnapshot{
		SchemaVersion: "ai-job-input-v1",
		TaskType:      "quality_check",
		ArticleType:   "standard",
		Content: store.AIJobContentContext{
			ID: "article", RevisionID: "revision", RevisionHash: "revision-hash",
		},
		Voice: store.AIJobVoiceContext{ID: "voice", Version: 1},
		Evidence: store.AIJobEvidenceContext{
			ID: "evidence", Version: 1,
			Packet: store.EvidencePacketDocument{PublicationRecommendation: "ready"},
		},
	}
	inputJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	inputHashBytes := sha256.Sum256(inputJSON)
	inputHash := fmt.Sprintf("%x", inputHashBytes)
	_, err = db.Exec(`
		INSERT INTO workspaces(id, slug, name) VALUES ('workspace', 'workspace', 'Workspace');
		INSERT INTO users(id, email_normalized, status) VALUES ('user', 'user@example.test', 'active');
		INSERT INTO projects(id, workspace_id, slug, name, public_project_key)
		VALUES ('project', 'workspace', 'project', 'Project', 'public');
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES ('article', 'project', 'standard', 'user');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title, content_hash
		) VALUES ('revision', 'project', 'article', 1, 'human', 'Article', 'revision-hash');
		INSERT INTO voice_profiles(id, project_id, version, profile_json, created_by)
		VALUES ('voice', 'project', 1, '{}', 'user');
		INSERT INTO evidence_packets(
		  id, project_id, content_id, version, packet_json, approved_by, approved_at, created_by
		) VALUES ('evidence', 'project', 'article', 1, '{"publicationRecommendation":"ready"}', 'user', CURRENT_TIMESTAMP, 'user');
		INSERT INTO ai_jobs(
		  id, project_id, content_id, revision_id, task_type, article_type,
		  prompt_template_version, voice_profile_id, voice_profile_version,
		  evidence_packet_id, evidence_packet_version, input_hash, input_json,
		  source_revision_hash, started_by
		) VALUES (
		  'job', 'project', 'article', 'revision', 'quality_check', 'standard',
		  'quality-critique-v1', 'voice', 1, 'evidence', 1, ?, ?, 'revision-hash', 'user'
		);
	`, inputHash, string(inputJSON))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO ai_jobs(
		  id, project_id, content_id, revision_id, task_type, article_type,
		  prompt_template_version, voice_profile_id, voice_profile_version,
		  evidence_packet_id, evidence_packet_version, input_hash, input_json,
		  source_revision_hash, started_by
		) VALUES (
		  'bad-job', 'project', 'article', 'revision', 'quality_check', 'standard',
		  'quality-critique-v1', 'voice', 1, 'evidence', 1, 'tampered-hash', ?, 'revision-hash', 'user'
		)
	`, string(inputJSON))
	if err != nil {
		t.Fatal(err)
	}

	contentStore := store.New(db)
	claimed, err := contentStore.ClaimAIJobs(context.Background(), "worker", "provider", "model", 2*time.Minute, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Attempts != 1 || claimed[0].Snapshot.Content.ID != "article" {
		t.Fatalf("unexpected claim %#v", claimed)
	}
	var badStatus, badCategory string
	if err := db.QueryRow(`SELECT status, error_category FROM ai_jobs WHERE id = 'bad-job'`).Scan(&badStatus, &badCategory); err != nil {
		t.Fatal(err)
	}
	if badStatus != "failed" || badCategory != "invalid_input" {
		t.Fatalf("invalid immutable input was not rejected: status=%q category=%q", badStatus, badCategory)
	}
	if err := contentStore.CompleteAIJob(context.Background(), "worker", claimed[0], store.AIExecutionResult{Output: json.RawMessage(`{"summary":"missing checks"}`)}); err == nil {
		t.Fatal("expected invalid task-specific output to be rejected")
	}
	output := json.RawMessage(`{
		"summary":"Evidence checks complete",
		"checks":[{
			"type":"source_support","severity":"blocking","status":"failed",
			"message":"A material claim needs a source.","evidence":{"blockId":"block-1"}
		}]
	}`)
	if err := contentStore.CompleteAIJob(context.Background(), "worker", claimed[0], store.AIExecutionResult{
		Output: output, InputTokens: 120, OutputTokens: 40, EstimatedCostCents: 2,
	}); err != nil {
		t.Fatal(err)
	}

	var status, storedOutput string
	var inputTokens, outputTokens int
	if err := db.QueryRow(`
		SELECT status, output_json, input_tokens, output_tokens
		FROM ai_jobs WHERE project_id = 'project' AND id = 'job'
	`).Scan(&status, &storedOutput, &inputTokens, &outputTokens); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || storedOutput != string(output) || inputTokens != 120 || outputTokens != 40 {
		t.Fatalf("unexpected completed job status=%q input=%d output=%d body=%s", status, inputTokens, outputTokens, storedOutput)
	}
	var provider, model, runStatus string
	if err := db.QueryRow(`SELECT provider, model_identifier, status FROM ai_runs WHERE job_id = 'job'`).Scan(&provider, &model, &runStatus); err != nil {
		t.Fatal(err)
	}
	if provider != "provider" || model != "model" || runStatus != "succeeded" {
		t.Fatalf("unexpected run provenance %q %q %q", provider, model, runStatus)
	}
	var checkStatus, severity string
	if err := db.QueryRow(`SELECT status, severity FROM quality_check_results WHERE revision_id = 'revision'`).Scan(&checkStatus, &severity); err != nil {
		t.Fatal(err)
	}
	if checkStatus != "failed" || severity != "blocking" {
		t.Fatalf("unexpected quality result %q %q", checkStatus, severity)
	}
	if next, err := contentStore.ClaimAIJobs(context.Background(), "worker", "provider", "model", 2*time.Minute, 2); err != nil || len(next) != 0 {
		t.Fatalf("completed job must not be reclaimed: %#v, %v", next, err)
	}
}
