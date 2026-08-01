package httpapi

import (
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"

	"seoblog/apps/backend/internal/store"
)

var fiberParameter = regexp.MustCompile(`:([A-Za-z][A-Za-z0-9_]*)`)
var operationIDPart = regexp.MustCompile(`[^A-Za-z0-9]+`)

const adminSessionSecuritySchemeName = "adminSession"

var implementedAdminRouteStatuses = map[string]string{
	"POST /api/v1/auth/login":                                                  "200",
	"POST /api/v1/auth/forgot-password":                                        "202",
	"POST /api/v1/auth/reset-password":                                         "200",
	"POST /api/v1/invitations/{token}/accept":                                  "200",
	"GET /api/v1/auth/me":                                                      "200",
	"HEAD /api/v1/auth/me":                                                     "200",
	"GET /api/v1/auth/csrf":                                                    "200",
	"HEAD /api/v1/auth/csrf":                                                   "200",
	"POST /api/v1/auth/reauthenticate":                                         "200",
	"POST /api/v1/auth/logout":                                                 "204",
	"GET /api/v1/projects":                                                     "200",
	"HEAD /api/v1/projects":                                                    "200",
	"POST /api/v1/projects":                                                    "201",
	"GET /api/v1/projects/{projectID}":                                         "200",
	"HEAD /api/v1/projects/{projectID}":                                        "200",
	"PATCH /api/v1/projects/{projectID}":                                       "200",
	"POST /api/v1/projects/{projectID}/suspend":                                "200",
	"POST /api/v1/projects/{projectID}/archive":                                "200",
	"GET /api/v1/projects/{projectID}/deletion-impact":                         "200",
	"HEAD /api/v1/projects/{projectID}/deletion-impact":                        "200",
	"DELETE /api/v1/projects/{projectID}":                                      "204",
	"GET /api/v1/projects/{projectID}/members":                                 "200",
	"HEAD /api/v1/projects/{projectID}/members":                                "200",
	"POST /api/v1/projects/{projectID}/invitations":                            "201",
	"PATCH /api/v1/projects/{projectID}/members/{userID}":                      "200",
	"DELETE /api/v1/projects/{projectID}/members/{userID}":                     "204",
	"POST /api/v1/projects/{projectID}/members/{userID}/disable-login":         "200",
	"POST /api/v1/projects/{projectID}/members/{userID}/enable-login":          "200",
	"GET /api/v1/projects/{projectID}/api-keys":                                "200",
	"HEAD /api/v1/projects/{projectID}/api-keys":                               "200",
	"POST /api/v1/projects/{projectID}/api-keys":                               "201",
	"POST /api/v1/projects/{projectID}/api-keys/{keyID}/rotate":                "200",
	"POST /api/v1/projects/{projectID}/api-keys/{keyID}/revoke":                "200",
	"GET /api/v1/projects/{projectID}/articles":                                "200",
	"HEAD /api/v1/projects/{projectID}/articles":                               "200",
	"POST /api/v1/projects/{projectID}/articles":                               "201",
	"GET /api/v1/projects/{projectID}/articles/{articleID}":                    "200",
	"HEAD /api/v1/projects/{projectID}/articles/{articleID}":                   "200",
	"GET /api/v1/projects/{projectID}/articles/{articleID}/autosave":           "200",
	"HEAD /api/v1/projects/{projectID}/articles/{articleID}/autosave":          "200",
	"PUT /api/v1/projects/{projectID}/articles/{articleID}/autosave":           "200",
	"DELETE /api/v1/projects/{projectID}/articles/{articleID}/autosave":        "204",
	"DELETE /api/v1/projects/{projectID}/articles/{articleID}":                 "204",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/restore":           "200",
	"POST /api/v1/projects/{projectID}/revisions/{revisionID}/submit":          "200",
	"POST /api/v1/projects/{projectID}/revisions/{revisionID}/request-changes": "200",
	"POST /api/v1/projects/{projectID}/revisions/{revisionID}/approve":         "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/publish":           "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/schedule":          "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/unpublish":         "200",
	"GET /api/v1/projects/{projectID}/categories":                              "200",
	"HEAD /api/v1/projects/{projectID}/categories":                             "200",
	"POST /api/v1/projects/{projectID}/categories":                             "201",
	"PATCH /api/v1/projects/{projectID}/categories/{termID}":                   "200",
	"GET /api/v1/projects/{projectID}/tags":                                    "200",
	"HEAD /api/v1/projects/{projectID}/tags":                                   "200",
	"POST /api/v1/projects/{projectID}/tags":                                   "201",
	"GET /api/v1/projects/{projectID}/authors":                                 "200",
	"HEAD /api/v1/projects/{projectID}/authors":                                "200",
	"POST /api/v1/projects/{projectID}/authors":                                "201",
	"GET /api/v1/projects/{projectID}/authors/{authorID}":                      "200",
	"HEAD /api/v1/projects/{projectID}/authors/{authorID}":                     "200",
	"PATCH /api/v1/projects/{projectID}/authors/{authorID}":                    "200",
	"DELETE /api/v1/projects/{projectID}/authors/{authorID}":                   "200",
	"GET /api/v1/projects/{projectID}/series":                                  "200",
	"HEAD /api/v1/projects/{projectID}/series":                                 "200",
	"POST /api/v1/projects/{projectID}/series":                                 "201",
	"GET /api/v1/projects/{projectID}/media":                                   "200",
	"HEAD /api/v1/projects/{projectID}/media":                                  "200",
	"POST /api/v1/projects/{projectID}/media/uploads":                          "201",
	"GET /api/v1/projects/{projectID}/media/{assetID}/file":                    "200",
	"HEAD /api/v1/projects/{projectID}/media/{assetID}/file":                   "200",
	"GET /api/v1/projects/{projectID}/media/{assetID}":                         "200",
	"HEAD /api/v1/projects/{projectID}/media/{assetID}":                        "200",
	"POST /api/v1/projects/{projectID}/media/{assetID}/complete":               "200",
	"PATCH /api/v1/projects/{projectID}/media/{assetID}":                       "200",
	"DELETE /api/v1/projects/{projectID}/media/{assetID}":                      "204",
	"GET /api/v1/projects/{projectID}/ai/jobs":                                 "200",
	"HEAD /api/v1/projects/{projectID}/ai/jobs":                                "200",
	"POST /api/v1/projects/{projectID}/ai/jobs":                                "202",
	"GET /api/v1/projects/{projectID}/ai/jobs/{jobID}":                         "200",
	"HEAD /api/v1/projects/{projectID}/ai/jobs/{jobID}":                        "200",
	"POST /api/v1/projects/{projectID}/ai/jobs/{jobID}/cancel":                 "200",
	"GET /api/v1/projects/{projectID}/ai/jobs/{jobID}/events":                  "200",
	"HEAD /api/v1/projects/{projectID}/ai/jobs/{jobID}/events":                 "200",
	"GET /api/v1/projects/{projectID}/ai/runs":                                 "200",
	"HEAD /api/v1/projects/{projectID}/ai/runs":                                "200",
	"GET /api/v1/projects/{projectID}/quality-checks":                          "200",
	"HEAD /api/v1/projects/{projectID}/quality-checks":                         "200",
	"GET /api/v1/projects/{projectID}/review-assignees":                        "200",
	"HEAD /api/v1/projects/{projectID}/review-assignees":                       "200",
	"GET /api/v1/projects/{projectID}/articles/{articleID}/assignments":        "200",
	"HEAD /api/v1/projects/{projectID}/articles/{articleID}/assignments":       "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/assignments":       "201",
	"POST /api/v1/projects/{projectID}/assignments/{assignmentID}/complete":    "200",
	"POST /api/v1/projects/{projectID}/assignments/{assignmentID}/cancel":      "200",
	"GET /api/v1/projects/{projectID}/voice-profile":                           "200",
	"HEAD /api/v1/projects/{projectID}/voice-profile":                          "200",
	"POST /api/v1/projects/{projectID}/voice-profile":                          "201",
	"GET /api/v1/projects/{projectID}/evidence-packets":                        "200",
	"HEAD /api/v1/projects/{projectID}/evidence-packets":                       "200",
	"POST /api/v1/projects/{projectID}/evidence-packets":                       "201",
	"POST /api/v1/projects/{projectID}/evidence-packets/{packetID}/approve":    "200",
	"GET /api/v1/projects/{projectID}/sources":                                 "200",
	"HEAD /api/v1/projects/{projectID}/sources":                                "200",
	"POST /api/v1/projects/{projectID}/sources":                                "201",
	"PATCH /api/v1/projects/{projectID}/sources/{sourceID}":                    "200",
	"GET /api/v1/projects/{projectID}/revisions/{revisionID}/claims":           "200",
	"HEAD /api/v1/projects/{projectID}/revisions/{revisionID}/claims":          "200",
	"POST /api/v1/projects/{projectID}/revisions/{revisionID}/claims":          "201",
	"POST /api/v1/projects/{projectID}/claims/{claimID}/verify":                "200",
	"GET /api/v1/projects/{projectID}/articles/{articleID}/comments":           "200",
	"HEAD /api/v1/projects/{projectID}/articles/{articleID}/comments":          "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/comments":          "201",
	"POST /api/v1/projects/{projectID}/comments/{commentID}/resolve":           "200",
	"POST /api/v1/projects/{projectID}/comments/{commentID}/reopen":            "200",
	"GET /api/v1/projects/{projectID}/articles/{articleID}/disclosures":        "200",
	"HEAD /api/v1/projects/{projectID}/articles/{articleID}/disclosures":       "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/disclosures":       "201",
	"GET /api/v1/projects/{projectID}/articles/{articleID}/corrections":        "200",
	"HEAD /api/v1/projects/{projectID}/articles/{articleID}/corrections":       "200",
	"POST /api/v1/projects/{projectID}/articles/{articleID}/corrections":       "201",
	"POST /api/v1/projects/{projectID}/preview-tokens":                         "201",
	"POST /api/v1/projects/{projectID}/preview-tokens/{tokenID}/revoke":        "200",
	"GET /api/v1/projects/{projectID}/webhook-attempts":                        "200",
	"HEAD /api/v1/projects/{projectID}/webhook-attempts":                       "200",
	"POST /api/v1/projects/{projectID}/webhook-attempts/{attemptID}/replay":    "202",
	"GET /api/v1/projects/{projectID}/webhooks":                                "200",
	"HEAD /api/v1/projects/{projectID}/webhooks":                               "200",
	"POST /api/v1/projects/{projectID}/webhooks":                               "201",
	"POST /api/v1/projects/{projectID}/webhooks/{endpointID}/revoke":           "200",
	"GET /api/v1/projects/{projectID}/audit-events":                            "200",
	"HEAD /api/v1/projects/{projectID}/audit-events":                           "200",
	"GET /api/v1/projects/{projectID}/delivery/status":                         "200",
	"HEAD /api/v1/projects/{projectID}/delivery/status":                        "200",
}

