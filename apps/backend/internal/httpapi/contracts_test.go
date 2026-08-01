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
		"/api/v1/projects/project/articles/article",
		nil,
	)
	response := mustTest(t, server, request)
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated article read to return 401, got %d", response.StatusCode)
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
		{http.MethodPost, "/api/v1/auth/forgot-password", "202"},
		{http.MethodPost, "/api/v1/auth/reset-password", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/media", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/media/uploads", "201"},
		{http.MethodGet, "/api/v1/projects/{projectID}/media/{assetID}/file", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/media/{assetID}", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/media/{assetID}/complete", "200"},
		{http.MethodPatch, "/api/v1/projects/{projectID}/media/{assetID}", "200"},
		{http.MethodDelete, "/api/v1/projects/{projectID}/media/{assetID}", "204"},
		{http.MethodGet, "/api/v1/projects/{projectID}/ai/jobs", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/ai/jobs", "202"},
		{http.MethodGet, "/api/v1/projects/{projectID}/ai/jobs/{jobID}", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/ai/jobs/{jobID}/cancel", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/ai/jobs/{jobID}/events", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/ai/runs", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/quality-checks", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/restore", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/voice-profile", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/voice-profile", "201"},
		{http.MethodGet, "/api/v1/projects/{projectID}/evidence-packets", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/evidence-packets", "201"},
		{http.MethodPost, "/api/v1/projects/{projectID}/evidence-packets/{packetID}/approve", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/webhooks", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/webhooks", "201"},
		{http.MethodPost, "/api/v1/projects/{projectID}/webhooks/{endpointID}/revoke", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/delivery/status", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/articles/{articleID}/disclosures", "200"},
		{http.MethodHead, "/api/v1/projects/{projectID}/articles/{articleID}/disclosures", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/disclosures", "201"},
		{http.MethodGet, "/api/v1/projects/{projectID}/webhook-attempts", "200"},
		{http.MethodHead, "/api/v1/projects/{projectID}/webhook-attempts", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/webhook-attempts/{attemptID}/replay", "202"},
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

func TestCoreArticleAdministrationContractsAreImplemented(t *testing.T) {
	server, _ := newAdminTestServer(t)
	routes := []struct {
		method, path, success string
	}{
		{http.MethodGet, "/api/v1/projects/{projectID}/articles", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles", "201"},
		{http.MethodGet, "/api/v1/projects/{projectID}/articles/{articleID}", "200"},
		{http.MethodPut, "/api/v1/projects/{projectID}/articles/{articleID}", "200"},
		{http.MethodDelete, "/api/v1/projects/{projectID}/articles/{articleID}", "204"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/restore", "200"},
		{http.MethodGet, "/api/v1/projects/{projectID}/articles/{articleID}/autosave", "200"},
		{http.MethodHead, "/api/v1/projects/{projectID}/articles/{articleID}/autosave", "200"},
		{http.MethodPut, "/api/v1/projects/{projectID}/articles/{articleID}/autosave", "200"},
		{http.MethodDelete, "/api/v1/projects/{projectID}/articles/{articleID}/autosave", "204"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/publish", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/schedule", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/unpublish", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/copy-to-project", "201"},
		{http.MethodGet, "/api/v1/projects/{projectID}/articles/{articleID}/disclosures", "200"},
		{http.MethodPost, "/api/v1/projects/{projectID}/articles/{articleID}/disclosures", "201"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			item := server.openAPI.Paths[route.path]
			if item == nil {
				t.Fatal("article API route is missing from OpenAPI")
			}
			operation := operationForMethod(item, route.method)
			if operation == nil {
				t.Fatal("article API operation is missing from OpenAPI")
			}
			if _, unfinished := operation.Responses["501"]; unfinished {
				t.Fatal("article API operation is still marked unimplemented")
			}
			if _, ok := operation.Responses[route.success]; !ok {
				t.Fatalf("expected success response %s", route.success)
			}
		})
	}
}

func TestArticleAutosaveOpenAPIContractIsExplicit(t *testing.T) {
	server, _ := newAdminTestServer(t)
	path := "/api/v1/projects/{projectID}/articles/{articleID}/autosave"
	item := server.openAPI.Paths[path]
	if item == nil || item.Get == nil || item.Head == nil || item.Put == nil || item.Delete == nil {
		t.Fatal("expected explicit autosave GET, HEAD, PUT and DELETE operations")
	}

	operation := item.Put
	if operation.OperationID != "saveArticleAutosave" {
		t.Fatalf("expected explicit autosave operation ID, got %q", operation.OperationID)
	}
	assertAdminSessionSecurity(t, operation)
	assertRequiredParameter(t, operation, "projectID", "path")
	assertRequiredParameter(t, operation, "articleID", "path")
	assertRequiredParameter(t, operation, "X-CSRF-Token", "header")
	if operation.RequestBody == nil || !operation.RequestBody.Required {
		t.Fatal("expected required autosave request body")
	}
	mediaType := operation.RequestBody.Content["application/json"]
	if mediaType == nil || mediaType.Schema == nil {
		t.Fatal("expected autosave JSON request schema")
	}
	requestSchema := resolveContractSchema(t, server, mediaType.Schema)
	for _, property := range []string{"baseRevisionId", "expectedVersion", "draft"} {
		propertySchema := contractProperty(t, requestSchema, property)
		if !containsString(requestSchema.Required, property) {
			t.Fatalf("expected autosave property %q to be required", property)
		}
		if property == "expectedVersion" && propertySchema.Nullable {
			t.Fatal("expected autosave expectedVersion to reject null")
		}
	}
	if _, ok := operation.Responses["409"]; !ok {
		t.Fatal("expected autosave contract to document version and base conflicts")
	}
	assertProblemResponseMediaTypes(t, server, operation, "400", "401", "403", "404", "409", "500")

	deleteOperation := item.Delete
	assertRequiredParameter(t, deleteOperation, "X-CSRF-Token", "header")
	if deleteOperation.RequestBody != nil {
		t.Fatal("autosave deletion must not require a request body")
	}
	assertAdminSessionSecurity(t, deleteOperation)
}

func TestAdminArticleListAndRestoreContractsAreExplicit(t *testing.T) {
	server, _ := newAdminTestServer(t)
	listOperation := operationForMethod(server.openAPI.Paths["/api/v1/projects/{projectID}/articles"], http.MethodGet)
	if listOperation == nil || listOperation.OperationID != "listAdminArticles" {
		t.Fatal("expected the explicit admin article-list operation")
	}
	assertRequiredParameter(t, listOperation, "projectID", "path")
	for _, parameter := range []string{"cursor", "limit", "q", "publicationState", "includeArchived"} {
		operationParameter(t, listOperation, parameter, "query")
	}
	if query := operationParameter(t, listOperation, "q", "query"); query.Schema == nil || query.Schema.MaxLength == nil || *query.Schema.MaxLength != 100 {
		t.Fatalf("expected a 100-character article search limit, got %#v", query.Schema)
	}
	if includeArchived := operationParameter(t, listOperation, "includeArchived", "query"); includeArchived.Schema == nil || includeArchived.Schema.Type != "boolean" {
		t.Fatalf("expected includeArchived to be boolean, got %#v", includeArchived.Schema)
	}
	assertAdminSessionSecurity(t, listOperation)

	restoreOperation := operationForMethod(server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/restore"], http.MethodPost)
	if restoreOperation == nil || restoreOperation.OperationID != "restoreArchivedArticle" {
		t.Fatal("expected the explicit archived-article restore operation")
	}
	assertRequiredParameter(t, restoreOperation, "projectID", "path")
	assertRequiredParameter(t, restoreOperation, "articleID", "path")
	assertRequiredParameter(t, restoreOperation, "X-CSRF-Token", "header")
	if restoreOperation.RequestBody != nil {
		t.Fatal("article restore must not require a request body")
	}
	if response := restoreOperation.Responses["200"]; response == nil || response.Content["application/json"] == nil || response.Content["application/json"].Schema == nil {
		t.Fatal("expected the restore response schema")
	}
	assertAdminSessionSecurity(t, restoreOperation)
}

func TestAdminOpenAPIDoesNotAdvertiseScaffoldedRoutes(t *testing.T) {
	server, _ := newAdminTestServer(t)
	for path, item := range server.openAPI.Paths {
		if !strings.HasPrefix(path, "/api/v1/") {
			continue
		}
		for _, method := range []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodPut,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
		} {
			operation := operationForMethod(item, method)
			if operation == nil {
				continue
			}
			if response := operation.Responses["501"]; response != nil {
				t.Fatalf("%s %s advertises scaffolded 501 response: %s", method, path, response.Description)
			}
		}
	}
}

func TestPasswordResetOpenAPIContracts(t *testing.T) {
	server, _ := newAdminTestServer(t)
	routes := []struct {
		path          string
		operationID   string
		successStatus string
		required      []string
	}{
		{
			path:          "/api/v1/auth/forgot-password",
			operationID:   "requestPasswordReset",
			successStatus: "202",
			required:      []string{"email"},
		},
		{
			path:          "/api/v1/auth/reset-password",
			operationID:   "completePasswordReset",
			successStatus: "200",
			required:      []string{"token", "password"},
		},
	}

	for _, route := range routes {
		t.Run(route.operationID, func(t *testing.T) {
			item := server.openAPI.Paths[route.path]
			if item == nil || item.Post == nil {
				t.Fatal("expected documented password-reset operation")
			}
			operation := item.Post
			if operation.OperationID != route.operationID {
				t.Fatalf("expected operation ID %q, got %q", route.operationID, operation.OperationID)
			}
			if operation.RequestBody == nil || !operation.RequestBody.Required {
				t.Fatal("expected required request body")
			}
			mediaType := operation.RequestBody.Content["application/json"]
			if mediaType == nil || mediaType.Schema == nil {
				t.Fatal("expected JSON request schema")
			}
			requestSchema := resolveContractSchema(t, server, mediaType.Schema)
			for _, property := range route.required {
				propertySchema := contractProperty(t, requestSchema, property)
				if !containsString(requestSchema.Required, property) {
					t.Fatalf("expected required property %q", property)
				}
				switch property {
				case "email":
					if propertySchema.Format != "email" {
						t.Fatalf("expected email format, got %q", propertySchema.Format)
					}
				case "password":
					if propertySchema.MinLength == nil || *propertySchema.MinLength != passwordMinLength {
						t.Fatalf("expected password minimum length %d, got %#v", passwordMinLength, propertySchema.MinLength)
					}
					if propertySchema.MaxLength == nil || *propertySchema.MaxLength != passwordMaxLength {
						t.Fatalf("expected password maximum length %d, got %#v", passwordMaxLength, propertySchema.MaxLength)
					}
				}
			}
			if _, ok := operation.Responses[route.successStatus]; !ok {
				t.Fatalf("expected success status %s", route.successStatus)
			}
			if _, ok := operation.Responses["429"]; !ok {
				t.Fatal("expected rate-limit response")
			}
			if _, ok := operation.Responses["501"]; ok {
				t.Fatal("implemented operation must not advertise 501")
			}
			if len(operation.Security) != 0 {
				t.Fatal("password recovery must not require an admin session")
			}
			assertProblemResponseMediaTypes(t, server, operation, "400", "429", "500")
		})
	}
}

func TestRemovedEditorialWorkflowRoutesAreNotDocumented(t *testing.T) {
	server, _ := newAdminTestServer(t)
	for _, path := range []string{
		"/api/v1/projects/{projectID}/articles/{articleID}/revisions",
		"/api/v1/projects/{projectID}/articles/{articleID}/revisions/{revisionID}",
		"/api/v1/projects/{projectID}/revisions/{revisionID}/submit",
		"/api/v1/projects/{projectID}/revisions/{revisionID}/request-changes",
		"/api/v1/projects/{projectID}/revisions/{revisionID}/approve",
		"/api/v1/projects/{projectID}/articles/{articleID}/rollback",
		"/api/v1/projects/{projectID}/review-assignees",
		"/api/v1/projects/{projectID}/articles/{articleID}/comments",
		"/api/v1/projects/{projectID}/comments/{commentID}/resolve",
		"/api/v1/projects/{projectID}/comments/{commentID}/reopen",
		"/api/v1/projects/{projectID}/articles/{articleID}/assignments",
		"/api/v1/projects/{projectID}/assignments/{assignmentID}/complete",
		"/api/v1/projects/{projectID}/assignments/{assignmentID}/cancel",
	} {
		if server.openAPI.Paths[path] != nil {
			t.Fatalf("removed workflow route remains documented: %s", path)
		}
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
		"primaryCategoryId",
		"slug",
		"canonicalDecision",
		"canonicalOriginalUrl",
		"contributorMappings",
	} {
		contractProperty(t, requestSchema, property)
	}
	for _, property := range []string{
		"destinationProjectId",
		"primaryCategoryId",
		"slug",
		"canonicalDecision",
		"contributorMappings",
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

func TestSaveArticleOpenAPIContract(t *testing.T) {
	server, _ := newAdminTestServer(t)
	assertAdminSessionSecurityScheme(t, server)

	item := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}"]
	if item == nil || item.Put == nil {
		t.Fatal("expected save article PUT operation")
	}
	operation := item.Put
	if operation.OperationID != "saveAdminArticle" {
		t.Fatalf("unexpected save article operation ID %q", operation.OperationID)
	}
	if _, ok := operation.Responses["501"]; ok {
		t.Fatal("implemented article save must not advertise 501")
	}
	assertAdminSessionSecurity(t, operation)
	assertRequiredParameter(t, operation, "projectID", "path")
	assertRequiredParameter(t, operation, "articleID", "path")
	assertRequiredParameter(t, operation, "X-CSRF-Token", "header")

	if operation.RequestBody == nil || !operation.RequestBody.Required {
		t.Fatal("expected required article save request body")
	}
	requestMediaType := operation.RequestBody.Content["application/json"]
	if requestMediaType == nil || requestMediaType.Schema == nil {
		t.Fatal("expected article save JSON request schema")
	}
	requestSchema := resolveContractSchema(t, server, requestMediaType.Schema)
	for _, property := range []string{"baseRevisionId", "title", "bodyDocument", "html"} {
		contractProperty(t, requestSchema, property)
	}
	for _, property := range []string{"title"} {
		if !containsString(requestSchema.Required, property) {
			t.Fatalf("expected article save property %q to be required", property)
		}
	}
	for _, property := range []string{"baseRevisionId", "primaryCategoryId", "deck", "excerpt", "shortAnswer", "bodyDocument", "html"} {
		if containsString(requestSchema.Required, property) {
			t.Fatalf("expected article save property %q to be optional", property)
		}
	}

	assertProblemResponseMediaTypes(t, server, operation, "400", "401", "403", "404", "409", "500")
	success := operation.Responses["200"]
	if success == nil || success.Content["application/json"] == nil || success.Content["application/json"].Schema == nil {
		t.Fatal("expected article save 200 response schema")
	}
	responseEnvelopeSchema := resolveContractSchema(t, server, success.Content["application/json"].Schema)
	articleSchema := resolveContractSchema(t, server, contractProperty(t, responseEnvelopeSchema, "data"))
	for _, property := range []string{"id", "title", "bodyDocument", "html", "primaryCategoryId", "contributors", "seo", "latestRevision"} {
		contractProperty(t, articleSchema, property)
	}
}

func TestRemovedRevisionRoutesReturnNotFound(t *testing.T) {
	server, _ := newAdminTestServer(t)
	for _, path := range []string{
		"/api/v1/projects/project/articles/article/revisions",
		"/api/v1/projects/project/articles/article/revisions/revision",
	} {
		request := httptest.NewRequest(http.MethodHead, path, nil)
		response := mustTest(t, server, request)
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("expected removed HEAD %s to return 404, got %d", path, response.StatusCode)
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
