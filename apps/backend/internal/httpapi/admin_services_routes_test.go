package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"seoblog/apps/backend/internal/media"
	"seoblog/apps/backend/internal/mediajobs"
	"seoblog/apps/backend/internal/platform/b2"
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
	if !strings.HasPrefix(created.Data.ObjectKey, "blogSEO/pending/"+projectA.ID+"/") {
		t.Fatalf("expected project-scoped blogSEO pending object key, got %q", created.Data.ObjectKey)
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

func TestAdminMediaRegistrationRejectsUnsafeFiles(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "media-validation-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-validation","name":"Media Validation"}`)
	path := "/api/v1/projects/" + project.ID + "/media/uploads"

	cases := []struct {
		name string
		body string
	}{
		{
			name: "svg",
			body: `{"filename":"logo.svg","contentType":"image/svg+xml","bytes":1024}`,
		},
		{
			name: "extension mismatch",
			body: `{"filename":"hero.jpg","contentType":"image/png","bytes":1024}`,
		},
		{
			name: "unsupported octet stream",
			body: `{"filename":"archive.bin","contentType":"application/octet-stream","bytes":1024}`,
		},
		{
			name: "missing extension",
			body: `{"filename":"hero","contentType":"image/png","bytes":1024}`,
		},
		{
			name: "oversized image",
			body: `{"filename":"huge.png","contentType":"image/png","bytes":26214401}`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, testCase.body, login))
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected unsafe media registration to fail with 400, got %d: %s", response.StatusCode, readBody(t, response))
			}
		})
	}
}

func TestAdminMediaUploadCompletionScansAndCreatesVariants(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-complete-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-complete","name":"Media Complete"}`)
	imageBytes := routeTestPNG(t, 8, 6)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":`+strconv.Itoa(len(imageBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Status != "uploading" ||
		created.Data.Upload == nil ||
		created.Data.Upload.URL == "" ||
		created.Data.Upload.Method != http.MethodPut ||
		created.Data.Upload.Headers["Content-Type"] != "image/png" ||
		len(created.Data.Upload.Fields) != 0 {
		t.Fatalf("expected signed upload target, got %#v", created.Data)
	}
	mediaStorage.objects[created.Data.ObjectKey] = imageBytes

	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected complete 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	var completed Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, completeResponse, &completed)
	if completed.Data.Status != "processing" || completed.Data.ScanStatus != "pending" {
		t.Fatalf("expected completion route to queue processing, got %#v", completed.Data)
	}
	processed, err := mediajobs.Processor{Store: server.store, Storage: mediaStorage}.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 {
		t.Fatalf("expected one processed media asset, got %d", processed)
	}
	ready, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	server.enrichMediaAsset(&ready)
	if ready.Status != "ready" ||
		ready.ScanStatus != "passed" ||
		ready.Width != 8 ||
		ready.Height != 6 ||
		len(ready.SHA256) != 64 ||
		len(ready.Variants) != 3 {
		t.Fatalf("unexpected ready asset %#v", ready)
	}
	if ready.URL != "/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/file?variant=square_1x1" {
		t.Fatalf("expected API-backed preview URL, got %q", ready.URL)
	}
	if !strings.HasPrefix(ready.Variants[0].ObjectKey, "blogSEO/projects/"+project.ID+"/media/variants/") {
		t.Fatalf("expected blogSEO variant prefix, got %q", ready.Variants[0].ObjectKey)
	}
	if ready.Variants[1].URL != "/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/file?variant=landscape_4x3" {
		t.Fatalf("expected API-backed variant URL, got %q", ready.Variants[1].URL)
	}
	previewRequest := httptest.NewRequest(http.MethodGet, ready.URL, nil)
	addCookies(previewRequest, login.cookies)
	previewResponse := mustTest(t, server, previewRequest)
	if previewResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewResponse.StatusCode, readBody(t, previewResponse))
	}
	if contentType := previewResponse.Header.Get("Content-Type"); contentType != "image/jpeg" {
		t.Fatalf("expected JPEG preview content type, got %q", contentType)
	}
	previewBody, err := io.ReadAll(previewResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(previewBody, mediaStorage.puts[ready.Variants[0].ObjectKey]) {
		t.Fatal("expected preview response to stream the first generated variant")
	}
	if ready.ObjectKey != ready.Variants[0].ObjectKey {
		t.Fatalf("expected ready asset object key to be promoted to first variant, got %q want %q", ready.ObjectKey, ready.Variants[0].ObjectKey)
	}
	if len(mediaStorage.puts) != 3 {
		t.Fatalf("expected three variant uploads, got %d", len(mediaStorage.puts))
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != 1 {
		t.Fatalf("expected processed original cleanup, got %#v", mediaStorage.deletes)
	}
	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != 2 {
		t.Fatalf("expected ready deletion to sweep the already-cleaned pending key idempotently, got %#v", mediaStorage.deletes)
	}
	for _, variant := range ready.Variants {
		if mediaStorage.deletes[variant.ObjectKey] != 1 {
			t.Fatalf("expected variant object %q deletion, got %#v", variant.ObjectKey, mediaStorage.deletes)
		}
		if _, exists := mediaStorage.puts[variant.ObjectKey]; exists {
			t.Fatalf("expected ready variant %q to be removed from B2 storage", variant.ObjectKey)
		}
	}
	if _, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID); err == nil {
		t.Fatal("expected ready image media row to be removed after B2 objects")
	}
}

