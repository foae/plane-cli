package cli

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

// adminPermissionsHandlers sends only the explicitly selected native operation.
// workspace-feature-get remains registered by typeGovernanceHandlers.
func adminPermissionsHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, operation := range []struct {
		name, method string
	}{
		{"role-list", "GET"},
		{"role-get", "GET"},
		{"permission-scheme-list", "GET"},
		{"permission-scheme-get", "GET"},
		{"permission-workspace-effective", "GET"},
		{"permission-project-effective", "GET"},
		{"permission-check", "GET"},
		{"permission-resource", "GET"},
		{"permission-accessible", "GET"},
		{"permission-relation", "GET"},
		{"permission-grant-list", "GET"},
		{"permission-grant-get", "GET"},
		{"permission-grant-create", "POST"},
		{"permission-grant-update", "PATCH"},
		{"permission-grant-delete", "DELETE"},
		{"project-feature-get", "GET"},
		{"project-feature-update", "PATCH"},
		{"workspace-feature-update", "PATCH"},
	} {
		handlers[operation.name] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string)}
			switch operation.name {
			case "role-list", "role-get":
				segments = append(segments, "roles")
			case "permission-scheme-list", "permission-scheme-get":
				segments = append(segments, "permission-schemes")
			case "permission-workspace-effective":
				segments = append(segments, "permissions", "me")
			case "permission-project-effective":
				segments = append(segments, "projects", flags["project"].(string), "permissions", "me")
			case "permission-check":
				segments = append(segments, "permissions", "check")
			case "permission-resource":
				segments = append(segments, "permissions", "resource")
			case "permission-accessible":
				segments = append(segments, "permissions", "accessible")
			case "permission-relation":
				segments = append(segments, "permissions", "relation")
			case "project-feature-get", "project-feature-update":
				segments = append(segments, "projects", flags["project"].(string), "features")
			case "workspace-feature-update":
				segments = append(segments, "features")
			default:
				segments = append(segments, "permissions", "grants")
			}
			if id, ok := flags["id"].(string); ok {
				segments = append(segments, id)
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
			return apiRequest(cmd, operation.method, segments, query, body, operation.method != "GET")
		}
	}
	return handlers
}
