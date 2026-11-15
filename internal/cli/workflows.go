package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// workflowHandlers keeps project and catalog routes distinct. In particular,
// catalog chain states use catalog state ids, not project chain entry ids.
func workflowHandlers() map[string]handler {
	operations := []struct {
		key          string
		method       string
		route        string
		mutation     bool
		rotation     bool
		secretCreate bool
	}{
		{"workflow-project-list", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/", false, false, false},
		{"workflow-project-get", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{pk}/", false, false, false},
		{"workflow-project-create", "POST", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/", true, false, false},
		{"workflow-project-update", "PATCH", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{pk}/", true, false, false},
		{"workflow-project-delete", "DELETE", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{pk}/", true, false, false},
		{"workflow-project-preview-fallback", "POST", "/api/v2/workspaces/{slug}/projects/{project_id}/workflow-fallback-preview/", false, false, false},
		{"workflow-project-state-list", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/states/", false, false, false},
		{"workflow-project-state-get", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/states/{pk}/", false, false, false},
		{"workflow-project-state-create", "POST", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/states/", true, false, false},
		{"workflow-project-state-update", "PATCH", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/states/{pk}/", true, false, false},
		{"workflow-project-state-delete", "DELETE", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/states/{pk}/", true, false, false},
		{"workflow-project-transition-list", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/", false, false, false},
		{"workflow-project-transition-get", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{pk}/", false, false, false},
		{"workflow-project-transition-create", "POST", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/", true, false, false},
		{"workflow-project-transition-update", "PATCH", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{pk}/", true, false, false},
		{"workflow-project-transition-delete", "DELETE", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{pk}/", true, false, false},
		{"workflow-project-activity-list", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/activities/", false, false, false},
		{"workflow-project-activity-get", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/activities/{pk}/", false, false, false},
		{"workflow-project-hook-list", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/", false, false, false},
		{"workflow-project-hook-get", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/", false, false, false},
		{"workflow-project-hook-create", "POST", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/", true, false, true},
		{"workflow-project-hook-update", "PATCH", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/", true, false, false},
		{"workflow-project-hook-delete", "DELETE", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/", true, false, false},
		{"workflow-project-hook-list-executions", "GET", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/executions/", false, false, false},
		{"workflow-project-hook-regenerate-secret", "POST", "/api/v2/workspaces/{slug}/projects/{project_id}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/regenerate-webhook-secret/", true, true, true},
		{"workflow-workspace-list", "GET", "/api/v2/workspaces/{slug}/workflows/", false, false, false},
		{"workflow-workspace-get", "GET", "/api/v2/workspaces/{slug}/workflows/{pk}/", false, false, false},
		{"workflow-workspace-create", "POST", "/api/v2/workspaces/{slug}/workflows/", true, false, false},
		{"workflow-workspace-update", "PATCH", "/api/v2/workspaces/{slug}/workflows/{pk}/", true, false, false},
		{"workflow-workspace-delete", "DELETE", "/api/v2/workspaces/{slug}/workflows/{pk}/", true, false, false},
		{"workflow-workspace-get-usage", "GET", "/api/v2/workspaces/{slug}/workflows/{pk}/usage/", false, false, false},
		{"workflow-workspace-state-list", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/states/", false, false, false},
		{"workflow-workspace-state-get", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/states/{state_id}/", false, false, false},
		{"workflow-workspace-state-add", "POST", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/states/", true, false, false},
		{"workflow-workspace-state-update", "PATCH", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/states/{state_id}/", true, false, false},
		{"workflow-workspace-state-remove", "DELETE", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/states/{state_id}/", true, false, false},
		{"workflow-workspace-state-mark-default", "POST", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/states/{state_id}/mark-default/", true, false, false},
		{"workflow-workspace-transition-list", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/", false, false, false},
		{"workflow-workspace-transition-get", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{pk}/", false, false, false},
		{"workflow-workspace-transition-create", "POST", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/", true, false, false},
		{"workflow-workspace-transition-update", "PATCH", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{pk}/", true, false, false},
		{"workflow-workspace-transition-delete", "DELETE", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{pk}/", true, false, false},
		{"workflow-workspace-activity-list", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/activities/", false, false, false},
		{"workflow-workspace-activity-get", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/activities/{pk}/", false, false, false},
		{"workflow-workspace-hook-list", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/", false, false, false},
		{"workflow-workspace-hook-get", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/", false, false, false},
		{"workflow-workspace-hook-create", "POST", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/", true, false, true},
		{"workflow-workspace-hook-update", "PATCH", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/", true, false, false},
		{"workflow-workspace-hook-delete", "DELETE", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/", true, false, false},
		{"workflow-workspace-hook-list-executions", "GET", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/executions/", false, false, false},
		{"workflow-workspace-hook-regenerate-secret", "POST", "/api/v2/workspaces/{slug}/workflows/{workflow_id}/state-transitions/{transition_id}/hooks/{pk}/regenerate-webhook-secret/", true, true, true},
	}
	handlers := make(map[string]handler, len(operations))
	for _, operation := range operations {
		recovery := ""
		if operation.rotation || operation.secretCreate {
			scope := "project"
			if strings.HasPrefix(operation.key, "workflow-workspace-") {
				scope = "workspace"
			}
			recovery = "Reads do not return the signing secret. Explicitly rotate it with workflow " + scope + " hook regenerate-secret --yes using the affected workflow, transition and hook ids. No retry or rollback was attempted."
		}
		handlers[operation.key] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := strings.Split(strings.Trim(operation.route, "/"), "/")
			for i, segment := range segments {
				switch segment {
				case "{slug}":
					segments[i], _ = flags["workspace"].(string)
				case "{project_id}":
					segments[i], _ = flags["project"].(string)
				case "{workflow_id}":
					segments[i], _ = flags["workflow"].(string)
				case "{transition_id}":
					segments[i], _ = flags["transition"].(string)
				case "{pk}", "{state_id}":
					segments[i], _ = flags["id"].(string)
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
			if operation.rotation {
				return apiIntentionalSecretRequest(cmd, operation.method, segments, query, body, []string{"/secret"}, recovery)
			}
			if operation.secretCreate {
				// Offline validation requires an object and the documented handler enum.
				hook := input["body"].(map[string]any)
				if hook["handler_name"] == "send_webhook" {
					if query.Has("fields") {
						return &output.Error{Code: "usage_error", Message: "--fields is not supported when creating a send_webhook hook because it can omit the one-time secret", Command: commandName(cmd), Exit: 2}
					}
					return apiIntentionalSecretRequest(cmd, operation.method, segments, query, body, []string{"/config/secret_plaintext"}, recovery)
				}
				if cmd.Flags().Changed("secret-output-file") || cmd.Flags().Changed("overwrite") {
					return &output.Error{Code: "usage_error", Message: "Secret delivery flags require a send_webhook hook", Command: commandName(cmd), Exit: 2}
				}
			}
			// preview-fallback is a documented read-only POST, not a mutation.
			return apiRequest(cmd, operation.method, segments, query, body, operation.mutation)
		}
	}
	return handlers
}
