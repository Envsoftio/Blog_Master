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
}