// documentFiberRoutes adds the Fiber-owned routes to the same OpenAPI document
// as the Huma-owned endpoints. Runtime routing remains in Fiber while the
// contract stays complete and available at /openapi.json and /openapi.yaml.
func documentFiberRoutes(api huma.API, app *fiber.App) {
	documentPasswordResetRoutes(api)
	documentAPIKeyRoutes(api)
	documentArticleManagementRoutes(api)
	documentRollbackRoute(api)
	documentCopyArticleRoute(api)
	documentRevisionHistoryRoutes(api)

	for _, methodRoutes := range app.Stack() {
		for _, route := range methodRoutes {
			if route.Path == "/healthz" ||
				route.Path == "/metrics" ||
				strings.HasPrefix(route.Path, "/openapi") ||
				strings.HasPrefix(route.Path, "/docs") ||
				strings.HasPrefix(route.Path, "/schemas") ||
				strings.Contains(route.Path, "*") {
				continue
			}
			method := strings.ToUpper(route.Method)
			if method != http.MethodGet &&
				method != http.MethodPost &&
				method != http.MethodPut &&
				method != http.MethodPatch &&
				method != http.MethodDelete &&
				method != http.MethodHead &&
				method != http.MethodOptions {
				continue
			}
			if method == http.MethodHead && route.Path == "/" {
				continue
			}
			path := fiberParameter.ReplaceAllString(route.Path, `{$1}`)
			item := api.OpenAPI().Paths[path]
			if item != nil && operationForMethod(item, method) != nil {
				continue
			}
			operationID := strings.Trim(operationIDPart.ReplaceAllString(strings.ToLower(method+"_"+path), "_"), "_")
			successStatus := "200"
			implementedStatus, implemented := implementedAdminRouteStatuses[method+" "+path]
			if implemented {
				successStatus = implementedStatus
			}
			responses := map[string]*huma.Response{
				successStatus: {Description: "Successful response"},
				"400":         {Description: "Invalid request"},
				"401":         {Description: "Authentication required"},
				"403":         {Description: "Insufficient permission"},
				"404":         {Description: "Resource not found"},
				"500":         {Description: "Internal server error"},
			}
			description := ""
			if strings.HasPrefix(path, "/api/v1/") {
				description = "Administrative API route."
			}
			if strings.HasPrefix(path, "/api/v1/") && !implemented {
				responses["501"] = &huma.Response{Description: "Administrative workflow is not implemented yet"}
			}
			api.OpenAPI().AddOperation(&huma.Operation{
				Method:      method,
				Path:        path,
				OperationID: operationID,
				Summary:     method + " " + path,
				Description: description,
				Tags:        []string{routeTag(path)},
				Responses:   responses,
			})
			// Fiber v3 automatically serves HEAD for GET routes without adding a
			// synthetic HEAD entry to App.Stack. Keep the generated contract in
			// sync with that runtime behavior.
			if method == http.MethodGet {
				pathItem := api.OpenAPI().Paths[path]
				if pathItem != nil && operationForMethod(pathItem, http.MethodHead) == nil {
					headResponses := map[string]*huma.Response{
						"200": {Description: "Successful response without a body"},
						"400": {Description: "Invalid request"},
						"401": {Description: "Authentication required"},
						"403": {Description: "Insufficient permission"},
						"404": {Description: "Resource not found"},
						"500": {Description: "Internal server error"},
					}
					api.OpenAPI().AddOperation(&huma.Operation{
						Method:      http.MethodHead,
						Path:        path,
						OperationID: strings.Trim(operationIDPart.ReplaceAllString(strings.ToLower(http.MethodHead+"_"+path), "_"), "_"),
						Summary:     http.MethodHead + " " + path,
						Description: description,
						Tags:        []string{routeTag(path)},
						Responses:   headResponses,
					})
				}
			}
		}
	}
}

