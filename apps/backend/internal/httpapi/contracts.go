package httpapi

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
)

var fiberParameter = regexp.MustCompile(`:([A-Za-z][A-Za-z0-9_]*)`)
var operationIDPart = regexp.MustCompile(`[^A-Za-z0-9]+`)

// documentFiberRoutes adds the Fiber-owned routes to the same OpenAPI document
// as the Huma-owned endpoints. Runtime routing remains in Fiber while the
// contract stays complete and available at /openapi.json and /openapi.yaml.
func documentFiberRoutes(api huma.API, app *fiber.App) {
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
