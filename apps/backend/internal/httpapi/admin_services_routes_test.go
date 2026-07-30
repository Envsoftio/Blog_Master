package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"seoblog/apps/backend/internal/security"
	"seoblog/apps/backend/internal/store"
)

func TestAdminMediaRoutesRequireCSRFAndRemainProjectScoped(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "media-owner@example.test", "correct horse battery staple")
	projectA := createTestProject(t, server, login, `{"slug":"media-a","name":"Media A"}`)
	projectB := createTestProject(t, server, login, `{"slug":"media-b","name":"Media B"}`)
	path := "/api/v1/projects/" + projectA.ID + "/media/uploads"
	body := `{"filename":"hero image.png","contentType":"image/png","bytes":2048}`

	withoutCSRF := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	withoutCSRF.Header.Set("Content-Type", "application/json")
	addCookies(withoutCSRF, login.cookies)
	if response := mustTest(t, server, withoutCSRF); response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected media registration without CSRF to fail with 403, got %d", response.StatusCode)
	}

	createResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, body, login))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media registration 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.ProjectID != projectA.ID || created.Data.Status != "registered" {
		t.Fatalf("unexpected media response %#v", created.Data)
	}
	if !strings.HasPrefix(created.Data.ObjectKey, "pending/"+projectA.ID+"/") {
		t.Fatalf("expected project-scoped pending object key, got %q", created.Data.ObjectKey)
	}

	listA := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectA.ID+"/media", nil)
	addCookies(listA, login.cookies)
	listAResponse := mustTest(t, server, listA)
	var assetsA ListEnvelope[store.AdminMediaAsset]
	decodeJSONResponse(t, listAResponse, &assetsA)
	if len(assetsA.Data) != 1 || assetsA.Data[0].ID != created.Data.ID {
		t.Fatalf("expected created asset in project A, got %#v", assetsA.Data)
	}

	listB := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectB.ID+"/media", nil)
	addCookies(listB, login.cookies)
	listBResponse := mustTest(t, server, listB)
	var assetsB ListEnvelope[store.AdminMediaAsset]
	decodeJSONResponse(t, listBResponse, &assetsB)
	if len(assetsB.Data) != 0 {
		t.Fatalf("expected project B media list to be empty, got %#v", assetsB.Data)
	}
}

