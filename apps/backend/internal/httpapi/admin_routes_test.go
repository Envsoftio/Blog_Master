package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/security"
	"seoblog/apps/backend/internal/store"
)

func TestAdminLoginMeAndProjectCreate(t *testing.T) {
	server, db := newAdminTestServer(t)
	password := "correct horse battery staple"
	seedOwner(t, db, "owner@example.test", password)

	login := adminLogin(t, server, "OWNER@example.test", password)

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, login.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected /auth/me 200, got %d", meResponse.StatusCode)
	}

	body := strings.NewReader(`{"slug":"Demo Project","name":"Demo Project","primaryDomain":"example.test","supportedLocales":["en","fr"]}`)
	createWithoutCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	createWithoutCSRF.Header.Set("Content-Type", "application/json")
	addCookies(createWithoutCSRF, login.cookies)
	missingCSRFResponse := mustTest(t, server, createWithoutCSRF)
	if missingCSRFResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected create without CSRF to fail with 403, got %d", missingCSRFResponse.StatusCode)
	}

	body = strings.NewReader(`{"slug":"Demo Project","name":"Demo Project","primaryDomain":"example.test","supportedLocales":["en","fr"]}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(createRequest, login.cookies)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected create project 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}

	var created Envelope[store.AdminProject]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Slug != "demo-project" {
		t.Fatalf("expected slug to be normalized, got %q", created.Data.Slug)
	}
	if created.Data.Role != "project_owner" {
		t.Fatalf("expected creator owner membership, got %q", created.Data.Role)
	}
	if created.Data.PublicProjectKey == "" {
		t.Fatal("expected public project key")
	}

	var membershipRole string
	if err := db.QueryRow(`
		SELECT role
		FROM project_memberships
		WHERE project_id = ? AND user_id = ?
	`, created.Data.ID, login.userID).Scan(&membershipRole); err != nil {
		t.Fatal(err)
	}
	if membershipRole != "project_owner" {
		t.Fatalf("expected project_owner membership, got %q", membershipRole)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected project list 200, got %d", listResponse.StatusCode)
	}
	var list ListEnvelope[store.AdminProject]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 1 || list.Data[0].ID != created.Data.ID {
		t.Fatalf("expected project list to include created project, got %#v", list.Data)
	}
}

func TestAdminProjectAccessIsMembershipScoped(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	otherLogin := seedAndLogin(t, server, db, "other@example.test", "another correct horse battery staple")

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"slug":"private","name":"Private Project"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-CSRF-Token", ownerLogin.csrfToken)
	addCookies(createRequest, ownerLogin.cookies)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected create project 201, got %d", createResponse.StatusCode)
	}
	var created Envelope[store.AdminProject]
	decodeJSONResponse(t, createResponse, &created)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.Data.ID, nil)
	addCookies(getRequest, otherLogin.cookies)
	getResponse := mustTest(t, server, getRequest)
	if getResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected non-member project lookup to return 404, got %d", getResponse.StatusCode)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	addCookies(listRequest, otherLogin.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listResponse.StatusCode)
	}
	var list ListEnvelope[store.AdminProject]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 0 {
		t.Fatalf("expected non-member project list to be empty, got %#v", list.Data)
	}
}

func TestAdminLogoutRevokesSession(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(logoutRequest, login.cookies)
	logoutResponse := mustTest(t, server, logoutRequest)
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected logout 204, got %d", logoutResponse.StatusCode)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, login.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked session to fail with 401, got %d", meResponse.StatusCode)
	}
}

func TestProjectAPIKeyLifecycleAndContentAuth(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"keys","name":"Keys Project"}`)
	_ = createTestCategory(t, server, login, project.ID, `{"slug":"docs","name":"Docs"}`)

	first := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"production build"}`)
	if first.Data.Secret == "" {
		t.Fatal("expected raw API key secret at creation")
	}
	if !strings.HasPrefix(first.Data.Secret, first.Data.Key.TokenPrefix) {
		t.Fatalf("expected token prefix to match secret prefix")
	}

	var storedHash string
	if err := db.QueryRow(`SELECT token_hash FROM project_api_keys WHERE id = ?`, first.Data.Key.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash == "" || storedHash == first.Data.Secret {
		t.Fatalf("expected only a verifier to be stored, got %q", storedHash)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/api-keys", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	listBody := readBody(t, listResponse)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected API key list 200, got %d: %s", listResponse.StatusCode, listBody)
	}
	if strings.Contains(listBody, first.Data.Secret) || strings.Contains(listBody, `"secret"`) {
		t.Fatalf("expected list response to omit raw secrets, got %s", listBody)
	}

	assertContentCategoriesStatus(t, server, first.Data.Secret, http.StatusOK)

	rotatedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/api-keys/"+first.Data.Key.ID+"/rotate", nil)
	rotatedRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(rotatedRequest, login.cookies)
	rotatedResponse := mustTest(t, server, rotatedRequest)
	if rotatedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected rotate key 200, got %d: %s", rotatedResponse.StatusCode, readBody(t, rotatedResponse))
	}
	var rotated Envelope[store.APIKeyWithSecret]
	decodeJSONResponse(t, rotatedResponse, &rotated)
	if rotated.Data.Secret == "" || rotated.Data.Secret == first.Data.Secret {
		t.Fatal("expected rotation to return a new one-time secret")
	}
	if rotated.Data.Key.ID == first.Data.Key.ID {
		t.Fatal("expected rotation to create a replacement key")
	}
	assertContentCategoriesStatus(t, server, first.Data.Secret, http.StatusOK)
	assertContentCategoriesStatus(t, server, rotated.Data.Secret, http.StatusOK)

	second := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"production runtime"}`)
	assertContentCategoriesStatus(t, server, second.Data.Secret, http.StatusOK)

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/api-keys/"+first.Data.Key.ID+"/revoke", nil)
	revokeRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(revokeRequest, login.cookies)
	revokeResponse := mustTest(t, server, revokeRequest)
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke key 200, got %d: %s", revokeResponse.StatusCode, readBody(t, revokeResponse))
	}
	var revoked Envelope[store.AdminAPIKey]
	decodeJSONResponse(t, revokeResponse, &revoked)
	if revoked.Data.RevokedAt == "" {
		t.Fatal("expected revoked key response to include revokedAt")
	}
	assertContentCategoriesStatus(t, server, first.Data.Secret, http.StatusUnauthorized)
	assertContentCategoriesStatus(t, server, rotated.Data.Secret, http.StatusOK)
	assertContentCategoriesStatus(t, server, second.Data.Secret, http.StatusOK)

	var rotationAuditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND action = 'api_key.rotate'
		  AND target_id = ?
		  AND json_extract(metadata_json, '$.replacesKeyId') = ?
	`, project.ID, rotated.Data.Key.ID, first.Data.Key.ID).Scan(&rotationAuditCount); err != nil {
		t.Fatal(err)
	}
	if rotationAuditCount != 1 {
		t.Fatalf("expected one rotation audit event, got %d", rotationAuditCount)
	}
}

func TestWriterCannotManageProjectAPIKeys(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"writer-denied","name":"Writer Denied"}`)

	_, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/api-keys", strings.NewReader(`{"environment":"production","name":"should fail"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", writerLogin.csrfToken)
	addCookies(request, writerLogin.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer key creation to fail with 403, got %d: %s", response.StatusCode, readBody(t, response))
	}
}

