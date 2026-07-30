package httpapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func TestProblemResponsesUseProblemJSON(t *testing.T) {
	server, _ := newAdminTestServer(t)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/project/articles/article/revisions",
		nil,
	)
	response := mustTest(t, server, request)
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated revision history to return 401, got %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != problemMediaType {
		t.Fatalf("expected problem response content type %q, got %q", problemMediaType, contentType)
	}
}

func TestAdminFrontendServiceContractsAreImplemented(t *testing.T) {
	server, _ := newAdminTestServer(t)
	routes := []struct {
		method        string
		path          string
		successStatus string
	}{
		{http.MethodGet, "/api/v1/projects/{projectID}/media", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/media/uploads", "201"},
		{http.MethodGet, "/api/v1/projects/{projectID}/media/{assetID}", "200"},
		{http.MethodPatch, "/api/v1/projects/{projectID}/media/{assetID}", "200"},
		{http.MethodDelete, "/api/v1/projects/{projectID}/media/{assetID}", "204"},
		{http.MethodGet, "/api/v1/projects/{projectID}/ai/jobs", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/ai/jobs", "202"},
		{http.MethodGet, "/api/v1/projects/{projectID}/ai/jobs/{jobID}", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/ai/jobs/{jobID}/cancel", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/webhooks", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/webhooks", "201"},
		{http.MethodPost, "/api/v1/projects/{projectID}/webhooks/{endpointID}/revoke", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/delivery/status", "200"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			item := server.openAPI.Paths[route.path]
			if item == nil {
				t.Fatal("expected documented route")
			}
			operation := operationForMethod(item, route.method)
			if operation == nil {
				t.Fatal("expected documented operation")
			}
			if _, ok := operation.Responses["501"]; ok {
				t.Fatal("implemented operation must not advertise 501")
			}
			if _, ok := operation.Responses[route.successStatus]; !ok {
				t.Fatalf("expected success status %s", route.successStatus)
			}
		})
	}
}

func TestRollbackOpenAPIContract(t *testing.T) {
	server, _ := newAdminTestServer(t)
	assertAdminSessionSecurityScheme(t, server)

	item := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/rollback"]
	if item == nil || item.Post == nil {
		t.Fatal("expected rollback POST operation")
	}
	operation := item.Post
	if operation.OperationID != "rollbackArticle" {
		t.Fatalf("expected explicit rollback operation ID, got %q", operation.OperationID)
	}
	assertAdminSessionSecurity(t, operation)
	assertRequiredParameter(t, operation, "projectID", "path")
	assertRequiredParameter(t, operation, "articleID", "path")
	assertRequiredParameter(t, operation, "X-CSRF-Token", "header")
	if operation.RequestBody == nil || !operation.RequestBody.Required {
		t.Fatal("expected required rollback request body")
	}
	mediaType := operation.RequestBody.Content["application/json"]
	if mediaType == nil || mediaType.Schema == nil {
		t.Fatal("expected rollback JSON request schema")
	}
	requestSchema := resolveContractSchema(t, server, mediaType.Schema)
	if _, ok := requestSchema.Properties["revisionId"]; !ok {
		t.Fatal("expected rollback request to document revisionId")
	}
	if _, ok := requestSchema.Properties["locale"]; !ok {
		t.Fatal("expected rollback request to document locale")
	}
	if _, ok := requestSchema.Properties["slug"]; ok {
		t.Fatal("rollback request must not document slug")
	}
	if _, ok := requestSchema.Properties["canonicalUrl"]; ok {
		t.Fatal("rollback request must not document canonicalUrl")
	}
	if !containsString(requestSchema.Required, "revisionId") {
		t.Fatal("expected revisionId to be required")
	}
	if _, ok := operation.Responses["409"]; !ok {
		t.Fatal("expected rollback contract to document workflow conflicts")
	}
	if _, ok := operation.Responses["501"]; ok {
		t.Fatal("implemented rollback operation must not advertise 501")
	}
	assertProblemResponseMediaTypes(t, server, operation, "400", "401", "403", "404", "409", "500")
	success := operation.Responses["200"]
	if success == nil || success.Content["application/json"] == nil || success.Content["application/json"].Schema == nil {
		t.Fatal("expected rollback success response schema")
	}
}

func TestCopyArticleOpenAPIContract(t *testing.T) {
	server, _ := newAdminTestServer(t)
	assertAdminSessionSecurityScheme(t, server)

	item := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/copy-to-project"]
	if item == nil || item.Post == nil {
		t.Fatal("expected copy-to-project POST operation")
	}
	operation := item.Post
	if operation.OperationID != "copyArticleToProject" {
		t.Fatalf("expected explicit copy operation ID, got %q", operation.OperationID)
	}
	if _, ok := operation.Responses["501"]; ok {
		t.Fatal("implemented copy operation must not advertise 501")
	}
	assertAdminSessionSecurity(t, operation)
	assertRequiredParameter(t, operation, "projectID", "path")
	assertRequiredParameter(t, operation, "articleID", "path")
	assertRequiredParameter(t, operation, "X-CSRF-Token", "header")
	if operation.RequestBody == nil || !operation.RequestBody.Required {
		t.Fatal("expected required copy request body")
	}
	mediaType := operation.RequestBody.Content["application/json"]
	if mediaType == nil || mediaType.Schema == nil {
		t.Fatal("expected copy JSON request schema")
	}
	requestSchema := resolveContractSchema(t, server, mediaType.Schema)
	for _, property := range []string{
		"destinationProjectId",
		"sourceRevisionId",
		"primaryCategoryId",
		"slug",
		"locale",
		"canonicalDecision",
		"canonicalOriginalUrl",
	} {
		contractProperty(t, requestSchema, property)
	}
	for _, property := range []string{
		"destinationProjectId",
		"sourceRevisionId",
		"primaryCategoryId",
		"slug",
		"canonicalDecision",
	} {
		if !containsString(requestSchema.Required, property) {
			t.Fatalf("expected copy property %q to be required", property)
		}
	}
	decisionSchema := contractProperty(t, requestSchema, "canonicalDecision")
	if !reflect.DeepEqual(decisionSchema.Enum, []any{"canonical_original", "material_adaptation"}) {
		t.Fatalf("unexpected canonical decision enum %#v", decisionSchema.Enum)
	}
	originalURLSchema := contractProperty(t, requestSchema, "canonicalOriginalUrl")
	if originalURLSchema.Format != "uri" || !strings.Contains(originalURLSchema.Description, "must match") {
		t.Fatalf("expected source-bound canonical URL documentation, got %#v", originalURLSchema)
	}
	assertProblemResponseMediaTypes(t, server, operation, "400", "401", "403", "404", "409", "500")
	success := operation.Responses["201"]
	if success == nil || success.Content["application/json"] == nil || success.Content["application/json"].Schema == nil {
		t.Fatal("expected copy success response schema")
	}
	responseEnvelopeSchema := resolveContractSchema(t, server, success.Content["application/json"].Schema)
	articleSchema := resolveContractSchema(t, server, contractProperty(t, responseEnvelopeSchema, "data"))
	for _, property := range []string{
		"id",
		"projectId",
		"originProjectId",
		"originArticleId",
		"canonicalPolicy",
		"canonicalUrl",
		"latestRevision",
	} {
		contractProperty(t, articleSchema, property)
	}
}

func TestCreateRevisionOpenAPIContract(t *testing.T) {
	server, _ := newAdminTestServer(t)
	assertAdminSessionSecurityScheme(t, server)

	item := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/revisions"]
	if item == nil || item.Post == nil {
		t.Fatal("expected create revision POST operation")
	}
	operation := item.Post
	if operation.OperationID != "createArticleRevision" {
		t.Fatalf("unexpected create revision operation ID %q", operation.OperationID)
	}
	if _, ok := operation.Responses["501"]; ok {
		t.Fatal("implemented create revision operation must not advertise 501")
	}
	assertAdminSessionSecurity(t, operation)
	assertRequiredParameter(t, operation, "projectID", "path")
	assertRequiredParameter(t, operation, "articleID", "path")
	assertRequiredParameter(t, operation, "X-CSRF-Token", "header")

	if operation.RequestBody == nil || !operation.RequestBody.Required {
		t.Fatal("expected required create revision request body")
	}
	requestMediaType := operation.RequestBody.Content["application/json"]
	if requestMediaType == nil || requestMediaType.Schema == nil {
		t.Fatal("expected create revision JSON request schema")
	}
	requestSchema := resolveContractSchema(t, server, requestMediaType.Schema)
	for _, property := range []string{"baseRevisionId", "title", "bodyDocument", "html"} {
		contractProperty(t, requestSchema, property)
	}
	for _, property := range []string{"baseRevisionId", "title"} {
		if !containsString(requestSchema.Required, property) {
			t.Fatalf("expected create revision property %q to be required", property)
		}
	}
	for _, property := range []string{"primaryCategoryId", "deck", "excerpt", "shortAnswer", "bodyDocument", "html"} {
		if containsString(requestSchema.Required, property) {
			t.Fatalf("expected create revision property %q to be optional", property)
		}
	}

	assertProblemResponseMediaTypes(t, server, operation, "400", "401", "403", "404", "409", "500")
	success := operation.Responses["201"]
	if success == nil || success.Content["application/json"] == nil || success.Content["application/json"].Schema == nil {
		t.Fatal("expected create revision 201 response schema")
	}
	responseEnvelopeSchema := resolveContractSchema(t, server, success.Content["application/json"].Schema)
	revisionSchema := resolveContractSchema(t, server, contractProperty(t, responseEnvelopeSchema, "data"))
	for _, property := range []string{"id", "articleId", "revisionNumber", "editorialState"} {
		contractProperty(t, revisionSchema, property)
	}
}

func TestRevisionHistoryOpenAPIContract(t *testing.T) {
	server, _ := newAdminTestServer(t)
	assertAdminSessionSecurityScheme(t, server)

	listItem := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/revisions"]
	if listItem == nil || listItem.Get == nil {
		t.Fatal("expected revision history GET operation")
	}
	listOperation := listItem.Get
	if listOperation.OperationID != "listArticleRevisions" {
		t.Fatalf("unexpected revision list operation ID %q", listOperation.OperationID)
	}
	if _, ok := listOperation.Responses["501"]; ok {
		t.Fatal("implemented revision history must not advertise 501")
	}
	assertAdminSessionSecurity(t, listOperation)
	assertRequiredParameter(t, listOperation, "projectID", "path")
	assertRequiredParameter(t, listOperation, "articleID", "path")
	cursorParameter := operationParameter(t, listOperation, "cursor", "query")
	if cursorParameter.Required || cursorParameter.Schema == nil || cursorParameter.Schema.Type != "string" {
		t.Fatal("expected an optional string revision-history cursor")
	}
	limitParameter := operationParameter(t, listOperation, "limit", "query")
	if limitParameter.Required || limitParameter.Schema == nil || limitParameter.Schema.Type != "integer" {
		t.Fatal("expected an optional integer revision-history limit")
	}
	if limitParameter.Schema.Minimum == nil || *limitParameter.Schema.Minimum != 1 {
		t.Fatal("expected revision-history limit minimum 1")
	}
	if limitParameter.Schema.Maximum == nil || *limitParameter.Schema.Maximum != 100 {
		t.Fatal("expected revision-history limit maximum 100")
	}
	assertProblemResponseMediaTypes(t, server, listOperation, "400", "401", "403", "404", "500")

	listSuccess := listOperation.Responses["200"]
	if listSuccess == nil || listSuccess.Content["application/json"] == nil || listSuccess.Content["application/json"].Schema == nil {
		t.Fatal("expected revision history success response schema")
	}
	listEnvelopeSchema := resolveContractSchema(t, server, listSuccess.Content["application/json"].Schema)
	listDataSchema := contractProperty(t, listEnvelopeSchema, "data")
	if listDataSchema.Items == nil {
		t.Fatal("expected revision history data to document array items")
	}
	revisionSummarySchema := resolveContractSchema(t, server, listDataSchema.Items)
	for _, property := range []string{"id", "articleId", "revisionNumber", "baseRevisionId", "publishedLocales"} {
		contractProperty(t, revisionSummarySchema, property)
	}
	if !containsString(revisionSummarySchema.Required, "publishedLocales") {
		t.Fatal("expected publishedLocales to be required in revision summaries")
	}
	if contractProperty(t, revisionSummarySchema, "publishedLocales").Nullable {
		t.Fatal("expected publishedLocales to be non-nullable in revision summaries")
	}
	assertRevisionHeadOperation(t, listItem.Head, "headArticleRevisions", true)

	detailItem := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/revisions/{revisionID}"]
	if detailItem == nil || detailItem.Get == nil {
		t.Fatal("expected revision detail GET operation")
	}
	detailOperation := detailItem.Get
	if detailOperation.OperationID != "getArticleRevision" {
		t.Fatalf("unexpected revision detail operation ID %q", detailOperation.OperationID)
	}
	if _, ok := detailOperation.Responses["501"]; ok {
		t.Fatal("implemented revision detail must not advertise 501")
	}
	assertAdminSessionSecurity(t, detailOperation)
	assertRequiredParameter(t, detailOperation, "projectID", "path")
	assertRequiredParameter(t, detailOperation, "articleID", "path")
	assertRequiredParameter(t, detailOperation, "revisionID", "path")
	assertProblemResponseMediaTypes(t, server, detailOperation, "401", "403", "404", "500")

	detailSuccess := detailOperation.Responses["200"]
	if detailSuccess == nil || detailSuccess.Content["application/json"] == nil || detailSuccess.Content["application/json"].Schema == nil {
		t.Fatal("expected revision detail success response schema")
	}
	detailEnvelopeSchema := resolveContractSchema(t, server, detailSuccess.Content["application/json"].Schema)
	detailSchema := resolveContractSchema(t, server, contractProperty(t, detailEnvelopeSchema, "data"))
	for _, property := range []string{
		"id",
		"baseRevisionId",
		"bodyDocument",
		"plainText",
		"taxonomySnapshot",
		"seoSnapshot",
		"publishedLocales",
	} {
		contractProperty(t, detailSchema, property)
	}
	for _, property := range []string{"bodyDocument", "plainText", "taxonomySnapshot", "seoSnapshot", "publishedLocales"} {
		if !containsString(detailSchema.Required, property) {
			t.Fatalf("expected revision detail property %q to be required", property)
		}
	}
	assertRevisionHeadOperation(t, detailItem.Head, "headArticleRevision", false)
}

func TestRevisionHeadRoutesUseImplementedHandlers(t *testing.T) {
	server, _ := newAdminTestServer(t)
	for _, path := range []string{
		"/api/v1/projects/project/articles/article/revisions",
		"/api/v1/projects/project/articles/article/revisions/revision",
	} {
		request := httptest.NewRequest(http.MethodHead, path, nil)
		response := mustTest(t, server, request)
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected unauthenticated HEAD %s to return 401, got %d", path, response.StatusCode)
		}
	}
}

func resolveContractSchema(t *testing.T, server *Server, schema *huma.Schema) *huma.Schema {
	t.Helper()
	if schema.Ref == "" {
		return schema
	}
	resolved := server.openAPI.Components.Schemas.SchemaFromRef(schema.Ref)
	if resolved == nil {
		t.Fatalf("could not resolve schema reference %q", schema.Ref)
	}
	return resolved
}

func contractProperty(t *testing.T, schema *huma.Schema, name string) *huma.Schema {
	t.Helper()
	property := schema.Properties[name]
	if property == nil {
		t.Fatalf("expected schema property %q", name)
	}
	return property
}

func operationParameter(t *testing.T, operation *huma.Operation, name, location string) *huma.Param {
	t.Helper()
	for _, parameter := range operation.Parameters {
		if parameter.Name == name && parameter.In == location {
			return parameter
		}
	}
	t.Fatalf("expected %s parameter %q", location, name)
	return nil
}

func assertRequiredParameter(t *testing.T, operation *huma.Operation, name, location string) {
	t.Helper()
	parameter := operationParameter(t, operation, name, location)
	if !parameter.Required {
		t.Fatalf("expected %s parameter %q to be required", location, name)
	}
}

func assertAdminSessionSecurityScheme(t *testing.T, server *Server) {
	t.Helper()
	schemes := server.openAPI.Components.SecuritySchemes
	scheme := schemes[adminSessionSecuritySchemeName]
	if scheme == nil {
		t.Fatalf("expected %q security scheme", adminSessionSecuritySchemeName)
	}
	if scheme.Type != "apiKey" || scheme.In != "cookie" || scheme.Name != sessionCookieName {
		t.Fatalf("unexpected admin session security scheme %#v", scheme)
	}
}

func assertAdminSessionSecurity(t *testing.T, operation *huma.Operation) {
	t.Helper()
	if len(operation.Security) != 1 || len(operation.Security[0]) != 1 {
		t.Fatalf("expected one admin session security requirement, got %#v", operation.Security)
	}
	scopes, ok := operation.Security[0][adminSessionSecuritySchemeName]
	if !ok || len(scopes) != 0 {
		t.Fatalf("expected empty-scope %q security requirement, got %#v", adminSessionSecuritySchemeName, operation.Security)
	}
}

func assertProblemResponseMediaTypes(t *testing.T, server *Server, operation *huma.Operation, statuses ...string) {
	t.Helper()
	for _, status := range statuses {
		response := operation.Responses[status]
		if response == nil {
			t.Fatalf("expected response status %s", status)
		}
		if len(response.Content) != 1 {
			t.Fatalf("expected status %s to document one problem response media type, got %#v", status, response.Content)
		}
		mediaType := response.Content[problemMediaType]
		if mediaType == nil || mediaType.Schema == nil {
			t.Fatalf("expected status %s to document %s", status, problemMediaType)
		}
		problemSchema := resolveContractSchema(t, server, mediaType.Schema)
		contractProperty(t, problemSchema, "title")
		contractProperty(t, problemSchema, "status")
	}
}

func assertRevisionHeadOperation(t *testing.T, operation *huma.Operation, operationID string, hasBadRequest bool) {
	t.Helper()
	if operation == nil {
		t.Fatalf("expected revision HEAD operation %q", operationID)
	}
	if operation.OperationID != operationID {
		t.Fatalf("expected revision HEAD operation ID %q, got %q", operationID, operation.OperationID)
	}
	if _, ok := operation.Responses["501"]; ok {
		t.Fatalf("implemented revision HEAD operation %q must not advertise 501", operationID)
	}
	if _, ok := operation.Responses["200"]; !ok {
		t.Fatalf("expected revision HEAD operation %q success response", operationID)
	}
	if _, ok := operation.Responses["400"]; ok != hasBadRequest {
		t.Fatalf("unexpected 400 response presence for revision HEAD operation %q", operationID)
	}
	assertAdminSessionSecurity(t, operation)
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
