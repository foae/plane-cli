package cli

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

func epicHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, action := range []string{"list", "get", "create", "update", "delete", "add-work-items", "list-work-items"} {
		handlers["epic-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			workspace, _ := flags["workspace"].(string)
			project, _ := flags["project"].(string)
			segments := []string{"api", "v1", "workspaces", workspace, "projects", project, "epics"}
			if action != "list" && action != "create" {
				id, _ := flags["id"].(string)
				segments = append(segments, id)
			}
			method := http.MethodGet
			switch action {
			case "create", "add-work-items":
				method = http.MethodPost
			case "update":
				method = http.MethodPatch
			case "delete":
				method = http.MethodDelete
			}
			if action == "add-work-items" || action == "list-work-items" {
				segments = append(segments, "issues")
			}
			query := url.Values{}
			if values, ok := input["query"].(map[string]any); ok {
				for key, value := range values {
					switch v := value.(type) {
					case string:
						query.Set(key, v)
					case json.Number:
						query.Set(key, v.String())
					}
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, method, segments, query, body, action == "delete")
		}
	}
	return handlers
}
