package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// Project type operations keep the v1 nested definitions and native values
// separate from v2 project definitions and type-property attachments.
func projectTypesHandlers() map[string]handler {
	handlers := make(map[string]handler)
	actions := map[string][]string{
		"work-item-type":            {"list", "get", "get-schema", "create", "update", "delete", "enable", "import", "mark-default"},
		"work-item-property":        {"list", "get", "create", "update", "delete"},
		"work-item-property-option": {"list", "get", "create", "update", "delete"},
		"work-item-type-property":   {"list", "get", "attach", "detach"},
		"work-item-property-value":  {"get", "upsert", "update", "delete"},
	}
	for group, verbs := range actions {
		for _, action := range verbs {
			handlers[group+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version := flags["api-version"].(string)
				path := []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string)}
				query := url.Values{}
				if values, ok := input["query"].(map[string]any); ok {
					for key, value := range values {
						query.Set(key, fmt.Sprint(value))
					}
				}
				switch group {
				case "work-item-type":
					path = append(path, "work-item-types")
					if action == "get-schema" {
						if version == "v2" {
							path = append(path, flags["id"].(string))
						}
						path = append(path, "schema")
					}
					if version == "v1" && action == "get-schema" {
						if id, ok := flags["id"].(string); ok {
							query.Set("type_id", id)
						}
					}
				case "work-item-property":
					if version == "v1" {
						path = append(path, "work-item-types", flags["type"].(string))
					}
					path = append(path, "work-item-properties")
				case "work-item-property-option":
					path = append(path, "work-item-properties", flags["property"].(string), "options")
				case "work-item-type-property":
					path = append(path, "work-item-types", flags["type"].(string), "properties")
				case "work-item-property-value":
					path = append(path, "work-items", flags["work-item"].(string), "work-item-properties", flags["property"].(string), "values")
				}
				if group != "work-item-property-value" && (action == "get" || action == "update" || action == "delete" || action == "detach" || action == "mark-default") {
					path = append(path, flags["id"].(string))
				}
				method := http.MethodGet
				switch action {
				case "create", "attach", "upsert":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete", "detach":
					method = http.MethodDelete
				case "enable", "import", "mark-default":
					method = http.MethodPost
					path = append(path, action)
				}
				body, err := resourceBody(cmd, input)
				if err != nil {
					return err
				}
				return apiRequest(cmd, method, path, query, body, action == "delete" || action == "detach")
			}
		}
	}
	return handlers
}
