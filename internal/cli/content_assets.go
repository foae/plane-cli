package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// contentAssetsHandlers implements only documented JSON API phases. Storage
// transfer and documented download redirects have separate transport policies.
func contentAssetsHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for group, actions := range map[string][]string{
		"attachment":                {"list", "get", "create-upload", "confirm-upload", "delete"},
		"workspace-page-attachment": {"list", "get", "create-upload", "confirm-upload", "delete"},
		"workspace-asset":           {"list", "get", "create-upload", "confirm-upload", "delete"},
		"user-asset":                {"list", "get", "create-upload", "confirm-upload", "delete"},
	} {
		for _, action := range actions {
			handlers[group+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version := flags["api-version"].(string)
				path := []string{"api", version}
				if group == "user-asset" {
					if version == "v1" {
						path = append(path, "assets", "user-assets")
					} else {
						path = append(path, "users", "me", "assets")
					}
				} else {
					path = append(path, "workspaces", flags["workspace"].(string))
					switch group {
					case "attachment":
						path = append(path, "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "attachments")
					case "workspace-page-attachment":
						path = append(path, "pages", flags["page"].(string), "attachments")
					case "workspace-asset":
						path = append(path, "assets")
					}
				}
				if action != "list" && action != "create-upload" {
					path = append(path, flags["id"].(string))
				}
				method := http.MethodGet
				switch action {
				case "create-upload":
					method = http.MethodPost
				case "confirm-upload":
					method = http.MethodPatch
				case "delete":
					method = http.MethodDelete
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
				return apiAssetRequest(cmd, method, path, query, body, action == "delete", action == "create-upload")
			}
		}
	}
	return handlers
}