func documentAPIKeyRoutes(api huma.API) {
	openAPI := api.OpenAPI()
	documentAdminSessionSecurity(openAPI)
	registry := openAPI.Components.Schemas
	metadataSchema := registry.Schema(reflect.TypeOf(store.AdminAPIKey{}), true, "AdminAPIKey")
	listSchema := registry.Schema(reflect.TypeOf(ListEnvelope[store.AdminAPIKey]{}), true, "APIKeyListResponse")
	secretSchema := registry.Schema(reflect.TypeOf(Envelope[store.APIKeyWithSecret]{}), true, "APIKeySecretResponse")
	keySchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminAPIKey]{}), true, "APIKeyResponse")
	requestSchema := registry.Schema(reflect.TypeOf(apiKeyRequest{}), true, "CreateAPIKeyRequest")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")
	resolvedMetadataSchema := metadataSchema
	if metadataSchema.Ref != "" {
		resolvedMetadataSchema = registry.SchemaFromRef(metadataSchema.Ref)
	}
	if status := resolvedMetadataSchema.Properties["status"]; status != nil {
		status.Enum = []any{"active", "expired", "revoked"}
	}
	if environment := resolvedMetadataSchema.Properties["environment"]; environment != nil {
		environment.Enum = []any{"production", "staging", "development", "preview"}
	}
	resolvedRequestSchema := requestSchema
	if requestSchema.Ref != "" {
		resolvedRequestSchema = registry.SchemaFromRef(requestSchema.Ref)
	}
	resolvedRequestSchema.Required = []string{"name"}
	if name := resolvedRequestSchema.Properties["name"]; name != nil {
		name.MinLength = intPointer(1)
		name.MaxLength = intPointer(100)
	}
	if environment := resolvedRequestSchema.Properties["environment"]; environment != nil {
		environment.Enum = []any{"production", "staging", "development", "preview"}
	}
	if scopes := resolvedRequestSchema.Properties["scopes"]; scopes != nil && scopes.Items != nil {
		scopes.Items.Enum = []any{"content:published:read", "taxonomy:published:read", "authors:published:read", "discovery:read", "redirects:read"}
	}

	problemResponse := func(description string) *huma.Response {
		return &huma.Response{Description: description, Content: map[string]*huma.MediaType{
			problemMediaType: {Schema: problemSchema},
		}}
	}
	projectParameter := &huma.Param{Name: "projectID", In: "path", Description: "Project identifier", Required: true, Schema: &huma.Schema{Type: "string"}}
	keyParameter := &huma.Param{Name: "keyID", In: "path", Description: "API key identifier", Required: true, Schema: &huma.Schema{Type: "string"}}
	csrfParameter := &huma.Param{Name: "X-CSRF-Token", In: "header", Description: "Administrative session CSRF token", Required: true, Schema: &huma.Schema{Type: "string"}}
	mutationResponses := func(successDescription string, successSchema *huma.Schema) map[string]*huma.Response {
		return map[string]*huma.Response{
			"200": {Description: successDescription, Content: map[string]*huma.MediaType{"application/json": {Schema: successSchema}}},
			"400": problemResponse("Invalid request"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Management permission or recent reauthentication required"),
			"404": problemResponse("Project or API key not found"),
			"409": problemResponse("Project or API key state does not allow this operation"),
			"500": problemResponse("Internal server error"),
		}
	}

	listParameters := []*huma.Param{
		projectParameter,
		{Name: "cursor", In: "query", Description: "Opaque API-key list cursor", Schema: &huma.Schema{Type: "string"}},
		{Name: "limit", In: "query", Description: "Page size, up to 100", Schema: &huma.Schema{Type: "integer", Minimum: float64Pointer(1), Maximum: float64Pointer(100)}},
	}
	openAPI.AddOperation(&huma.Operation{
		Method: http.MethodGet, Path: "/api/v1/projects/{projectID}/api-keys", OperationID: "listProjectAPIKeys",
		Summary: "List project API keys", Description: "Returns key metadata and server-derived status. Raw secrets and stored verifiers are never returned.",
		Tags: []string{"Administration"}, Parameters: listParameters, Security: adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "API key page", Content: map[string]*huma.MediaType{"application/json": {Schema: listSchema}}},
			"401": problemResponse("Authentication required"), "403": problemResponse("Management permission required"),
			"404": problemResponse("Project not found"), "500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method: http.MethodHead, Path: "/api/v1/projects/{projectID}/api-keys", OperationID: "headProjectAPIKeys",
		Summary: "Check the project API key list", Tags: []string{"Administration"}, Parameters: listParameters, Security: adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "The API key list is available"}, "401": {Description: "Authentication required"},
			"403": {Description: "Management permission required"}, "404": {Description: "Project not found"}, "500": {Description: "Internal server error"},
		},
	})
	createResponses := mutationResponses("API key created; the secret is returned exactly once", secretSchema)
	createResponses["201"] = createResponses["200"]
	delete(createResponses, "200")
	openAPI.AddOperation(&huma.Operation{
		Method: http.MethodPost, Path: "/api/v1/projects/{projectID}/api-keys", OperationID: "createProjectAPIKey",
		Summary: "Create a project API key", Description: "Creates a scoped server credential after recent human reauthentication. Store the returned secret immediately because it cannot be retrieved later.",
		Tags: []string{"Administration"}, Parameters: []*huma.Param{projectParameter, csrfParameter}, Security: adminSessionSecurityRequirement(),
		RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"application/json": {Schema: requestSchema}}}, Responses: createResponses,
	})
	openAPI.AddOperation(&huma.Operation{
		Method: http.MethodPost, Path: "/api/v1/projects/{projectID}/api-keys/{keyID}/rotate", OperationID: "rotateProjectAPIKey",
		Summary: "Rotate a project API key", Description: "Creates a replacement with the same environment, scopes, and expiration. The previous key remains active for a zero-downtime deployment and must be revoked separately.",
		Tags: []string{"Administration"}, Parameters: []*huma.Param{projectParameter, keyParameter, csrfParameter}, Security: adminSessionSecurityRequirement(),
		Responses: mutationResponses("Replacement API key and one-time secret", secretSchema),
	})
	openAPI.AddOperation(&huma.Operation{
		Method: http.MethodPost, Path: "/api/v1/projects/{projectID}/api-keys/{keyID}/revoke", OperationID: "revokeProjectAPIKey",
		Summary: "Revoke a project API key", Description: "Immediately and permanently disables the selected credential after recent human reauthentication.",
		Tags: []string{"Administration"}, Parameters: []*huma.Param{projectParameter, keyParameter, csrfParameter}, Security: adminSessionSecurityRequirement(),
		Responses: mutationResponses("Revoked API key metadata", keySchema),
	})
}