func TestProjectAPIKeyMutationsRequireRecentReauthentication(t *testing.T) {
	server, db := newAdminTestServer(t)
	password := "correct horse battery staple"
	login := seedAndLogin(t, server, db, "owner@example.test", password)
	project := createTestProject(t, server, login, `{"slug":"reauth","name":"Reauthentication"}`)
	existing := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"existing"}`)

	if _, err := db.Exec(`
		UPDATE sessions
		SET reauthenticated_at = datetime(CURRENT_TIMESTAMP, '-10 minutes')
		WHERE user_id = ?
	`, login.userID); err != nil {
		t.Fatal(err)
	}

	createRequest := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"requires reauthentication"}`,
		login,
	)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected stale reauthentication to fail with 403, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	for _, path := range []string{
		"/api/v1/projects/" + project.ID + "/api-keys/" + existing.Data.Key.ID + "/rotate",
		"/api/v1/projects/" + project.ID + "/api-keys/" + existing.Data.Key.ID + "/revoke",
	} {
		request := newAPIKeyMutationRequest(t, http.MethodPost, path, `{}`, login)
		response := mustTest(t, server, request)
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("expected stale reauthentication for %s to fail with 403, got %d: %s", path, response.StatusCode, readBody(t, response))
		}
	}
	assertContentCategoriesStatus(t, server, existing.Data.Secret, http.StatusOK)

	badReauth := newAPIKeyMutationRequest(t, http.MethodPost, "/api/v1/auth/reauthenticate", `{"password":"incorrect"}`, login)
	badReauthResponse := mustTest(t, server, badReauth)
	if badReauthResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected incorrect password to fail with 401, got %d", badReauthResponse.StatusCode)
	}

	reauth := newAPIKeyMutationRequest(t, http.MethodPost, "/api/v1/auth/reauthenticate", `{"password":"`+password+`"}`, login)
	reauthResponse := mustTest(t, server, reauth)
	if reauthResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected reauthentication 200, got %d: %s", reauthResponse.StatusCode, readBody(t, reauthResponse))
	}

	createRequest = newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"reauthenticated"}`,
		login,
	)
	createResponse = mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected reauthenticated key creation 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
}

func TestExpiredProjectAPIKeyCannotBeCreatedOrRotated(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"expired-key","name":"Expired Key"}`)

	expiredCreate := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"already expired","expiresAt":"2000-01-01T00:00:00Z"}`,
		login,
	)
	expiredCreateResponse := mustTest(t, server, expiredCreate)
	if expiredCreateResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected past expiry to fail with 400, got %d: %s", expiredCreateResponse.StatusCode, readBody(t, expiredCreateResponse))
	}

	key := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"expired"}`)
	if _, err := db.Exec(`
		UPDATE project_api_keys
		SET expires_at = datetime(CURRENT_TIMESTAMP, '-1 minute')
		WHERE id = ?
	`, key.Data.Key.ID); err != nil {
		t.Fatal(err)
	}

	rotate := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys/"+key.Data.Key.ID+"/rotate",
		`{}`,
		login,
	)
	rotateResponse := mustTest(t, server, rotate)
	if rotateResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected expired rotation to fail with 409, got %d: %s", rotateResponse.StatusCode, readBody(t, rotateResponse))
	}
}

