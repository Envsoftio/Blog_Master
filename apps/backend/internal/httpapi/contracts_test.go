package httpapi

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func TestRollbackOpenAPIContract(t *testing.T) {
	server, _ := newAdminTestServer(t)
	item := server.openAPI.Paths["/api/v1/projects/{projectID}/articles/{articleID}/rollback"]
	if item == nil || item.Post == nil {
		t.Fatal("expected rollback POST operation")
	}
	operation := item.Post
	if operation.OperationID != "rollbackArticle" {
		t.Fatalf("expected explicit rollback operation ID, got %q", operation.OperationID)
	}
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
	success := operation.Responses["200"]
	if success == nil || success.Content["application/json"] == nil || success.Content["application/json"].Schema == nil {
		t.Fatal("expected rollback success response schema")
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

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