func documentArticleManagementRoutes(api huma.API) {
	openAPI := api.OpenAPI()
	documentAdminSessionSecurity(openAPI)
	registry := openAPI.Components.Schemas
	listSchema := registry.Schema(reflect.TypeOf(ListEnvelope[store.AdminArticle]{}), true, "AdminArticleListResponse")
	articleSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminArticle]{}), true, "AdminArticleResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")
	problemResponse := func(description string) *huma.Response {
		return &huma.Response{
			Description: description,
			Content: map[string]*huma.MediaType{
				problemMediaType: {Schema: problemSchema},
			},
		}
	}
	projectParameter := &huma.Param{
		Name:        "projectID",
		In:          "path",
		Description: "Project identifier",
		Required:    true,
		Schema:      &huma.Schema{Type: "string"},
	}
	listParameters := []*huma.Param{
		projectParameter,
		{
			Name:        "cursor",
			In:          "query",
			Description: "Opaque article-list cursor",
			Schema:      &huma.Schema{Type: "string"},
		},
		{
			Name:        "limit",
			In:          "query",
			Description: "Page size, up to 100",
			Schema:      &huma.Schema{Type: "integer", Minimum: float64Pointer(1), Maximum: float64Pointer(100)},
		},
		{
			Name:        "q",
			In:          "query",
			Description: "Case-insensitive title, slug, or article-type search; wildcard characters are treated literally",
			Schema:      &huma.Schema{Type: "string", MaxLength: intPointer(100)},
		},
		{
			Name:        "editorialState",
			In:          "query",
			Description: "Exact latest-revision editorial state",
			Schema:      &huma.Schema{Type: "string", Enum: []any{"draft", "in_review", "changes_requested", "approved"}},
		},
		{
			Name:        "publicationState",
			In:          "query",
			Description: "Exact publication state; archived automatically includes archived records",
			Schema:      &huma.Schema{Type: "string", Enum: []any{"unpublished", "scheduled", "published", "archived"}},
		},
		{
			Name:        "includeArchived",
			In:          "query",
			Description: "Include archived articles alongside active records",
			Schema:      &huma.Schema{Type: "boolean"},
		},
	}
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/projects/{projectID}/articles",
		OperationID: "listAdminArticles",
		Summary:     "List project articles",
		Description: "Returns a project-scoped, cursor-paginated article list with allowlisted server-side filters.",
		Tags:        []string{"Administration"},
		Parameters:  listParameters,
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "Article page", Content: map[string]*huma.MediaType{"application/json": {Schema: listSchema}}},
			"400": problemResponse("Invalid filter or pagination input"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Project not found"),
			"500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodHead,
		Path:        "/api/v1/projects/{projectID}/articles",
		OperationID: "headAdminArticles",
		Summary:     "Check the project article list",
		Tags:        []string{"Administration"},
		Parameters:  listParameters,
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "The article list is available"},
			"400": {Description: "Invalid filter or pagination input"},
			"401": {Description: "Authentication required"},
			"403": {Description: "Insufficient permission"},
			"404": {Description: "Project not found"},
			"500": {Description: "Internal server error"},
		},
	})

	restoreParameters := []*huma.Param{
		projectParameter,
		{
			Name:        "articleID",
			In:          "path",
			Description: "Archived article identifier",
			Required:    true,
			Schema:      &huma.Schema{Type: "string"},
		},
		{
			Name:        "X-CSRF-Token",
			In:          "header",
			Description: "Administrative session CSRF token",
			Required:    true,
			Schema:      &huma.Schema{Type: "string"},
		},
	}
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/restore",
		OperationID: "restoreArchivedArticle",
		Summary:     "Restore an archived article",
		Description: "Restores retained content and revisions without republishing it. The publication state becomes unpublished.",
		Tags:        []string{"Administration"},
		Parameters:  restoreParameters,
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "Restored unpublished article", Content: map[string]*huma.MediaType{"application/json": {Schema: articleSchema}}},
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Publishing permission required"),
			"404": problemResponse("Archived article not found"),
			"409": problemResponse("Project is not active"),
			"500": problemResponse("Internal server error"),
		},
	})
}