func TestAdminMediaProcessorCleansReadyPendingOriginals(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-cleanup-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-cleanup","name":"Media Cleanup"}`)
	imageBytes := routeTestPNG(t, 8, 6)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":`+strconv.Itoa(len(imageBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = imageBytes
	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected completion queue 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	processor := mediajobs.Processor{Store: server.store, Storage: mediaStorage}
	if _, err := processor.Process(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	ready, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready.Variants) == 0 {
		t.Fatalf("expected image variants, got %#v", ready)
	}

	originalDeletes := mediaStorage.deletes[created.Data.ObjectKey]
	mediaStorage.objects[created.Data.ObjectKey] = imageBytes
	if _, err := db.Exec(`UPDATE assets SET object_key = ? WHERE project_id = ? AND id = ?`, created.Data.ObjectKey, project.ID, created.Data.ID); err != nil {
		t.Fatal(err)
	}
	processed, err := processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 0 {
		t.Fatalf("expected no processing jobs during cleanup sweep, got %d", processed)
	}
	cleaned, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.ObjectKey != ready.Variants[0].ObjectKey {
		t.Fatalf("expected stale ready original to be promoted to first variant, got %q want %q", cleaned.ObjectKey, ready.Variants[0].ObjectKey)
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != originalDeletes+1 {
		t.Fatalf("expected stale ready original cleanup, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; exists {
		t.Fatalf("expected stale pending original to be removed from storage")
	}
}

func TestAdminMediaProcessingRetriesPendingOriginalCleanup(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-delete-failure-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-failure","name":"Media Delete Failure"}`)
	imageBytes := routeTestPNG(t, 8, 6)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":`+strconv.Itoa(len(imageBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = imageBytes
	mediaStorage.failDeletes[created.Data.ObjectKey] = true
	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected completion queue 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	processor := mediajobs.Processor{Store: server.store, Storage: mediaStorage}
	if processed, err := processor.Process(context.Background(), 10); err == nil {
		t.Fatal("expected original cleanup failure")
	} else if processed != 0 {
		t.Fatalf("expected failed cleanup not to count as processed, got %d", processed)
	}
	pendingCleanup, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pendingCleanup.Status != "ready" || pendingCleanup.ObjectKey != created.Data.ObjectKey || len(pendingCleanup.Variants) != 3 {
		t.Fatalf("expected ready asset to retain its pending pointer for cleanup retry, got %#v", pendingCleanup)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatalf("expected original object to remain when cleanup delete fails")
	}

	delete(mediaStorage.failDeletes, created.Data.ObjectKey)
	if processed, err := processor.Process(context.Background(), 10); err != nil {
		t.Fatal(err)
	} else if processed != 0 {
		t.Fatalf("expected cleanup retry not to count as a processing job, got %d", processed)
	}
	cleaned, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.Status != "ready" || cleaned.ObjectKey != cleaned.Variants[0].ObjectKey {
		t.Fatalf("expected cleanup retry to promote the ready variant, got %#v", cleaned)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; exists {
		t.Fatal("expected cleanup retry to remove the pending original")
	}
}

func TestAdminMediaUploadCompletionMovesNoVariantOriginalOutOfPending(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-original-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-original","name":"Media Original"}`)
	pdfBytes := safeRouteTestPDF()
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"Brief 2026.pdf","contentType":"application/pdf","bytes":`+strconv.Itoa(len(pdfBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = pdfBytes
	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected completion queue 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	processed, err := (mediajobs.Processor{Store: server.store, Storage: mediaStorage}).Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 {
		t.Fatalf("expected one processed media asset, got %d", processed)
	}
	ready, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "ready" || ready.ObjectKey == created.Data.ObjectKey || strings.Contains(ready.ObjectKey, "/pending/") || len(ready.Variants) != 0 {
		t.Fatalf("expected ready no-variant asset outside pending, got %#v", ready)
	}
	if !strings.HasPrefix(ready.ObjectKey, "blogSEO/projects/"+project.ID+"/media/originals/"+created.Data.ID+"/") {
		t.Fatalf("expected processed original prefix, got %q", ready.ObjectKey)
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != 1 {
		t.Fatalf("expected pending original cleanup, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; exists {
		t.Fatal("expected pending original to be removed from storage")
	}
	if !bytes.Equal(mediaStorage.puts[ready.ObjectKey], pdfBytes) {
		t.Fatal("expected processed original to be uploaded to final storage")
	}

	server.enrichMediaAsset(&ready)
	fileRequest := httptest.NewRequest(http.MethodGet, ready.URL, nil)
	addCookies(fileRequest, login.cookies)
	fileResponse := mustTest(t, server, fileRequest)
	if fileResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected file 200, got %d: %s", fileResponse.StatusCode, readBody(t, fileResponse))
	}
	if contentType := fileResponse.Header.Get("Content-Type"); contentType != "application/pdf" {
		t.Fatalf("expected PDF content type, got %q", contentType)
	}
	fileBody, err := io.ReadAll(fileResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fileBody, pdfBytes) {
		t.Fatal("expected file route to stream the processed original")
	}
}

func TestAdminMediaNoVariantProcessingRetriesPendingOriginalCleanup(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-original-cleanup-retry-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-original-cleanup-retry","name":"Media Original Cleanup Retry"}`)
	pdfBytes := safeRouteTestPDF()
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"retry.pdf","contentType":"application/pdf","bytes":`+strconv.Itoa(len(pdfBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = pdfBytes
	mediaStorage.failDeletes[created.Data.ObjectKey] = true
	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected completion queue 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}

	processor := mediajobs.Processor{Store: server.store, Storage: mediaStorage}
	if processed, err := processor.Process(context.Background(), 10); err == nil {
		t.Fatal("expected pending original cleanup failure")
	} else if processed != 0 {
		t.Fatalf("expected failed cleanup not to count as processed, got %d", processed)
	}
	pendingCleanup, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pendingCleanup.Status != "ready" || pendingCleanup.ObjectKey != created.Data.ObjectKey || len(pendingCleanup.Variants) != 0 {
		t.Fatalf("expected ready document to retain its pending pointer for cleanup retry, got %#v", pendingCleanup)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatal("expected pending document to remain after failed B2 deletion")
	}
	finalObjectKey := media.ProcessedOriginalObjectKey(project.ID, created.Data.ID, created.Data.Filename)
	if !bytes.Equal(mediaStorage.puts[finalObjectKey], pdfBytes) {
		t.Fatal("expected the processed document to exist before pending cleanup")
	}

	delete(mediaStorage.failDeletes, created.Data.ObjectKey)
	if processed, err := processor.Process(context.Background(), 10); err != nil {
		t.Fatal(err)
	} else if processed != 0 {
		t.Fatalf("expected cleanup retry not to count as a processing job, got %d", processed)
	}
	cleaned, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.Status != "ready" || cleaned.ObjectKey != finalObjectKey || strings.Contains(cleaned.ObjectKey, "/pending/") {
		t.Fatalf("expected cleanup retry to promote the processed document, got %#v", cleaned)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; exists {
		t.Fatal("expected cleanup retry to remove the pending document")
	}
}

func TestAdminMediaDeleteRemovesProcessedOriginalFromStorage(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-delete-original-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-original","name":"Media Delete Original"}`)
	pdfBytes := safeRouteTestPDF()
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"brief.pdf","contentType":"application/pdf","bytes":`+strconv.Itoa(len(pdfBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = pdfBytes
	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected completion queue 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	if _, err := (mediajobs.Processor{Store: server.store, Storage: mediaStorage}).Process(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	ready, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.ObjectKey == "" || strings.Contains(ready.ObjectKey, "/pending/") {
		t.Fatalf("expected processed original object key, got %q", ready.ObjectKey)
	}
	if _, exists := mediaStorage.puts[ready.ObjectKey]; !exists {
		t.Fatalf("expected processed original %q in storage", ready.ObjectKey)
	}

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if mediaStorage.deletes[ready.ObjectKey] != 1 {
		t.Fatalf("expected processed original B2 deletion, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.puts[ready.ObjectKey]; exists {
		t.Fatal("expected processed original to be removed from storage")
	}
	if _, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID); err == nil {
		t.Fatal("expected media row to be removed after successful B2 deletion")
	}
}

func TestReadyMediaObjectDeletionSweepsPendingAndProcessedCopies(t *testing.T) {
	mediaStorage := newFakeMediaStorage()
	server := &Server{mediaStorage: mediaStorage}
	asset := store.AdminMediaAsset{
		ID:        "asset_cleanup_window",
		ProjectID: "project_cleanup_window",
		Filename:  "brief.pdf",
		Status:    "ready",
	}
	asset.ObjectKey = media.PendingOriginalObjectKey(asset.ProjectID, asset.ID, asset.Filename)
	processedObjectKey := media.ProcessedOriginalObjectKey(asset.ProjectID, asset.ID, asset.Filename)
	mediaStorage.objects[asset.ObjectKey] = []byte("pending")
	mediaStorage.puts[processedObjectKey] = []byte("processed")

	if err := server.deleteMediaObjects(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	if mediaStorage.deletes[asset.ObjectKey] != 1 || mediaStorage.deletes[processedObjectKey] != 1 {
		t.Fatalf("expected ready cleanup-window copies to be deleted once, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[asset.ObjectKey]; exists {
		t.Fatal("expected pending ready-media copy to be deleted")
	}
	if _, exists := mediaStorage.puts[processedObjectKey]; exists {
		t.Fatal("expected processed ready-media copy to be deleted")
	}
}

func TestAdminMediaDeleteBlocksAuthorPhotoBeforeStorageDeletion(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	login := seedAndLogin(t, server, db, "media-delete-author-photo-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-author-photo","name":"Media Delete Author Photo"}`)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"author.png","contentType":"image/png","bytes":128}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = []byte("uploaded object")
	createTestAuthor(t, server, login, project.ID, `{"displayName":"Referenced Author","photoAssetId":"`+created.Data.ID+`"}`)

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected in-use media delete 409, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if len(mediaStorage.deletes) != 0 {
		t.Fatalf("expected usage preflight before any B2 deletion, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatal("expected referenced author photo to remain in storage")
	}
	if _, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID); err != nil {
		t.Fatalf("expected referenced media row to remain: %v", err)
	}
}

