package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

func triageHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, group := range []string{"intake", "recurring-work-item", "work-item-relation"} {
		actions := []string{"list", "get", "create", "update", "delete"}
		if group == "work-item-relation" {
			actions = []string{"list", "create", "remove"}
		}
		for _, action := range actions {
			handlers[group+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				path := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string)}
				switch group {
				case "intake":
					path = append(path, "intake-issues")
				case "recurring-work-item":
					path = append(path, "recurring-work-items")
				case "work-item-relation":
					path = append(path, "work-items", flags["work-item"].(string), "relations")
				}
				method := http.MethodGet
				switch action {
				case "get", "update", "delete":
					path = append(path, flags["id"].(string))
				}
				switch action {
				case "create":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete":
					method = http.MethodDelete
				case "remove":
					method = http.MethodPost
					path = append(path, "remove")
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
				return apiRequest(cmd, method, path, query, body, action == "delete" || action == "remove")
			}
		}
	}
	return handlers
}
