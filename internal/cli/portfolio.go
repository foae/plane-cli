package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func portfolioHandlers() map[string]handler {
	handlers := make(map[string]handler)
	families := map[string][]string{
		"initiative":       {"list", "get", "create", "update", "delete", "list-labels", "add-labels", "remove-labels", "manage-labels", "list-projects", "add-projects", "remove-projects", "manage-projects", "list-epics", "add-epics", "remove-epics", "list-work-items", "manage-work-items"},
		"initiative-label": {"list", "get", "create", "update", "delete"},
		"teamspace":        {"list", "get", "create", "update", "delete", "list-projects", "add-projects", "remove-projects", "list-members", "add-members", "remove-members"},
	}
	for family, actions := range families {
		for _, action := range actions {
			handlers[family+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version, _ := flags["api-version"].(string)
				workspace, _ := flags["workspace"].(string)
				id, _ := flags["id"].(string)
				resource := family + "s"
				if family == "initiative-label" {
					resource = "initiatives"
				}
				segments := []string{"api", version, "workspaces", workspace, resource}
				if family == "initiative-label" {
					segments = append(segments, "labels")
				}
				method := http.MethodGet
				if action != "list" && action != "create" {
					segments = append(segments, id)
				}
				switch action {
				case "create":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete":
					method = http.MethodDelete
				}
				if prefix, child, ok := strings.Cut(action, "-"); ok {
					segments = append(segments, child)
					switch prefix {
					case "add", "manage":
						method = http.MethodPost
					case "remove":
						method = http.MethodDelete
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
				destructive := method == http.MethodDelete
				if version == "v2" && action == "update" {
					if values, ok := input["body"].(map[string]any); ok {
						_, projects := values["project_ids"]
						_, members := values["member_ids"]
						destructive = (family == "initiative" && projects) ||
							(family == "teamspace" && (projects || members))
					}
				}
				if strings.HasPrefix(action, "manage-") {
					if values, ok := input["body"].(map[string]any); ok {
						removed, _ := values["remove"].([]any)
						destructive = len(removed) > 0
					}
				}
				return apiRequest(cmd, method, segments, query, body, destructive)
			}
		}
	}
	return handlers
}