func TestScheduledPublishFlow(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{"slug":"schedule","name":"Schedule Project","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"launches","name":"Launches"}`)

	articleRequest := `{
		"articleType":"standard",
		"title":"Scheduled Post",
		"slug":"scheduled-post",
		"primaryCategoryId":"` + category.ID + `",
		"excerpt":"A scheduled excerpt",
		"html":"<p>Scheduled body</p>"
	}`
	article := createTestArticle(t, server, login, project.ID, articleRequest)
	revisionID := article.LatestRevision.ID

	scheduleBeforeApproval := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		strings.NewReader(`{"revisionId":"`+revisionID+`","slug":"scheduled-post","scheduledForUtc":"`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"}`),
	)
	scheduleBeforeApproval.Header.Set("Content-Type", "application/json")
	scheduleBeforeApproval.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(scheduleBeforeApproval, login.cookies)
	scheduleBeforeApprovalResponse := mustTest(t, server, scheduleBeforeApproval)
	if scheduleBeforeApprovalResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected scheduling an unapproved revision to fail with 409, got %d", scheduleBeforeApprovalResponse.StatusCode)
	}

	approveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/approve", strings.NewReader(`{}`))
	approveRequest.Header.Set("Content-Type", "application/json")
	approveRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(approveRequest, login.cookies)
	approveResponse := mustTest(t, server, approveRequest)
	if approveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected approve revision 200, got %d: %s", approveResponse.StatusCode, readBody(t, approveResponse))
	}

	scheduleRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		strings.NewReader(`{"revisionId":"`+revisionID+`","slug":"scheduled-post","scheduledForUtc":"`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"}`),
	)
	scheduleRequest.Header.Set("Content-Type", "application/json")
	scheduleRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(scheduleRequest, login.cookies)
	scheduleResponse := mustTest(t, server, scheduleRequest)
	if scheduleResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected schedule article 200, got %d: %s", scheduleResponse.StatusCode, readBody(t, scheduleResponse))
	}

	beforePublishRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/scheduled-post?locale=en", nil)
	beforePublishRequest.Header.Set("X-Dev-Project-ID", project.ID)
	beforePublishResponse := mustTest(t, server, beforePublishRequest)
	if beforePublishResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected scheduled article to remain hidden before worker publish, got %d", beforePublishResponse.StatusCode)
	}

	published, err := store.New(db).PublishDueSchedules(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("expected worker to publish one scheduled article, got %d", published)
	}

	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/scheduled-post?locale=en", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected published article to be readable, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}
	var payload Envelope[store.PublishedPost]
	decodeJSONResponse(t, publishedResponse, &payload)
	if payload.Data.Title != "Scheduled Post" {
		t.Fatalf("expected published title, got %q", payload.Data.Title)
	}
	if payload.Data.SEO.CanonicalURL != "https://example.test/blog/scheduled-post" {
		t.Fatalf("unexpected canonical URL %q", payload.Data.SEO.CanonicalURL)
	}
}

