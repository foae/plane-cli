package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// workflowGovernanceHandlers supplements, but does not replace, the P05 governance get.
func workflowGovernanceHandlers() map[string]handler {
	operations := []struct {
		key, method, route string
		mutation           bool
	}{
		{"work-item-type-workflow-list", "GET", "projects/{project}/work-item-types/workflows", false},
		{"work-item-type-workflow-get", "GET", "projects/{project}/work-item-types/{type}/workflow", false},
		{"work-item-type-workflow-update", "PATCH", "projects/{project}/work-item-types/{type}/workflow", true},
		{"work-item-type-governance-update", "PATCH", "work-item-types/{type}/governance", true},
		{"work-item-type-governance-preview", "POST", "work-item-types/{type}/governance/preview", false},
		{"work-item-type-governance-pin-list", "GET", "work-item-types/{type}/governance/pins", false},
		{"work-item-type-governance-pin-get", "GET", "work-item-types/{type}/governance/pins/{id}", false},
		{"work-item-type-governance-pin-create", "POST", "work-item-types/{type}/governance/pins", true},
		{"work-item-type-governance-pin-delete", "DELETE", "work-item-types/{type}/governance/pins/{id}", true},
	}
	handlers := make(map[string]handler, len(operations))
	for _, operation := range operations {
		handlers[operation.key] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", "v2", "workspaces", flags["workspace"].(string)}
			for _, segment := range strings.Split(operation.route, "/") {
				if strings.HasPrefix(segment, "{") {
					segment = flags[strings.Trim(segment, "{}")].(string)
				}
				segments = append(segments, segment)
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
			// The network preview is a read even though its method is PATCH.
			preview := operation.key == "work-item-type-workflow-update" && query.Get("dry_run") == "true"
			mutation := operation.mutation && !preview
			if preview {
				sideEffect := cmd.Annotations["side_effect"]
				cmd.Annotations["side_effect"] = "read"
				defer func() { cmd.Annotations["side_effect"] = sideEffect }()
			}
			return apiRequest(cmd, operation.method, segments, query, body, mutation)
		}
	}
	return handlers
}
