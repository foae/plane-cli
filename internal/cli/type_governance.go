package cli

import (
	"net/http"

	"github.com/spf13/cobra"
)

func typeGovernanceHandlers() map[string]handler {
	return map[string]handler{
		"workspace-feature-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			return apiRequest(cmd, http.MethodGet, []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "features"}, nil, nil, false)
		},
		"work-item-type-governance-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			return apiRequest(cmd, http.MethodGet, []string{"api", "v2", "workspaces", flags["workspace"].(string), "work-item-types", flags["type"].(string), "governance"}, nil, nil, false)
		},
	}
}