func intPointer(value int) *int {
	return &value
}

func documentPasswordResetRoutes(api huma.API) {
	openAPI := api.OpenAPI()
	registry := openAPI.Components.Schemas
	forgotRequestSchema := registry.Schema(reflect.TypeOf(forgotPasswordRequest{}), true, "ForgotPasswordRequest")
	resetRequestSchema := registry.Schema(reflect.TypeOf(resetPasswordRequest{}), true, "ResetPasswordRequest")
	responseSchema := registry.Schema(reflect.TypeOf(passwordResetResponse{}), true, "PasswordResetResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")

	for _, item := range []struct {
		schema   *huma.Schema
		required []string
	}{
		{schema: forgotRequestSchema, required: []string{"email"}},
		{schema: resetRequestSchema, required: []string{"token", "password"}},
	} {
		resolved := item.schema
		if item.schema.Ref != "" {
			resolved = registry.SchemaFromRef(item.schema.Ref)
		}
		resolved.Required = item.required
	}
	problemResponse := func(description string) *huma.Response {
		return &huma.Response{
			Description: description,
			Content: map[string]*huma.MediaType{
				problemMediaType: {Schema: problemSchema},
			},
		}
	}

	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/forgot-password",
		OperationID: "requestPasswordReset",
		Summary:     "Request a password reset",
		Description: "Creates a one-time reset token for an active account and sends it by email. The response is identical when the address is unknown.",
		Tags:        []string{"Authentication"},
		RequestBody: &huma.RequestBody{
			Description: "Account email address.",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: forgotRequestSchema},
			},
		},
		Responses: map[string]*huma.Response{
			"202": {
				Description: "Password-reset request accepted",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: responseSchema},
				},
			},
			"400": problemResponse("Invalid request body"),
			"429": problemResponse("Password-reset rate limit exceeded"),
			"500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/reset-password",
		OperationID: "completePasswordReset",
		Summary:     "Complete a password reset",
		Description: "Consumes an unexpired one-time reset token, changes the password and revokes every active session for the account.",
		Tags:        []string{"Authentication"},
		RequestBody: &huma.RequestBody{
			Description: "One-time reset token and replacement password.",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: resetRequestSchema},
			},
		},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Password reset completed",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: responseSchema},
				},
			},
			"400": problemResponse("Invalid request, password or reset token"),
			"429": problemResponse("Password-reset rate limit exceeded"),
			"500": problemResponse("Internal server error"),
		},
	})
}

