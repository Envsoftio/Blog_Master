package httpapi

import (
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"

	"seoblog/apps/backend/internal/store"
)

var fiberParameter = regexp.MustCompile(`:([A-Za-z][A-Za-z0-9_]*)`)
var operationIDPart = regexp.MustCompile(`[^A-Za-z0-9]+`)

const adminSessionSecuritySchemeName = "adminSession"

// documentFiberRoutes adds the Fiber-owned routes to the same OpenAPI document
// as the Huma-owned endpoints. Runtime routing remains in Fiber while the
// contract stays complete and available at /openapi.json and /openapi.yaml.
func documentFiberRoutes(api huma.API, app *fiber.App) {
	documentRollbackRoute(api)
	documentCopyArticleRoute(api)
	documentRevisionHistoryRoutes(api)

	for _, methodRoutes := range app.Stack() {
		for _, route := range methodRoutes {
			if route.Path == "/healthz" ||
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
			responses := map[string]*huma.Response{
				"200": {Description: "Successful response"},
				"400": {Description: "Invalid request"},
				"401": {Description: "Authentication required"},
				"403": {Description: "Insufficient permission"},
				"404": {Description: "Resource not found"},
				"500": {Description: "Internal server error"},
			}
			description := ""
			if strings.HasPrefix(path, "/api/v1/") {
				description = "Administrative API route."
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
		}
	}
}

func documentRevisionHistoryRoutes(api huma.API) {
	openAPI := api.OpenAPI()
	documentAdminSessionSecurity(openAPI)
	registry := openAPI.Components.Schemas
	listSchema := registry.Schema(reflect.TypeOf(ListEnvelope[store.AdminRevisionSummary]{}), true, "RevisionHistoryResponse")
	detailSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminRevisionDetail]{}), true, "RevisionDetailResponse")
	createRequestSchema := registry.Schema(reflect.TypeOf(revisionRequest{}), true, "CreateRevisionRequest")
	createResponseSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminRevision]{}), true, "CreateRevisionResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")
	resolvedCreateRequestSchema := createRequestSchema
	if createRequestSchema.Ref != "" {
		resolvedCreateRequestSchema = registry.SchemaFromRef(createRequestSchema.Ref)
	}
	resolvedCreateRequestSchema.Required = []string{"baseRevisionId", "title"}

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
		Description: "Publishes a previously approved revision for the selected locale while preserving publication routing metadata and revision history.",
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
			Description: "Approved revision and optional publication locale to restore.",
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
		Description: "Creates an independent destination draft from an exact source revision. The caller needs source access and destination content-create permission, and must record a canonical-original or material-adaptation decision. Canonical-original URLs are derived from the selected source revision's publication.",
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
