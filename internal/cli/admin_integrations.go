package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

func adminIntegrationHandlers() map[string]handler {
	handlers := make(map[string]handler)
	families := map[string][]string{
		"audit-log":    {"list", "get"},
		"notification": {"list", "get"},
		"webhook":      {"list", "get", "create", "update", "delete", "regenerate-secret"},
		"webhook-log":  {"list", "get"},
		"artifact":     {"get", "create", "update", "publish"},
	}
	for family, actions := range families {
		for _, action := range actions {
			handlers[family+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				workspace, _ := flags["workspace"].(string)
				id, _ := flags["id"].(string)
				segments := []string{"api", "v2", "workspaces", workspace, family + "s"}
				if family == "webhook-log" {
					webhook, _ := flags["webhook"].(string)
					segments = append(segments, webhook)
				}
				if action != "list" && action != "create" {
					segments = append(segments, id)
				}
				method := http.MethodGet
				switch action {
				case "create", "publish", "regenerate-secret":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete":
					method = http.MethodDelete
				}
				if family == "artifact" && (action == "update" || action == "publish") {
					segments = append(segments, action)
				}
				if action == "regenerate-secret" {
					segments = append(segments, "regenerate")
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
				if family == "webhook" && (action == "create" || action == "regenerate-secret") {
					return apiIntentionalSecretRequest(cmd, method, segments, query, body, []string{"/secret_key"}, "Inspect webhook list/get for the affected webhook. Reads do not return the secret; if delivery failed, an operator must explicitly rotate it with webhook regenerate-secret --yes. No retry or rollback was attempted.")
				}
				return apiRequest(cmd, method, segments, query, body, method != http.MethodGet)
			}
		}
	}
	return handlers
}