func TestAdminAIJobRoutesPersistAndCancelJobs(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "ai-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"ai-project","name":"AI Project"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"ai-jobs","name":"AI Jobs"}`)
	article := createTestArticle(
		t,
		server,
		login,
		project.ID,
		`{"articleType":"guide","title":"AI Job Guide","slug":"ai-job-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Initial AI job source.</p>"}`,
	)
	voiceProfile, evidencePacket := createApprovedAIJobContext(t, server, db, login, project.ID, article.ID)
	basePath := "/api/v1/projects/" + project.ID + "/ai/jobs"
	jobBody := validAIJobRequestBody("outline", article.ID, "guide", evidencePacket.ID, voiceProfile.Version, "A practical angle grounded in the approved evidence packet.")

	createResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, basePath, jobBody, login))
	if createResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected AI job create 202, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminAIJob]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Status != "queued" ||
		created.Data.Type != "outline" ||
		created.Data.RevisionID != article.LatestRevision.ID ||
		created.Data.VoiceProfileVersion != voiceProfile.Version ||
		created.Data.EvidencePacketVersion != evidencePacket.Version ||
		len(created.Data.InputHash) != 64 {
		t.Fatalf("unexpected AI job %#v", created.Data)
	}
	snapshot, err := server.store.GetAIJobInputSnapshot(context.Background(), project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Brief.Title != "A useful evidence-backed guide" ||
		snapshot.Content.RevisionHash != created.Data.SourceRevisionHash ||
		snapshot.Evidence.ID != evidencePacket.ID {
		t.Fatalf("unexpected AI input snapshot %#v", snapshot)
	}
	reusedResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, basePath, jobBody, login))
	if reusedResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected duplicate AI job request 202, got %d", reusedResponse.StatusCode)
	}
	var reused Envelope[store.AdminAIJob]
	decodeJSONResponse(t, reusedResponse, &reused)
	if reused.Data.ID != created.Data.ID || !reused.Data.Reused {
		t.Fatalf("expected matching input to reuse job %q, got %#v", created.Data.ID, reused.Data)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, basePath+"/"+created.Data.ID+"/events", nil)
	addCookies(eventsRequest, login.cookies)
	eventsResponse := mustTest(t, server, eventsRequest)
	if eventsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected AI job events 200, got %d: %s", eventsResponse.StatusCode, readBody(t, eventsResponse))
	}
	if cacheControl := eventsResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store AI events, got %q", cacheControl)
	}
	var createdEvents ListEnvelope[store.AIJobEvent]
	decodeJSONResponse(t, eventsResponse, &createdEvents)
	if len(createdEvents.Data) != 1 ||
		createdEvents.Data[0].Sequence != 1 ||
		createdEvents.Data[0].Status != "queued" ||
		createdEvents.Data[0].Progress != 0 {
		t.Fatalf("unexpected queued event %#v", createdEvents.Data)
	}
	progressEvent, err := server.store.RecordAIJobProgress(context.Background(), project.ID, created.Data.ID, store.AIJobProgressInput{
		Type:     "provider_progress",
		Status:   "running",
		Progress: 40,
		Message:  "Reviewing evidence",
		Metadata: map[string]any{"stage": "evidence"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if progressEvent.Sequence != 2 ||
		progressEvent.Status != "running" ||
		progressEvent.Progress != 40 ||
		!strings.Contains(string(progressEvent.Metadata), "evidence") {
		t.Fatalf("unexpected worker progress event %#v", progressEvent)
	}

	listRequest := httptest.NewRequest(http.MethodGet, basePath, nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if cacheControl := listResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store AI job list, got %q", cacheControl)
	}
	var jobs ListEnvelope[store.AdminAIJob]
	decodeJSONResponse(t, listResponse, &jobs)
	if len(jobs.Data) != 1 || jobs.Data[0].ID != created.Data.ID {
		t.Fatalf("expected created AI job in list, got %#v", jobs.Data)
	}

	cancelResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		basePath+"/"+created.Data.ID+"/cancel",
		`{}`,
		login,
	))
	if cancelResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected cancel 200, got %d: %s", cancelResponse.StatusCode, readBody(t, cancelResponse))
	}
	var cancelled Envelope[store.AdminAIJob]
	decodeJSONResponse(t, cancelResponse, &cancelled)
	if cancelled.Data.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %#v", cancelled.Data)
	}

	cancelledEventsRequest := httptest.NewRequest(http.MethodGet, basePath+"/"+created.Data.ID+"/events?after=1", nil)
	addCookies(cancelledEventsRequest, login.cookies)
	cancelledEventsResponse := mustTest(t, server, cancelledEventsRequest)
	var cancelledEvents ListEnvelope[store.AIJobEvent]
	decodeJSONResponse(t, cancelledEventsResponse, &cancelledEvents)
	if len(cancelledEvents.Data) != 2 ||
		cancelledEvents.Data[0].Sequence != 2 ||
		cancelledEvents.Data[0].Status != "running" ||
		cancelledEvents.Data[1].Sequence != 3 ||
		cancelledEvents.Data[1].Status != "cancelled" {
		t.Fatalf("unexpected cancellation events %#v", cancelledEvents.Data)
	}
	if _, err := server.store.RecordAIJobProgress(context.Background(), project.ID, created.Data.ID, store.AIJobProgressInput{
		Status:   "running",
		Progress: 60,
	}); err == nil {
		t.Fatal("expected a cancelled AI job to reject later worker progress")
	}

	invalidCursorRequest := httptest.NewRequest(http.MethodGet, basePath+"/"+created.Data.ID+"/events?after=-1", nil)
	addCookies(invalidCursorRequest, login.cookies)
	if response := mustTest(t, server, invalidCursorRequest); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected negative AI event cursor to fail with 400, got %d", response.StatusCode)
	}

	otherProject := createTestProject(t, server, login, `{"slug":"other-ai-project","name":"Other AI Project"}`)
	crossProjectRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+otherProject.ID+"/ai/jobs/"+created.Data.ID+"/events",
		nil,
	)
	addCookies(crossProjectRequest, login.cookies)
	if response := mustTest(t, server, crossProjectRequest); response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project AI events to fail with 404, got %d", response.StatusCode)
	}
	if _, err := db.Exec(`UPDATE projects SET status = 'suspended' WHERE id = ?`, otherProject.ID); err != nil {
		t.Fatal(err)
	}
	suspendedCreate := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+otherProject.ID+"/ai/jobs",
		`{"type":"outline"}`,
		login,
	))
	if suspendedCreate.StatusCode != http.StatusConflict {
		t.Fatalf("expected suspended project to reject new AI jobs with 409, got %d", suspendedCreate.StatusCode)
	}
}

