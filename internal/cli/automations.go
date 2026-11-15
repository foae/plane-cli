package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

func automationHandlers() map[string]handler {
	handlers := make(map[string]handler)
	families := map[string][]string{
		"":         {"list", "get", "create", "update", "delete", "set-status"},
		"node":     {"list", "get", "create", "update", "delete", "regenerate-webhook-secret"},
		"edge":     {"list", "get", "create", "update", "delete"},
		"activity": {"list", "get"},
	}
	for _, scope := range []string{"project", "workspace"} {
		for family, actions := range families {
			for _, action := range actions {
				key := "automation-" + scope + "-"
				if family != "" {
					key += family + "-"
				}
				handlers[key+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
					flags := input["flags"].(map[string]any)
					workspace, _ := flags["workspace"].(string)
					id, _ := flags["id"].(string)
					segments := []string{"api", "v2", "workspaces", workspace}
					if scope == "project" {
						project, _ := flags["project"].(string)
						segments = append(segments, "projects", project)
					}
					segments = append(segments, "automations")
					if family != "" {
						automation, _ := flags["automation"].(string)
						segments = append(segments, automation)
						resource := family + "s"
						if family == "activity" {
							resource = "activities"
						}
						segments = append(segments, resource)
					}
					if action != "list" && action != "create" {
						segments = append(segments, id)
					}
					method := http.MethodGet
					switch action {
					case "create", "set-status", "regenerate-webhook-secret":
						method = http.MethodPost
					case "update":
						method = http.MethodPatch
					case "delete":
						method = http.MethodDelete
					}
					if action == "set-status" {
						segments = append(segments, "status")
					}
					if action == "regenerate-webhook-secret" {
						segments = append(segments, action)
					}
					query := url.Values{}
					if values, ok := input["query"].(map[string]any); ok {
						for name, value := range values {
							query.Set(name, fmt.Sprint(value))
						}
					}
					body, err := resourceBody(cmd, input)
					if err != nil {
						return err
					}
					if action == "regenerate-webhook-secret" {
						recovery := "The old secret cannot be recovered by a read. Inspect the node with automation " + scope + " node get; explicitly run automation " + scope + " node regenerate-webhook-secret --yes only when authorized. No retry or rollback was attempted."
						return apiIntentionalSecretRequest(cmd, method, segments, query, body, []string{"/secret"}, recovery)
					}
					return apiRequest(cmd, method, segments, query, body, method != http.MethodGet)
				}
			}
		}
	}
	return handlers
}
