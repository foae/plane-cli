package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// contentPagesHandlers preserves each API generation's page and link routes.
// Catalog variants deliberately omit unresolved writes and v2 list contracts.
func contentPagesHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for group, actions := range map[string][]string{
		"project-page":   {"list", "get", "update", "archive", "restore", "delete"},
		"workspace-page": {"list", "get", "update", "archive", "restore", "delete"},
		"published-page": {"get"},
		"sticky":         {"list", "get", "delete"},
		"work-item-page": {"list", "get", "create", "delete"},
	} {
		for _, action := range actions {
			handlers[group+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version := flags["api-version"].(string)
				path := []string{"api", version, "workspaces", flags["workspace"].(string)}
				if group == "project-page" || group == "work-item-page" {
					path = append(path, "projects", flags["project"].(string))
				}
				if group == "work-item-page" {
					path = append(path, "work-items", flags["work-item"].(string))
				}
				switch group {
				case "sticky":
					path = append(path, "stickies")
				case "published-page":
					path = append(path, "published-pages", flags["anchor"].(string))
				default:
					path = append(path, "pages")
				}
				if group != "published-page" && action != "list" && action != "create" {
					path = append(path, flags["id"].(string))
				}
				method := http.MethodGet
				switch action {
				case "create":
					method = http.MethodPost
				case "update":
					method = http.MethodPut
				case "delete":
					method = http.MethodDelete
				case "archive":
					method = http.MethodPost
					path = append(path, "archive")
				case "restore":
					if version == "v1" {
						method = http.MethodDelete
						path = append(path, "archive")
					} else {
						method = http.MethodPost
						path = append(path, "unarchive")
					}
				}
				query := url.Values{}
				if values, ok := input["query"].(map[string]any); ok {
					for key, value := range values {
						query.Set(key, fmt.Sprint(value))
					}
				}
				body, err := resourceBody(cmd, input)
				if err != nil {
					return err
				}
				return apiRequest(cmd, method, path, query, body, action == "delete" || action == "archive" || action == "restore")
			}
		}
	}
	return handlers
}