func TestAdminAIJobsBindLiveApprovedContextAndRevisionHash(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "ai-context-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"ai-context","name":"AI Context"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"context","name":"Context"}`)
	article := createTestArticle(
		t,
		server,
		login,
		project.ID,
		`{"articleType":"guide","title":"Context Guide","slug":"context-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Original context.</p>"}`,
	)
	voice, approvedEvidence := createApprovedAIJobContext(t, server, db, login, project.ID, article.ID)
	path := "/api/v1/projects/" + project.ID + "/ai/jobs"
	validBody := validAIJobRequestBody(
		"draft",
		article.ID,
		"guide",
		approvedEvidence.ID,
		voice.Version,
		"Use the approved project evidence to make one concrete implementation argument.",
	)

	draftEvidenceResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/evidence-packets",
		validEvidencePacketBody(article.ID, "source-"+article.ID, "A newer evidence thesis that has not been approved yet."),
		login,
	))
	if draftEvidenceResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected draft evidence create 201, got %d: %s", draftEvidenceResponse.StatusCode, readBody(t, draftEvidenceResponse))
	}
	var draftEvidence Envelope[store.EvidencePacket]
	decodeJSONResponse(t, draftEvidenceResponse, &draftEvidence)
	unapprovedResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validAIJobRequestBody(
			"draft",
			article.ID,
			"guide",
			draftEvidence.Data.ID,
			voice.Version,
			"Use the pending packet even though a reviewer has not approved its factual basis.",
		),
		login,
	))
	if unapprovedResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected unapproved evidence to fail with 409, got %d", unapprovedResponse.StatusCode)
	}

	otherProject := createTestProject(t, server, login, `{"slug":"other-ai-context","name":"Other AI Context"}`)
	otherCategory := createTestCategory(t, server, login, otherProject.ID, `{"slug":"other-context","name":"Other Context"}`)
	otherArticle := createTestArticle(
		t,
		server,
		login,
		otherProject.ID,
		`{"articleType":"guide","title":"Other Context Guide","slug":"other-context-guide","primaryCategoryId":"`+otherCategory.ID+`","html":"<p>Other context.</p>"}`,
	)
	_, otherEvidence := createApprovedAIJobContext(t, server, db, login, otherProject.ID, otherArticle.ID)
	crossProjectResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validAIJobRequestBody(
			"draft",
			article.ID,
			"guide",
			otherEvidence.ID,
			voice.Version,
			"Attempt to bind evidence owned by another project into this draft request.",
		),
		login,
	))
	if crossProjectResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project evidence to fail with 400, got %d", crossProjectResponse.StatusCode)
	}

	missingVoiceResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validAIJobRequestBody(
			"draft",
			article.ID,
			"guide",
			approvedEvidence.ID,
			voice.Version+99,
			"Attempt to bind a voice profile version that does not exist for this project.",
		),
		login,
	))
	if missingVoiceResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected missing voice version to fail with 409, got %d", missingVoiceResponse.StatusCode)
	}

	if _, err := db.Exec(`DELETE FROM sources WHERE project_id = ? AND id = ?`, project.ID, "source-"+article.ID); err != nil {
		t.Fatal(err)
	}
	staleEvidenceResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, validBody, login))
	if staleEvidenceResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected stale evidence sources to fail with 400, got %d", staleEvidenceResponse.StatusCode)
	}
	if _, err := db.Exec(`
		INSERT INTO sources(id, project_id, title, url, source_type, is_primary)
		VALUES (?, ?, 'AI job evidence', 'https://example.test/ai-job-evidence', 'web', 1)
	`, "source-"+article.ID, project.ID); err != nil {
		t.Fatal(err)
	}

	createResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, validBody, login))
	if createResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected context-bound job 202, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminAIJob]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.RevisionID != article.LatestRevision.ID ||
		created.Data.SourceRevisionHash != article.LatestRevision.ContentHash ||
		created.Data.PromptTemplateVersion != "section-draft-v1" ||
		created.Data.EvidencePacketID != approvedEvidence.ID ||
		created.Data.VoiceProfileID != voice.ID {
		t.Fatalf("unexpected bound AI job %#v", created.Data)
	}
	if _, err := server.store.RecordAIJobProgress(context.Background(), project.ID, created.Data.ID, store.AIJobProgressInput{
		Type:     "started",
		Status:   "running",
		Progress: 10,
		Message:  "Validating the bound context",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.store.RecordAIJobProgress(context.Background(), project.ID, created.Data.ID, store.AIJobProgressInput{
		Type:     "needs_input",
		Status:   "needs_input",
		Progress: 15,
		Message:  "Waiting for a subject-matter detail",
	}); err != nil {
		t.Fatal(err)
	}
	needsInputReuseResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, validBody, login))
	var needsInputReuse Envelope[store.AdminAIJob]
	decodeJSONResponse(t, needsInputReuseResponse, &needsInputReuse)
	if needsInputReuseResponse.StatusCode != http.StatusAccepted ||
		needsInputReuse.Data.ID != created.Data.ID ||
		!needsInputReuse.Data.Reused {
		t.Fatalf("expected needs-input job to be reused, got %#v", needsInputReuse.Data)
	}

	changedBriefResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validAIJobRequestBody(
			"draft",
			article.ID,
			"guide",
			approvedEvidence.ID,
			voice.Version,
			"A materially different angle should produce a distinct canonical input hash.",
		),
		login,
	))
	var changedBrief Envelope[store.AdminAIJob]
	decodeJSONResponse(t, changedBriefResponse, &changedBrief)
	if changedBriefResponse.StatusCode != http.StatusAccepted ||
		changedBrief.Data.ID == created.Data.ID ||
		changedBrief.Data.InputHash == created.Data.InputHash {
		t.Fatalf("expected changed brief to create a distinct job, got %#v", changedBrief.Data)
	}

	newRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Context Guide Updated",
		"html":"<p>Updated source context for a new AI request.</p>"
	}`)
	newRevisionResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, validBody, login))
	var revisionBound Envelope[store.AdminAIJob]
	decodeJSONResponse(t, newRevisionResponse, &revisionBound)
	if newRevisionResponse.StatusCode != http.StatusAccepted ||
		revisionBound.Data.ID == created.Data.ID ||
		revisionBound.Data.RevisionID != newRevision.ID ||
		revisionBound.Data.SourceRevisionHash != newRevision.ContentHash ||
		revisionBound.Data.InputHash == created.Data.InputHash {
		t.Fatalf("expected new revision to change the AI job binding, got %#v", revisionBound.Data)
	}

	if _, err := db.Exec(`
		UPDATE ai_jobs
		SET input_json = '{}'
		WHERE project_id = ? AND id = ?
	`, project.ID, created.Data.ID); err == nil {
		t.Fatal("expected persisted AI job inputs to be immutable")
	}
	if _, err := db.Exec(`
		INSERT INTO ai_jobs(
		  id, project_id, content_id, revision_id, task_type, article_type, status,
		  prompt_template_version, voice_profile_id, voice_profile_version,
		  evidence_packet_id, evidence_packet_version, input_hash, input_json,
		  source_revision_hash, started_by
		) VALUES (
		  'mismatched-article-type-job', ?, ?, ?, 'outline', 'comparison', 'queued',
		  'outline-v1', ?, ?, ?, ?, 'mismatched-article-type-hash', '{}', ?, ?
		)
	`, project.ID, article.ID, newRevision.ID, voice.ID, voice.Version,
		approvedEvidence.ID, approvedEvidence.Version, newRevision.ContentHash, login.userID); err == nil {
		t.Fatal("expected the database to reject an AI job with a mismatched article type")
	}
	if _, err := db.Exec(`
		INSERT INTO ai_jobs(
		  id, project_id, content_id, revision_id, task_type, article_type, status,
		  prompt_template_version, voice_profile_id, voice_profile_version,
		  evidence_packet_id, evidence_packet_version, input_hash, input_json,
		  source_revision_hash, started_by
		) VALUES (
		  'cross-context-job', ?, ?, ?, 'outline', 'guide', 'queued',
		  'outline-v1', ?, ?, ?, ?, 'cross-context-hash', '{}', ?, ?
		)
	`, project.ID, article.ID, newRevision.ID, voice.ID, voice.Version,
		otherEvidence.ID, otherEvidence.Version, newRevision.ContentHash, login.userID); err == nil {
		t.Fatal("expected the database to reject a cross-project evidence binding")
	}
}

func TestAdminAIProvenanceQualityResultsAndApprovalGate(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "ai-reviewer@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"ai-review","name":"AI Review"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"research","name":"Research"}`)
	article := createTestArticle(
		t,
		server,
		login,
		project.ID,
		`{"articleType":"guide","title":"Reviewed Guide","slug":"reviewed-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Reviewed material.</p>"}`,
	)
	revisionID := article.LatestRevision.ID
	voiceProfile, evidencePacket := createApprovedAIJobContext(t, server, db, login, project.ID, article.ID)
	jobPath := "/api/v1/projects/" + project.ID + "/ai/jobs"
	jobResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		jobPath,
		validAIJobRequestBody(
			"quality_check",
			article.ID,
			"guide",
			evidencePacket.ID,
			voiceProfile.Version,
			"Review factual support and voice alignment before human approval.",
		),
		login,
	))
	if jobResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected quality job create 202, got %d: %s", jobResponse.StatusCode, readBody(t, jobResponse))
	}
	var job Envelope[store.AdminAIJob]
	decodeJSONResponse(t, jobResponse, &job)

	if _, err := db.Exec(`
		INSERT INTO ai_runs(
		  id, project_id, content_id, revision_id, job_id, task_type,
		  provider, model_identifier, prompt_template_version, input_hash,
		  output_hash, source_ids, started_by, started_at, completed_at,
		  status, input_tokens, output_tokens, estimated_cost_cents
		) VALUES (?, ?, ?, ?, ?, 'quality_check', 'test-provider', 'quality-model-v1',
		          'quality-v3', 'input-hash-new', 'output-hash-new', '["source-1"]',
		          ?, '2026-07-30 10:05:00', '2026-07-30 10:06:00',
		          'succeeded', 120, 45, 3)
	`, "run-new", project.ID, article.ID, revisionID, job.Data.ID, login.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO ai_runs(
		  id, project_id, content_id, revision_id, job_id, task_type,
		  provider, model_identifier, prompt_template_version, input_hash,
		  source_ids, started_by, started_at, status
		) VALUES (?, ?, ?, ?, ?, 'quality_check', 'test-provider', 'quality-model-v0',
		          'quality-v2', 'input-hash-old', '[]', ?,
		          '2026-07-30 10:00:00', 'failed')
	`, "run-old", project.ID, article.ID, revisionID, job.Data.ID, login.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO quality_check_results(
		  id, project_id, content_id, revision_id, check_type,
		  severity, status, message, evidence_json, created_at
		) VALUES (?, ?, ?, ?, 'source_coverage', 'blocking', 'failed',
		          'A material claim has no source.', '{"claimId":"claim-1"}',
		          '2026-07-30 10:06:00')
	`, "quality-failed", project.ID, article.ID, revisionID); err != nil {
		t.Fatal(err)
	}

	runsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/ai/runs?limit=1", nil)
	addCookies(runsRequest, login.cookies)
	runsResponse := mustTest(t, server, runsRequest)
	if runsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected AI runs 200, got %d: %s", runsResponse.StatusCode, readBody(t, runsResponse))
	}
	if cacheControl := runsResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store AI runs, got %q", cacheControl)
	}
	var firstRuns ListEnvelope[store.AIRun]
	decodeJSONResponse(t, runsResponse, &firstRuns)
	if len(firstRuns.Data) != 1 ||
		firstRuns.Data[0].ID != "run-new" ||
		firstRuns.Data[0].ModelIdentifier != "quality-model-v1" ||
		len(firstRuns.Data[0].SourceIDs) != 1 ||
		firstRuns.Meta.NextCursor == "" {
		t.Fatalf("unexpected first AI run page %#v", firstRuns)
	}

	secondRunsRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/ai/runs?limit=1&cursor="+firstRuns.Meta.NextCursor,
		nil,
	)
	addCookies(secondRunsRequest, login.cookies)
	secondRunsResponse := mustTest(t, server, secondRunsRequest)
	var secondRuns ListEnvelope[store.AIRun]
	decodeJSONResponse(t, secondRunsResponse, &secondRuns)
	if len(secondRuns.Data) != 1 || secondRuns.Data[0].ID != "run-old" {
		t.Fatalf("expected stable AI run pagination, got %#v", secondRuns.Data)
	}

	invalidRunStatusRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/ai/runs?status=published",
		nil,
	)
	addCookies(invalidRunStatusRequest, login.cookies)
	if response := mustTest(t, server, invalidRunStatusRequest); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid AI run status to fail with 400, got %d", response.StatusCode)
	}

	checksRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/quality-checks?revisionId="+revisionID,
		nil,
	)
	addCookies(checksRequest, login.cookies)
	checksResponse := mustTest(t, server, checksRequest)
	if checksResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected quality checks 200, got %d: %s", checksResponse.StatusCode, readBody(t, checksResponse))
	}
	if cacheControl := checksResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store quality checks, got %q", cacheControl)
	}
	var checks ListEnvelope[store.QualityCheckResult]
	decodeJSONResponse(t, checksResponse, &checks)
	if len(checks.Data) != 1 ||
		checks.Data[0].Status != "failed" ||
		checks.Data[0].Severity != "blocking" ||
		!strings.Contains(string(checks.Data[0].Evidence), "claim-1") {
		t.Fatalf("unexpected quality-check results %#v", checks.Data)
	}

	if _, err := db.Exec(`
		INSERT INTO quality_check_results(
		  id, project_id, content_id, revision_id, check_type,
		  severity, status, message, created_at
		) VALUES
		  ('quality-policy-failed', ?, ?, ?, 'editorial_policy', 'critical', 'failed',
		   'A prohibited claim was detected.', '2026-07-30 10:06:00'),
		  ('quality-policy-passed', ?, ?, ?, 'editorial_policy', 'critical', 'passed',
		   'No prohibited claims remain.', '2026-07-30 10:06:00')
	`, project.ID, article.ID, revisionID, project.ID, article.ID, revisionID); err != nil {
		t.Fatal(err)
	}
	latestChecksRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/quality-checks?revisionId="+revisionID+"&limit=1",
		nil,
	)
	addCookies(latestChecksRequest, login.cookies)
	latestChecksResponse := mustTest(t, server, latestChecksRequest)
	var latestChecks ListEnvelope[store.QualityCheckResult]
	decodeJSONResponse(t, latestChecksResponse, &latestChecks)
	if len(latestChecks.Data) != 1 ||
		latestChecks.Data[0].ID != "quality-policy-passed" ||
		latestChecks.Meta.NextCursor == "" {
		t.Fatalf("expected same-second quality results to use insertion order, got %#v", latestChecks)
	}

	blockedApproval := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/approve",
		`{}`,
		login,
	))
	if blockedApproval.StatusCode != http.StatusConflict {
		t.Fatalf("expected failed blocking quality check to prevent approval, got %d: %s", blockedApproval.StatusCode, readBody(t, blockedApproval))
	}

	if _, err := db.Exec(`
		INSERT INTO quality_check_results(
		  id, project_id, content_id, revision_id, check_type,
		  severity, status, message, evidence_json, created_at
		) VALUES (?, ?, ?, ?, 'source_coverage', 'blocking', 'passed',
		          'All material claims have sources.', '{}',
		          '2026-07-30 10:07:00')
	`, "quality-passed", project.ID, article.ID, revisionID); err != nil {
		t.Fatal(err)
	}
	approved := approveTestRevision(t, server, login, project.ID, revisionID)
	if approved.EditorialState != "approved" {
		t.Fatalf("expected newer passing check to allow approval, got %#v", approved)
	}

	otherProject := createTestProject(t, server, login, `{"slug":"ai-review-other","name":"AI Review Other"}`)
	if _, err := db.Exec(`
		INSERT INTO quality_check_results(
		  id, project_id, content_id, revision_id, check_type,
		  severity, status, message
		) VALUES ('cross-project-check', ?, ?, ?, 'scope', 'critical', 'failed', 'invalid')
	`, otherProject.ID, article.ID, revisionID); err == nil {
		t.Fatal("expected database guard to reject cross-project quality-check ownership")
	}

	otherProjectRunsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+otherProject.ID+"/ai/runs", nil)
	addCookies(otherProjectRunsRequest, login.cookies)
	otherProjectRunsResponse := mustTest(t, server, otherProjectRunsRequest)
	var otherProjectRuns ListEnvelope[store.AIRun]
	decodeJSONResponse(t, otherProjectRunsResponse, &otherProjectRuns)
	if len(otherProjectRuns.Data) != 0 {
		t.Fatalf("expected project-scoped AI run history, got %#v", otherProjectRuns.Data)
	}
}

