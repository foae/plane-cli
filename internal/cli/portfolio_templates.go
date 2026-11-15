package cli

import (
	"fmt"
	"net/url"

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// portfolioTemplateHandlers keeps the native template and saved-view contracts
// separate: only explicitly scoped families can select project routes.
func portfolioTemplateHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, resource := range []struct {
		name, route string
		scoped      bool
		actions     []string
	}{
		{"view", "views", true, []string{"list", "get", "create", "update", "delete"}},
		{"work-item-template", "work-item-templates", true, []string{"list", "get", "create", "update", "delete", "use"}},
		{"page-template", "page-templates", true, []string{"list", "get", "create", "update", "delete"}},
		{"project-template", "project-templates", false, []string{"list", "get", "create", "update", "delete", "copy", "use"}},
		{"template-category", "template-categories", false, []string{"list", "get"}},
	} {
		for _, action := range resource.actions {
			handlers[resource.name+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				if resource.scoped && flags["scope"] == "workspace" && cmd.Flags().Changed("project") {
					return &output.Error{Code: "usage_error", Message: "--project conflicts with --scope workspace", Command: commandName(cmd), Exit: 2}
				}
				segments := []string{"api", "v2"}
				if resource.name != "template-category" {
					segments = append(segments, "workspaces", flags["workspace"].(string))
				}
				if resource.scoped && flags["scope"] == "project" {
					segments = append(segments, "projects", flags["project"].(string))
				}
				segments = append(segments, resource.route)
				if action == "get" || action == "update" || action == "delete" || action == "use" {
					segments = append(segments, flags["id"].(string))
				}
				if action == "copy" || action == "use" {
					segments = append(segments, action)
				}
				query := url.Values{}
				if values, ok := input["query"].(map[string]any); ok {
					for key, value := range values {
						query.Set(key, fmt.Sprint(value))
					}
				}
				method := "GET"
				switch action {
				case "create", "copy", "use":
					method = "POST"
				case "update":
					method = "PATCH"
				case "delete":
					method = "DELETE"
				}
				body, err := resourceBody(cmd, input)
				if err != nil {
					return err
				}
				destructive := action == "delete"
				// Replacing a seed can erase omitted content; ordinary metadata
				// updates do not need confirmation and never fetch the old seed.
				if action == "update" {
					if values, ok := input["body"].(map[string]any); ok {
						if resource.name == "view" {
							_, destructive = values["access"]
						} else {
							_, destructive = values["template_data"]
						}
					}
				}
				return apiRequest(cmd, method, segments, query, body, destructive)
			}
		}
	}
	return handlers
}
