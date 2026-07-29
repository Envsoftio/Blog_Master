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

// documentFiberRoutes adds the Fiber-owned routes to the same OpenAPI document
// as the Huma-owned endpoints. Runtime routing remains in Fiber while the
// contract stays complete and available at /openapi.json and /openapi.yaml.
func documentFiberRoutes(api huma.API, app *fiber.App) {
	documentRollbackRoute(api)

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

func documentRollbackRoute(api huma.API) {
	openAPI := api.OpenAPI()
	registry := openAPI.Components.Schemas
	requestSchema := registry.Schema(reflect.TypeOf(rollbackRequest{}), true, "RollbackArticleRequest")
	responseSchema := registry.Schema(reflect.TypeOf(Envelope[store.AdminArticle]{}), true, "RollbackArticleResponse")
	problemSchema := registry.Schema(reflect.TypeOf(Problem{}), true, "Problem")

	problemResponse := func(description string) *huma.Response {
		return &huma.Response{
			Description: description,
			Content: map[string]*huma.MediaType{
				"application/problem+json": {Schema: problemSchema},
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
