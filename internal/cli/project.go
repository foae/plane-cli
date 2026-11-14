package cli

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func projectHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, action := range []string{"list", "get", "summary", "create", "create-with-template", "update", "delete", "archive", "unarchive", "upsert", "bulk-create", "bulk-update"} {
		handlers["project-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			settings, err := effectiveSettings(cmd)
			if err != nil {
				return err
			}
			flags := input["flags"].(map[string]any)
			workspace, _ := flags["workspace"].(string)
			segments := []string{"api", settings.APIVersion, "workspaces", workspace, "projects"}
			method := http.MethodGet
			destructive := false
			switch action {
			case "get", "summary", "update", "delete", "archive", "unarchive":
				id, _ := flags["id"].(string)
				segments = append(segments, id)
			}
			switch action {
			case "summary":
				segments = append(segments, "summary")
			case "create":
				method = http.MethodPost
			case "create-with-template":
				method = http.MethodPost
				segments = append(segments, "templates", "use")
			case "update":
				method = http.MethodPatch
			case "delete":
				method = http.MethodDelete
				destructive = true
			case "archive":
				method = http.MethodPost
				destructive = true
				segments = append(segments, "archive")
			case "unarchive":
				method = http.MethodPost
				suffix := "unarchive"
				if settings.APIVersion == "v1" {
					method = http.MethodDelete
					suffix = "archive"
				}
				segments = append(segments, suffix)
			case "upsert", "bulk-create", "bulk-update":
				method = http.MethodPost
				segments = append(segments, action)
			}
			query := url.Values{}
			if values, ok := input["query"].(map[string]any); ok {
				for key, value := range values {
					switch typed := value.(type) {
					case string:
						query.Set(key, typed)
					default:
						encoded, err := resourceBody(cmd, map[string]any{"body": value})
						if err != nil {
							return err
						}
						query.Set(key, strings.TrimSpace(string(encoded)))
					}
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, method, segments, query, body, destructive)
		}
	}
	return handlers
}