func documentRevisionHistoryRoutes(api huma.API) {
	openAPI := api.OpenAPI()
	documentAdminSessionSecurity(openAPI)
	registry := openAPI.Components.Schemas
	listSchema := registry.Schema(reflect.TypeOf(ListEnvelope[store.AdminRevisionSummary]{}), true, "RevisionHistoryResponse")
	detailSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminRevisionDetail]{}), true, "RevisionDetailResponse")
	createRequestSchema := registry.Schema(reflect.TypeOf(revisionRequest{}), true, "CreateRevisionRequest")
	createResponseSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminRevision]{}), true, "CreateRevisionResponse")
	autosaveRequestSchema := registry.Schema(reflect.TypeOf(articleAutosaveRequest{}), true, "ArticleAutosaveRequest")
	autosaveResponseSchema := registry.Schema(reflect.TypeOf(Envelope[store.ArticleAutosave]{}), true, "ArticleAutosaveResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")
	resolvedCreateRequestSchema := createRequestSchema
	if createRequestSchema.Ref != "" {
		resolvedCreateRequestSchema = registry.SchemaFromRef(createRequestSchema.Ref)
	}
	resolvedCreateRequestSchema.Required = []string{"baseRevisionId", "title"}
	resolvedAutosaveRequestSchema := autosaveRequestSchema
	if autosaveRequestSchema.Ref != "" {
		resolvedAutosaveRequestSchema = registry.SchemaFromRef(autosaveRequestSchema.Ref)
	}
	resolvedAutosaveRequestSchema.Required = []string{"baseRevisionId", "expectedVersion", "draft"}
	if expectedVersionSchema := resolvedAutosaveRequestSchema.Properties["expectedVersion"]; expectedVersionSchema != nil {
		expectedVersionSchema.Nullable = false
	}

	problemResponse := func(description string) *huma.Response {
		return &huma.Response{
			Description: description,
			Content: map[string]*huma.MediaType{
				problemMediaType: {Schema: problemSchema},
			},
		}
	}
	pathParameters := func(includeRevision bool) []*huma.Param {
		parameters := []*huma.Param{
			{
				Name:        "projectID",
				In:          "path",
				Description: "Project identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "articleID",
				In:          "path",
				Description: "Article identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
		}
		if includeRevision {
			parameters = append(parameters, &huma.Param{
				Name:        "revisionID",
				In:          "path",
				Description: "Revision identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			})
		}
		return parameters
	}

	listParameters := pathParameters(false)
	listParameters = append(listParameters,
		&huma.Param{
			Name:        "cursor",
			In:          "query",
			Description: "Opaque revision-history cursor",
			Schema:      &huma.Schema{Type: "string"},
		},
		&huma.Param{
			Name:        "limit",
			In:          "query",
			Description: "Page size, up to 100",
			Schema:      &huma.Schema{Type: "integer", Minimum: float64Pointer(1), Maximum: float64Pointer(100)},
		},
	)
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/revisions",
		OperationID: "listArticleRevisions",
		Summary:     "List article revision history",
		Description: "Lists immutable revisions newest first, including base-revision and current publication metadata.",
		Tags:        []string{"Administration"},
		Parameters:  listParameters,
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Revision history",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: listSchema},
				},
			},
			"400": problemResponse("Invalid pagination cursor"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Project or article not found"),
			"500": problemResponse("Internal server error"),
		},
	})

	autosavePath := "/api/v1/projects/{projectID}/articles/{articleID}/autosave"
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodGet,
		Path:        autosavePath,
		OperationID: "getArticleAutosave",
		Summary:     "Get the current user's article autosave",
		Description: "Returns the current user's project-scoped working draft and whether its immutable base revision is stale.",
		Tags:        []string{"Administration"},
		Parameters:  pathParameters(false),
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Article autosave",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: autosaveResponseSchema},
				},
			},
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Autosave not found"),
			"500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodHead,
		Path:        autosavePath,
		OperationID: "headArticleAutosave",
		Summary:     "Check the current user's article autosave",
		Description: "Returns the autosave GET status and headers without a response body.",
		Tags:        []string{"Administration"},
		Parameters:  pathParameters(false),
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "Article autosave is available"},
			"401": {Description: "Authentication required"},
			"403": {Description: "Insufficient permission"},
			"404": {Description: "Autosave not found"},
			"500": {Description: "Internal server error"},
		},
	})
	mutationParameters := append(pathParameters(false), &huma.Param{
		Name:        "X-CSRF-Token",
		In:          "header",
		Description: "Administrative session CSRF token",
		Required:    true,
		Schema:      &huma.Schema{Type: "string"},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPut,
		Path:        autosavePath,
		OperationID: "saveArticleAutosave",
		Summary:     "Save the current user's article working draft",
		Description: "Upserts a user-scoped working draft using immutable base-revision and optimistic autosave-version guards.",
		Tags:        []string{"Administration"},
		Parameters:  mutationParameters,
		RequestBody: &huma.RequestBody{
			Description: "Base revision, expected autosave version and recoverable working fields.",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: autosaveRequestSchema},
			},
		},
		Security: adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Article autosave stored",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: autosaveResponseSchema},
				},
			},
			"400": problemResponse("Invalid autosave input"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Project, article or referenced field not found"),
			"409": problemResponse("Autosave base or version is stale"),
			"500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodDelete,
		Path:        autosavePath,
		OperationID: "deleteArticleAutosave",
		Summary:     "Delete the current user's article working draft",
		Description: "Idempotently removes only the authenticated user's autosave for the selected project article.",
		Tags:        []string{"Administration"},
		Parameters:  mutationParameters,
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"204": {Description: "Article autosave deleted"},
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodHead,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/revisions",
		OperationID: "headArticleRevisions",
		Summary:     "Check article revision history",
		Description: "Returns the revision-history GET status and headers without a response body.",
		Tags:        []string{"Administration"},
		Parameters:  listParameters,
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "Revision history is available"},
			"400": {Description: "Invalid pagination cursor"},
			"401": {Description: "Authentication required"},
			"403": {Description: "Insufficient permission"},
			"404": {Description: "Project or article not found"},
			"500": {Description: "Internal server error"},
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/revisions",
		OperationID: "createArticleRevision",
		Summary:     "Create an article revision",
		Description: "Creates an immutable draft revision from the current base revision. A stale base revision is rejected.",
		Tags:        []string{"Administration"},
		Parameters: []*huma.Param{
			{
				Name:        "projectID",
				In:          "path",
				Description: "Project identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "articleID",
				In:          "path",
				Description: "Article identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "X-CSRF-Token",
				In:          "header",
				Description: "Administrative session CSRF token",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
		},
		RequestBody: &huma.RequestBody{
			Description: "Current base revision and public fields for the new draft.",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: createRequestSchema},
			},
		},
		Security: adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"201": {
				Description: "Article revision created",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: createResponseSchema},
				},
			},
			"400": problemResponse("Invalid revision input"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Project, article, or taxonomy not found"),
			"409": problemResponse("Base revision is stale"),
			"500": problemResponse("Internal server error"),
		},
	})

	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/revisions/{revisionID}",
		OperationID: "getArticleRevision",
		Summary:     "Get an article revision",
		Description: "Returns the immutable public fields, structured body and derived rendering data for revision comparison.",
		Tags:        []string{"Administration"},
		Parameters:  pathParameters(true),
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Revision detail",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: detailSchema},
				},
			},
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Project, article, or revision not found"),
			"500": problemResponse("Internal server error"),
		},
	})
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodHead,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/revisions/{revisionID}",
		OperationID: "headArticleRevision",
		Summary:     "Check an article revision",
		Description: "Returns the revision-detail GET status and headers without a response body.",
		Tags:        []string{"Administration"},
		Parameters:  pathParameters(true),
		Security:    adminSessionSecurityRequirement(),
		Responses: map[string]*huma.Response{
			"200": {Description: "Revision exists"},
			"401": {Description: "Authentication required"},
			"403": {Description: "Insufficient permission"},
			"404": {Description: "Project, article, or revision not found"},
			"500": {Description: "Internal server error"},
		},
	})
}