func TestAdminWebhookRoutesHashSecretAndReportDeliveryStatus(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "webhook-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"webhook-project","name":"Webhook Project"}`)
	basePath := "/api/v1/projects/" + project.ID + "/webhooks"

	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		basePath,
		`{"name":"Production","url":"https://example.test/api/revalidate","events":["content.published","content.updated"]}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected webhook create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.WebhookWithSecret]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Secret == "" || created.Data.Status != "active" {
		t.Fatalf("expected active endpoint and one-time secret, got %#v", created.Data)
	}
	var storedHash string
	if err := db.QueryRow(`SELECT secret_hash FROM webhook_endpoints WHERE project_id = ? AND id = ?`, project.ID, created.Data.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash == created.Data.Secret || storedHash != security.TokenHash(created.Data.Secret) {
		t.Fatal("expected only the webhook secret hash to be stored")
	}

	listRequest := httptest.NewRequest(http.MethodGet, basePath, nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	var endpoints ListEnvelope[store.WebhookEndpoint]
	decodeJSONResponse(t, listResponse, &endpoints)
	if len(endpoints.Data) != 1 || len(endpoints.Data[0].Events) != 2 {
		t.Fatalf("unexpected webhook list %#v", endpoints.Data)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/delivery/status", nil)
	addCookies(statusRequest, login.cookies)
	statusResponse := mustTest(t, server, statusRequest)
	var delivery Envelope[store.DeliveryStatus]
	decodeJSONResponse(t, statusResponse, &delivery)
	if delivery.Data.Endpoints != 1 || delivery.Data.Active != 1 || delivery.Data.Failures != 0 {
		t.Fatalf("unexpected delivery status %#v", delivery.Data)
	}

	if _, err := db.Exec(`
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key, processed_at, attempts
		) VALUES (?, ?, 'content.published', 'content', ?, '{}', ?, CURRENT_TIMESTAMP, 3)
	`, "event-failed", project.ID, "article-failed", "webhook-failed-event"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (?, ?, 'content.updated', 'content', ?, '{}', ?)
	`, "event-succeeded", project.ID, "article-succeeded", "webhook-succeeded-event"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status, status_code, error_category
		) VALUES (?, ?, ?, ?, 'failed', 500, 'timeout')
	`, "attempt-failed", project.ID, created.Data.ID, "event-failed"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO webhook_attempts(
		  id, project_id, endpoint_id, outbox_event_id, status, status_code
		) VALUES (?, ?, ?, ?, 'succeeded', 204)
	`, "attempt-succeeded", project.ID, created.Data.ID, "event-succeeded"); err != nil {
		t.Fatal(err)
	}

	attemptsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/webhook-attempts", nil)
	addCookies(attemptsRequest, login.cookies)
	attemptsResponse := mustTest(t, server, attemptsRequest)
	if attemptsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected webhook attempts list 200, got %d: %s", attemptsResponse.StatusCode, readBody(t, attemptsResponse))
	}
	var attempts ListEnvelope[store.WebhookAttempt]
	decodeJSONResponse(t, attemptsResponse, &attempts)
	if !hasWebhookAttempt(attempts.Data, "attempt-failed", "failed") || !hasWebhookAttempt(attempts.Data, "attempt-succeeded", "succeeded") {
		t.Fatalf("expected seeded webhook attempts in list, got %#v", attempts.Data)
	}

	firstPageRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/webhook-attempts?limit=1", nil)
	addCookies(firstPageRequest, login.cookies)
	firstPageResponse := mustTest(t, server, firstPageRequest)
	if firstPageResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected first attempt page 200, got %d: %s", firstPageResponse.StatusCode, readBody(t, firstPageResponse))
	}
	var firstPage ListEnvelope[store.WebhookAttempt]
	decodeJSONResponse(t, firstPageResponse, &firstPage)
	if len(firstPage.Data) != 1 || firstPage.Meta.NextCursor == "" {
		t.Fatalf("expected first page with next cursor, got %#v", firstPage)
	}
	secondPageRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/webhook-attempts?limit=1&cursor="+firstPage.Meta.NextCursor, nil)
	addCookies(secondPageRequest, login.cookies)
	secondPageResponse := mustTest(t, server, secondPageRequest)
	if secondPageResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected second attempt page 200, got %d: %s", secondPageResponse.StatusCode, readBody(t, secondPageResponse))
	}
	var secondPage ListEnvelope[store.WebhookAttempt]
	decodeJSONResponse(t, secondPageResponse, &secondPage)
	if len(secondPage.Data) != 1 || secondPage.Data[0].ID == firstPage.Data[0].ID {
		t.Fatalf("expected cursor to return next webhook attempt, first=%#v second=%#v", firstPage.Data, secondPage.Data)
	}

	replayRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/webhook-attempts/attempt-failed/replay",
		`{}`,
		login,
	)
	replayResponse := mustTest(t, server, replayRequest)
	if replayResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected webhook replay 202, got %d: %s", replayResponse.StatusCode, readBody(t, replayResponse))
	}
	var replayed Envelope[store.WebhookAttempt]
	decodeJSONResponse(t, replayResponse, &replayed)
	if replayed.Data.ID == "" || replayed.Data.ID == "attempt-failed" || replayed.Data.Status != "queued" || replayed.Data.OutboxEventID != "event-failed" {
		t.Fatalf("unexpected replayed attempt %#v", replayed.Data)
	}
	var processedAt string
	if err := db.QueryRow(`SELECT COALESCE(processed_at, '') FROM outbox_events WHERE project_id = ? AND id = ?`, project.ID, "event-failed").Scan(&processedAt); err != nil {
		t.Fatal(err)
	}
	if processedAt != "" {
		t.Fatalf("expected replay to requeue outbox event, processed_at=%q", processedAt)
	}

	replaySucceeded := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/webhook-attempts/attempt-succeeded/replay",
		`{}`,
		login,
	)
	replaySucceededResponse := mustTest(t, server, replaySucceeded)
	if replaySucceededResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected succeeded webhook replay to fail with 409, got %d: %s", replaySucceededResponse.StatusCode, readBody(t, replaySucceededResponse))
	}

	projectB := createTestProject(t, server, login, `{"slug":"webhook-project-b","name":"Webhook Project B"}`)
	crossProjectReplay := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectB.ID+"/webhook-attempts/attempt-failed/replay",
		`{}`,
		login,
	)
	crossProjectReplayResponse := mustTest(t, server, crossProjectReplay)
	if crossProjectReplayResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project webhook replay to return 404, got %d: %s", crossProjectReplayResponse.StatusCode, readBody(t, crossProjectReplayResponse))
	}

	statusAfterReplayRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/delivery/status", nil)
	addCookies(statusAfterReplayRequest, login.cookies)
	statusAfterReplayResponse := mustTest(t, server, statusAfterReplayRequest)
	if statusAfterReplayResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected delivery status after replay 200, got %d: %s", statusAfterReplayResponse.StatusCode, readBody(t, statusAfterReplayResponse))
	}
	var deliveryAfterReplay Envelope[store.DeliveryStatus]
	decodeJSONResponse(t, statusAfterReplayResponse, &deliveryAfterReplay)
	if deliveryAfterReplay.Data.Pending != 1 || deliveryAfterReplay.Data.Failures != 1 || deliveryAfterReplay.Data.Succeeded != 1 {
		t.Fatalf("unexpected delivery status after replay %#v", deliveryAfterReplay.Data)
	}
}