func TestAdminMediaDeleteBlocksStructuredRevisionReferenceBeforeStorageDeletion(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	login := seedAndLogin(t, server, db, "media-delete-revision-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-revision","name":"Media Delete Revision"}`)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"article.png","contentType":"image/png","bytes":128}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = []byte("uploaded object")
	category := createTestCategory(t, server, login, project.ID, `{"name":"Media references","slug":"media-references"}`)
	createTestArticle(t, server, login, project.ID, `{
		"title":"Article with media",
		"slug":"article-with-media",
		"primaryCategoryId":"`+category.ID+`",
		"bodyDocument":{
			"type":"doc",
			"content":[{"type":"image","attrs":{"assetId":"`+created.Data.ID+`"}}]
		}
	}`)

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected revision-referenced media delete 409, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if len(mediaStorage.deletes) != 0 {
		t.Fatalf("expected revision usage preflight before any B2 deletion, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatal("expected revision-referenced media to remain in storage")
	}
}

func TestAdminMediaProcessorCleansReadyPendingNoVariantOriginals(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-no-variant-cleanup-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-no-variant-cleanup","name":"Media No Variant Cleanup"}`)
	pdfBytes := safeRouteTestPDF()
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"brief.pdf","contentType":"application/pdf","bytes":`+strconv.Itoa(len(pdfBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = pdfBytes
	if _, err := db.Exec(`
		UPDATE assets
		SET status = 'ready',
		    scan_status = 'passed',
		    mime_type = 'application/pdf',
		    checksum_sha256 = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
		WHERE project_id = ? AND id = ?
	`, project.ID, created.Data.ID); err != nil {
		t.Fatal(err)
	}
	processed, err := (mediajobs.Processor{Store: server.store, Storage: mediaStorage}).Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 0 {
		t.Fatalf("expected cleanup sweep not processing jobs, got %d", processed)
	}
	cleaned, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cleaned.ObjectKey, "/pending/") || !strings.HasPrefix(cleaned.ObjectKey, "blogSEO/projects/"+project.ID+"/media/originals/"+created.Data.ID+"/") {
		t.Fatalf("expected stale ready original to move out of pending, got %q", cleaned.ObjectKey)
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != 1 {
		t.Fatalf("expected stale pending original cleanup, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; exists {
		t.Fatal("expected stale pending original to be removed from storage")
	}
	if !bytes.Equal(mediaStorage.puts[cleaned.ObjectKey], pdfBytes) {
		t.Fatal("expected stale ready original to be uploaded to final storage")
	}
}

func TestAdminMediaUploadCompletionRejectsUnsafePDF(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-reject-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-reject","name":"Media Reject"}`)
	pdfBytes := []byte("%PDF-1.7\n1 0 obj << /OpenAction << /S /JavaScript /JS (app.alert(1)) >> >> endobj\n")
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"brief.pdf","contentType":"application/pdf","bytes":`+strconv.Itoa(len(pdfBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = pdfBytes

	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected unsafe PDF to queue with 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	if _, err := (mediajobs.Processor{Store: server.store, Storage: mediaStorage}).Process(context.Background(), 10); err == nil {
		t.Fatal("expected unsafe PDF processing to return an error")
	}
	rejected, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "rejected" || rejected.ScanStatus != "failed" || rejected.ScanReason == "" {
		t.Fatalf("expected rejected scan state, got %#v", rejected)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; exists {
		t.Fatalf("expected rejected original to be deleted from B2")
	}
}

func TestAdminMediaProcessingCleansUpPartialVariantUploads(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	mediaStorage.failPutAfter = 1
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-partial-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-partial","name":"Media Partial"}`)
	imageBytes := routeTestPNG(t, 8, 6)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":`+strconv.Itoa(len(imageBytes))+`}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected signed media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = imageBytes
	completeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID+"/complete",
		`{}`,
		login,
	))
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected completion queue 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	if _, err := (mediajobs.Processor{Store: server.store, Storage: mediaStorage}).Process(context.Background(), 10); err == nil {
		t.Fatal("expected partial variant upload failure")
	}
	failed, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.ScanStatus != "skipped" || failed.ScanReason == "" {
		t.Fatalf("expected failed asset state, got %#v", failed)
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != 1 {
		t.Fatalf("expected original cleanup, got %#v", mediaStorage.deletes)
	}
	if len(mediaStorage.puts) != 0 {
		t.Fatalf("expected partial variants to be deleted, got %#v", mediaStorage.puts)
	}
}

