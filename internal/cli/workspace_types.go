package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/foae/plane-cli/internal/output"

	"github.com/spf13/cobra"
)

func workspaceTypesHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, group := range []string{"workspace-work-item-type", "workspace-work-item-property", "workspace-work-item-property-option", "workspace-work-item-type-property", "work-item-property-context"} {
		actions := []string{"list", "get", "create", "update", "delete"}
		if group == "workspace-work-item-type" {
			actions = append(actions, "mark-default")
		}
		if group == "workspace-work-item-type-property" {
			actions = []string{"list", "get", "attach", "detach"}
		}
		for _, action := range actions {
			handlers[group+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				if group == "work-item-property-context" {
					if body, ok := input["body"].(map[string]any); ok {
						options, _ := body["options"].([]any)
						ids, names := make(map[string]bool), make(map[string]bool)
						for _, value := range options {
							option, _ := value.(map[string]any)
							id, hasID := option["id"].(string)
							name, hasName := option["name"].(string)
							name = strings.ToLower(name)
							if hasID && ids[id] || hasName && names[name] {
								return &output.Error{Code: "usage_error", Message: "Context option IDs and case-insensitive names must be unique", Command: commandName(cmd), Exit: 2}
							}
							if hasID {
								ids[id] = true
							}
							if hasName {
								names[name] = true
							}
						}
					}
				}
				path := []string{"api", "v2", "workspaces", flags["workspace"].(string)}
				switch group {
				case "workspace-work-item-type":
					path = append(path, "work-item-types")
				case "workspace-work-item-property":
					path = append(path, "work-item-properties")
				case "workspace-work-item-property-option":
					path = append(path, "work-item-properties", flags["property"].(string), "options")
				case "workspace-work-item-type-property":
					path = append(path, "work-item-types", flags["type"].(string), "properties")
				case "work-item-property-context":
					path = append(path, "work-item-properties", flags["property"].(string), "contexts")
				}
				method := http.MethodGet
				switch action {
				case "get", "update", "delete", "detach", "mark-default":
					path = append(path, flags["id"].(string))
				}
				switch action {
				case "create", "attach":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete", "detach":
					method = http.MethodDelete
				case "mark-default":
					method = http.MethodPost
					path = append(path, "mark-default")
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
				return apiRequest(cmd, method, path, query, body, action == "delete" || action == "detach" || group == "work-item-property-context" && action == "update")
			}
		}
	}
	return handlers
}