func hasWebhookAttempt(attempts []store.WebhookAttempt, attemptID, status string) bool {
	for _, attempt := range attempts {
		if attempt.ID == attemptID && attempt.Status == status {
			return true
		}
	}
	return false
}

func createApprovedAIJobContext(
	t *testing.T,
	server *Server,
	db *sql.DB,
	login adminLoginResult,
	projectID, articleID string,
) (store.VoiceProfile, store.EvidencePacket) {
	t.Helper()
	voiceResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectID+"/voice-profile",
		validVoiceProfileBody("en", "Direct, exact, and practical"),
		login,
	))
	if voiceResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected AI test voice profile 201, got %d: %s", voiceResponse.StatusCode, readBody(t, voiceResponse))
	}
	var voice Envelope[store.VoiceProfile]
	decodeJSONResponse(t, voiceResponse, &voice)

	sourceID := "source-" + articleID
	if _, err := db.Exec(`
		INSERT INTO sources(id, project_id, title, url, source_type, is_primary)
		VALUES (?, ?, 'AI job evidence', 'https://example.test/ai-job-evidence', 'web', 1)
	`, sourceID, projectID); err != nil {
		t.Fatal(err)
	}
	evidenceResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectID+"/evidence-packets",
		validEvidencePacketBody(articleID, sourceID, "An evidence-backed thesis for a bounded AI job request."),
		login,
	))
	if evidenceResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected AI test evidence packet 201, got %d: %s", evidenceResponse.StatusCode, readBody(t, evidenceResponse))
	}
	var evidence Envelope[store.EvidencePacket]
	decodeJSONResponse(t, evidenceResponse, &evidence)

	approvalResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectID+"/evidence-packets/"+evidence.Data.ID+"/approve",
		`{}`,
		login,
	))
	if approvalResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected AI test evidence approval 200, got %d: %s", approvalResponse.StatusCode, readBody(t, approvalResponse))
	}
	decodeJSONResponse(t, approvalResponse, &evidence)
	return voice.Data, evidence.Data
}

func validAIJobRequestBody(
	taskType, articleID, articleType, evidencePacketID string,
	voiceProfileVersion int64,
	uniqueAngle string,
) string {
	return `{
		"type":"` + taskType + `",
		"contentId":"` + articleID + `",
		"articleType":"` + articleType + `",
		"evidencePacketId":"` + evidencePacketID + `",
		"voiceProfileVersion":` + strconv.FormatInt(voiceProfileVersion, 10) + `,
		"brief":{
			"title":"A useful evidence-backed guide",
			"purpose":"Help implementation teams make a documented and defensible decision.",
			"audience":"Product and engineering teams",
			"uniqueAngle":"` + uniqueAngle + `",
			"evidence":"Use only the approved packet and clearly identify its limitations.",
			"cta":"Review the evidence before accepting the proposed output."
		}
	}`
}