func documentRollbackRoute(api huma.API) {
	openAPI := api.OpenAPI()
	documentAdminSessionSecurity(openAPI)
	registry := openAPI.Components.Schemas
	requestSchema := registry.Schema(reflect.TypeOf(rollbackRequest{}), true, "RollbackArticleRequest")
	responseSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminArticle]{}), true, "RollbackArticleResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")

	problemResponse := func(description string) *huma.Response {
		return &huma.Response{
			Description: description,
			Content: map[string]*huma.MediaType{
				problemMediaType: {Schema: problemSchema},
			},
		}
	}
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/rollback",
		OperationID: "rollbackArticle",
		Summary:     "Rollback an article",
		Description: "Publishes a previously approved revision while preserving publication routing metadata and revision history.",
		Tags:        []string{"Administration"},
		Security:    adminSessionSecurityRequirement(),
		Parameters: []*huma.Param{
			{
				Name:        "projectID",
				In:          "path",
				Description: "Project identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "articleID",
				In:          "path",
				Description: "Article identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "X-CSRF-Token",
				In:          "header",
				Description: "Administrative session CSRF token",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
		},
		RequestBody: &huma.RequestBody{
			Description: "Approved revision to restore.",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: requestSchema},
			},
		},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Article publication rolled back",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: responseSchema},
				},
			},
			"400": problemResponse("Invalid request"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient permission"),
			"404": problemResponse("Project, article, or revision not found"),
			"409": problemResponse("Article or revision is not in a rollback-compatible state"),
			"500": problemResponse("Internal server error"),
		},
	})
}