func TestAdminMediaDeleteKeepsRowWhenB2DeleteFails(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	login := seedAndLogin(t, server, db, "media-delete-failure-route-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-failure-route","name":"Media Delete Failure Route"}`)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":128}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = []byte("uploaded object")
	mediaStorage.failDeletes[created.Data.ObjectKey] = true

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected delete failure 502, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if mediaStorage.deletes[created.Data.ObjectKey] != 1 {
		t.Fatalf("expected one B2 delete attempt, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatal("expected object to remain when B2 deletion fails")
	}
	if _, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID); err != nil {
		t.Fatalf("expected media row to remain when B2 deletion fails: %v", err)
	}
}

func TestAdminMediaDeleteWithoutStorageKeepsDatabaseRow(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	server.cfg.B2MediaPresignTTL = 15 * time.Minute
	login := seedAndLogin(t, server, db, "media-delete-no-storage-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-no-storage","name":"Media Delete No Storage"}`)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":128}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = []byte("uploaded object")
	server.mediaStorage = nil

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected unconfigured storage delete to fail with 502, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatal("expected B2 object to remain when storage is unavailable")
	}
	if _, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID); err != nil {
		t.Fatalf("expected media row to remain after storage-unavailable delete: %v", err)
	}
}

func TestAdminMediaDeleteRefusesRootFolderObjectKey(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	login := seedAndLogin(t, server, db, "media-delete-root-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-root","name":"Media Delete Root"}`)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":128}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	if _, err := db.Exec(`UPDATE assets SET object_key = ? WHERE project_id = ? AND id = ?`, "blogSEO/", project.ID, created.Data.ID); err != nil {
		t.Fatal(err)
	}

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected unsafe delete to fail with 502, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if len(mediaStorage.deletes) != 0 {
		t.Fatalf("expected no B2 delete calls for root key, got %#v", mediaStorage.deletes)
	}
	if _, err := server.store.GetMediaAsset(context.Background(), login.userID, project.ID, created.Data.ID); err != nil {
		t.Fatalf("expected asset row to remain after refused delete: %v", err)
	}
}

func TestAdminMediaDeleteRefusesUnrelatedVariantKeyBeforeDeletingAnything(t *testing.T) {
	server, db := newAdminTestServer(t)
	mediaStorage := newFakeMediaStorage()
	server.mediaStorage = mediaStorage
	login := seedAndLogin(t, server, db, "media-delete-scope-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"media-delete-scope","name":"Media Delete Scope"}`)
	createResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/media/uploads",
		`{"filename":"hero.png","contentType":"image/png","bytes":128}`,
		login,
	))
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected media create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.AdminMediaAsset]
	decodeJSONResponse(t, createResponse, &created)
	mediaStorage.objects[created.Data.ObjectKey] = []byte("original")
	variantID, err := security.RandomID("variant")
	if err != nil {
		t.Fatal(err)
	}
	unrelatedVariantKey := "blogSEO/projects/" + project.ID + "/media/variants/asset_other/square_1x1.jpg"
	if _, err := db.Exec(`
		INSERT INTO asset_variants(
			id, project_id, asset_id, variant_name, object_key,
			mime_type, width, height, byte_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, variantID, project.ID, created.Data.ID, "square_1x1", unrelatedVariantKey, "image/jpeg", 1, 1, 16); err != nil {
		t.Fatal(err)
	}

	deleteResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/media/"+created.Data.ID,
		`{}`,
		login,
	))
	if deleteResponse.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected unsafe delete to fail with 502, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	if len(mediaStorage.deletes) != 0 {
		t.Fatalf("expected validation to run before any B2 delete calls, got %#v", mediaStorage.deletes)
	}
	if _, exists := mediaStorage.objects[created.Data.ObjectKey]; !exists {
		t.Fatalf("expected valid original object to remain when unrelated variant key is refused")
	}
}

type fakeMediaStorage struct {
	objects      map[string][]byte
	puts         map[string][]byte
	deletes      map[string]int
	failDeletes  map[string]bool
	failPutAfter int
}

func newFakeMediaStorage() *fakeMediaStorage {
	return &fakeMediaStorage{
		objects:      map[string][]byte{},
		puts:         map[string][]byte{},
		deletes:      map[string]int{},
		failDeletes:  map[string]bool{},
		failPutAfter: -1,
	}
}

func (s *fakeMediaStorage) Bucket() string {
	return "media-bucket"
}

func (s *fakeMediaStorage) PublicURL(key string) string {
	if key == "" {
		return ""
	}
	return "https://cdn.example.test/" + key
}

func (s *fakeMediaStorage) PresignPut(key, contentType string, maxBytes int64, now time.Time) (b2.SignedUpload, error) {
	return b2.SignedUpload{
		URL:       "https://uploads.example.test/" + key,
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339),
		MaxBytes:  maxBytes,
	}, nil
}

func (s *fakeMediaStorage) GetObject(ctx context.Context, key string, maxBytes int64) ([]byte, string, error) {
	body, ok := s.objects[key]
	if !ok {
		body, ok = s.puts[key]
	}
	if !ok {
		return nil, "", fmt.Errorf("object %q not found", key)
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("object %q exceeds limit", key)
	}
	return append([]byte(nil), body...), http.DetectContentType(body), nil
}

func (s *fakeMediaStorage) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	if s.failPutAfter >= 0 && len(s.puts) >= s.failPutAfter {
		return fmt.Errorf("variant upload failed")
	}
	s.puts[key] = append([]byte(nil), body...)
	return nil
}

func (s *fakeMediaStorage) DeleteObject(ctx context.Context, key string) error {
	s.deletes[key]++
	if s.failDeletes[key] {
		return fmt.Errorf("object deletion failed")
	}
	delete(s.objects, key)
	delete(s.puts, key)
	return nil
}

func routeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	image := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			image.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 180, A: 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, image); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func safeRouteTestPDF() []byte {
	return []byte("%PDF-1.7\n1 0 obj << /Type /Catalog >> endobj\n2 0 obj << /Type /Page >> endobj\n%%EOF\n")
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

	published := publishTestArticle(t, server, login, project.ID, article.ID, article.Slug)
	if published.PublicationState != "published" {
		t.Fatalf("expected quality checks to remain advisory during direct publication, got %#v", published)
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
	var storedHash, storedCiphertext string
	if err := db.QueryRow(`
		SELECT secret_hash, COALESCE(secret_ciphertext, '')
		FROM webhook_endpoints
		WHERE project_id = ? AND id = ?
	`, project.ID, created.Data.ID).Scan(&storedHash, &storedCiphertext); err != nil {
		t.Fatal(err)
	}
	if storedHash == created.Data.Secret || storedHash != security.TokenHash(created.Data.Secret) {
		t.Fatal("expected only the webhook secret hash to be stored")
	}
	if storedCiphertext == "" || strings.Contains(storedCiphertext, created.Data.Secret) {
		t.Fatal("expected an encrypted signing-secret envelope")
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
	duplicateReplayResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/webhook-attempts/attempt-failed/replay",
		`{}`,
		login,
	))
	if duplicateReplayResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate pending replay conflict, got %d: %s", duplicateReplayResponse.StatusCode, readBody(t, duplicateReplayResponse))
	}
	var processedAt string
	if err := db.QueryRow(`SELECT COALESCE(processed_at, '') FROM outbox_events WHERE project_id = ? AND id = ?`, project.ID, "event-failed").Scan(&processedAt); err != nil {
		t.Fatal(err)
	}
	if processedAt == "" {
		t.Fatal("expected replay to leave shared outbox processing state unchanged")
	}
	var replayOfAttemptID string
	if err := db.QueryRow(`
		SELECT COALESCE(replay_of_attempt_id, '')
		FROM webhook_attempts
		WHERE id = ?
	`, replayed.Data.ID).Scan(&replayOfAttemptID); err != nil {
		t.Fatal(err)
	}
	if replayOfAttemptID != "attempt-failed" {
		t.Fatalf("expected replay to target only its source delivery, got %q", replayOfAttemptID)
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
	if _, err := db.Exec(`
		UPDATE webhook_attempts
		SET status = 'failed'
		WHERE id = ?
	`, replayed.Data.ID); err != nil {
		t.Fatal(err)
	}
	chainedReplayResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/webhook-attempts/"+replayed.Data.ID+"/replay",
		`{}`,
		login,
	))
	if chainedReplayResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("expected replay of failed replay 202, got %d: %s", chainedReplayResponse.StatusCode, readBody(t, chainedReplayResponse))
	}
	var chainedReplay Envelope[store.WebhookAttempt]
	decodeJSONResponse(t, chainedReplayResponse, &chainedReplay)
	if chainedReplay.Data.ReplayOfAttemptID != "attempt-failed" {
		t.Fatalf("expected replay chain to retain root attempt, got %#v", chainedReplay.Data)
	}

	revokeResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		basePath+"/"+created.Data.ID+"/revoke",
		`{}`,
		login,
	))
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected webhook revoke 200, got %d: %s", revokeResponse.StatusCode, readBody(t, revokeResponse))
	}
	var revoked Envelope[store.WebhookEndpoint]
	decodeJSONResponse(t, revokeResponse, &revoked)
	if revoked.Data.Status != "revoked" {
		t.Fatalf("expected revoked endpoint, got %#v", revoked.Data)
	}
	var webhookAuditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action IN ('webhook.create', 'webhook.revoke')
	`, project.ID, created.Data.ID).Scan(&webhookAuditCount); err != nil {
		t.Fatal(err)
	}
	if webhookAuditCount != 2 {
		t.Fatalf("expected webhook create and revoke audits, got %d", webhookAuditCount)
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
		validVoiceProfileBody("Direct, exact, and practical"),
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