type adminLoginResult struct {
	cookies   []*http.Cookie
	csrfToken string
	userID    string
}

func newAdminTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	server := New(Options{
		Config: config.Config{Env: "development", DevAuth: true},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Store:  store.New(db),
	})
	return server, db
}

func createTestProject(t *testing.T, server *Server, login adminLoginResult, body string) store.AdminProject {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create project 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminProject]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func createTestCategory(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.TaxonomyTerm {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/categories", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create category 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.TaxonomyTerm]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func createTestArticle(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.AdminArticle {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/articles", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create article 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminArticle]
	decodeJSONResponse(t, response, &payload)
	if payload.Data.LatestRevision == nil {
		t.Fatal("expected created article to include latest revision")
	}
	return payload.Data
}

func createTestAPIKey(t *testing.T, server *Server, login adminLoginResult, projectID, body string) Envelope[store.APIKeyWithSecret] {
	t.Helper()
	request := newAPIKeyMutationRequest(t, http.MethodPost, "/api/v1/projects/"+projectID+"/api-keys", body, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create API key 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.APIKeyWithSecret]
	decodeJSONResponse(t, response, &payload)
	return payload
}

func newAPIKeyMutationRequest(t *testing.T, method, path, body string, login adminLoginResult) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	return request
}

func assertContentCategoriesStatus(t *testing.T, server *Server, secret string, expected int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/content/v1/categories", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	response := mustTest(t, server, request)
	if response.StatusCode != expected {
		t.Fatalf("expected content categories status %d, got %d: %s", expected, response.StatusCode, readBody(t, response))
	}
}

func seedOwner(t *testing.T, db *sql.DB, email, password string) string {
	t.Helper()
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.New(db).BootstrapOwner(context.Background(), email, passwordHash)
	if err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func seedAndLogin(t *testing.T, server *Server, db *sql.DB, email, password string) adminLoginResult {
	t.Helper()
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := security.RandomID("usr")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id, email_normalized, password_hash, status, email_verified_at, password_changed_at)
		VALUES (?, ?, ?, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, strings.ToLower(email), passwordHash)
	if err != nil {
		t.Fatal(err)
	}
	return adminLogin(t, server, email, password)
}

func adminLogin(t *testing.T, server *Server, email, password string) adminLoginResult {
	t.Helper()
	requestBody, err := json.Marshal(loginRequest{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[authResponse]
	decodeJSONResponse(t, response, &payload)
	return adminLoginResult{
		cookies:   response.Cookies(),
		csrfToken: payload.Data.CSRFToken,
		userID:    payload.Data.User.ID,
	}
}

func mustTest(t *testing.T, server *Server, request *http.Request) *http.Response {
	t.Helper()
	response, err := server.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func addCookies(request *http.Request, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
}

func decodeJSONResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