func documentCopyArticleRoute(api huma.API) {
	openAPI := api.OpenAPI()
	documentAdminSessionSecurity(openAPI)
	registry := openAPI.Components.Schemas
	requestSchema := registry.Schema(reflect.TypeOf(copyArticleRequest{}), true, "CopyArticleRequest")
	responseSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminArticle]{}), true, "CopyArticleResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")
	resolvedRequestSchema := requestSchema
	if requestSchema.Ref != "" {
		resolvedRequestSchema = registry.SchemaFromRef(requestSchema.Ref)
	}
	resolvedRequestSchema.Required = []string{
		"destinationProjectId",
		"sourceRevisionId",
		"primaryCategoryId",
		"slug",
		"canonicalDecision",
		"contributorMappings",
	}
	if decision := resolvedRequestSchema.Properties["canonicalDecision"]; decision != nil {
		decision.Enum = []any{"canonical_original", "material_adaptation"}
		decision.Description = "Use the selected source revision's canonical URL, or create a destination-owned material adaptation."
	}
	if originalURL := resolvedRequestSchema.Properties["canonicalOriginalUrl"]; originalURL != nil {
		originalURL.Format = "uri"
		originalURL.Description = "Optional assertion for canonical_original. When supplied, it must match the selected source revision's source canonical URL; the server always derives the stored canonical from the source publication."
	}

	problemResponse := func(description string) *huma.Response {
		return &huma.Response{
			Description: description,
			Content: map[string]*huma.MediaType{
				problemMediaType: {Schema: problemSchema},
			},
		}
	}
	openAPI.AddOperation(&huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/{projectID}/articles/{articleID}/copy-to-project",
		OperationID: "copyArticleToProject",
		Summary:     "Copy an article to another project",
		Description: "Creates an independent destination draft from an exact source revision. The caller needs source access and destination content-create permission, must explicitly map every source contributor to an active destination author, and must record a canonical-original or material-adaptation decision. Canonical-original URLs are derived from the selected source revision's publication.",
		Tags:        []string{"Administration"},
		Security:    adminSessionSecurityRequirement(),
		Parameters: []*huma.Param{
			{
				Name:        "projectID",
				In:          "path",
				Description: "Source project identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "articleID",
				In:          "path",
				Description: "Source article identifier",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
			{
				Name:        "X-CSRF-Token",
				In:          "header",
				Description: "Administrative session CSRF token",
				Required:    true,
				Schema:      &huma.Schema{Type: "string"},
			},
		},
		RequestBody: &huma.RequestBody{
			Description: "Destination ownership, taxonomy, routing and canonical/adaptation decision.",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: requestSchema},
			},
		},
		Responses: map[string]*huma.Response{
			"201": {
				Description: "Independent destination article draft created",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: responseSchema},
				},
			},
			"400": problemResponse("Invalid copy input or destination routing conflict"),
			"401": problemResponse("Authentication required"),
			"403": problemResponse("Insufficient destination permission"),
			"404": problemResponse("Source, destination, revision, or taxonomy not found"),
			"409": problemResponse("Source or destination project is not active"),
			"500": problemResponse("Internal server error"),
		},
	})
}

func documentAdminSessionSecurity(openAPI *huma.OpenAPI) {
	if openAPI.Components.SecuritySchemes == nil {
		openAPI.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	openAPI.Components.SecuritySchemes[adminSessionSecuritySchemeName] = &huma.SecurityScheme{
		Type:        "apiKey",
		Description: "Administrative session cookie issued by the login endpoint.",
		Name:        sessionCookieName,
		In:          "cookie",
	}
}

func adminSessionSecurityRequirement() []map[string][]string {
	return []map[string][]string{
		{adminSessionSecuritySchemeName: {}},
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}

func operationForMethod(item *huma.PathItem, method string) *huma.Operation {
	switch method {
	case http.MethodGet:
		return item.Get
	case http.MethodPost:
		return item.Post
	case http.MethodPatch:
		return item.Patch
	case http.MethodPut:
		return item.Put
	case http.MethodDelete:
		return item.Delete
	case http.MethodHead:
		return item.Head
	case http.MethodOptions:
		return item.Options
	default:
		return nil
	}
}

func routeTag(path string) string {
	switch {
	case strings.HasPrefix(path, "/content/"):
		return "Content API"
	case strings.HasPrefix(path, "/api/v1/auth"):
		return "Authentication"
	case strings.HasPrefix(path, "/api/"):
		return "Administration"
	default:
		return "Operations"
	}
}
