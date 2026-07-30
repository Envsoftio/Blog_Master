package httpapi

import (
	"net/http"
	"net/http/httptest"
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
	basePath := "/api/v1/projects/" + project.ID + "/ai/jobs"

	createResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, basePath, `{"type":"outline","brief":{"title":"A useful guide"}}`, login))
	if createResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected AI job create 202, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminAIJob]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Status != "queued" || created.Data.Type != "outline" {
		t.Fatalf("unexpected AI job %#v", created.Data)
	}

	listRequest := httptest.NewRequest(http.MethodGet, basePath, nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
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
