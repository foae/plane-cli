package cli

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"

	"github.com/spf13/cobra"
)

// adminMembershipHandlers preserves native membership IDs, role fields and
// response shapes. Version availability is explicit; no request falls back.
func adminMembershipHandlers() map[string]handler {
	handlers := make(map[string]handler)
	families := map[string][]string{
		"member-workspace":             {"list", "remove"},
		"member-project":               {"list", "get", "create", "update", "delete", "role-distribution"},
		"invitation":                   {"list", "get", "create", "update", "delete", "bulk-create"},
		"group-sync-config":            {"get", "update"},
		"group-sync-workspace-mapping": {"list", "get", "create", "update", "delete"},
		"group-sync-project-mapping":   {"list", "get", "create", "update", "delete", "update-by-key"},
	}
	for family, actions := range families {
		for _, action := range actions {
			handlers[family+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version := flags["api-version"].(string)
				if (family == "member-workspace" && action == "remove" && version != "v2") ||
					(family == "invitation" && ((action == "update" && version != "v1") || (action == "bulk-create" && version != "v2"))) ||
					(action == "role-distribution" && version != "v2") ||
					(action == "update-by-key" && version != "v1") {
					return unavailable(commandName(cmd))
				}
				segments := []string{"api", version, "workspaces", flags["workspace"].(string)}
				switch family {
				case "member-workspace":
					segments = append(segments, "members")
					if action == "remove" {
						segments = append(segments, "remove")
					}
				case "member-project":
					resource := "members"
					if version == "v1" {
						resource = "project-members"
					}
					if action == "role-distribution" {
						segments = append(segments, "project-role-distribution")
					} else {
						segments = append(segments, "projects", flags["project"].(string), resource)
					}
				case "invitation":
					segments = append(segments, "invitations")
					if action == "bulk-create" {
						segments = append(segments, "bulk")
					}
				case "group-sync-config":
					segments = append(segments, "group-sync", "config")
				case "group-sync-workspace-mapping":
					segments = append(segments, "group-sync", "workspace-mappings")
				case "group-sync-project-mapping":
					segments = append(segments, "group-sync", "project-mappings")
				}
				if action == "update-by-key" {
					segments = append(segments, flags["project-key"].(string), flags["idp-group-name"].(string))
				} else if family != "group-sync-config" && (action == "get" || action == "update" || action == "delete") {
					segments = append(segments, flags["id"].(string))
				}
				method := http.MethodGet
				switch action {
				case "create", "bulk-create", "remove":
					method = http.MethodPost
				case "update", "update-by-key":
					method = http.MethodPatch
				case "delete":
					method = http.MethodDelete
				}
				query := url.Values{}
				if values, ok := input["query"].(map[string]any); ok {
					for key, value := range values {
						query.Set(key, fmt.Sprint(value))
					}
				}
				if (family == "invitation" || (family == "member-workspace" && action == "remove")) && version == "v2" {
					// Format is an annotation in the shared schema compiler.
					// Validate email syntax before dry-run or remote mutation.
					if body, ok := input["body"].(map[string]any); ok {
						if email, ok := body["email"].(string); ok {
							if err := validateInvitationEmail(email); err != nil {
								return err
							}
						}
						if emails, ok := body["emails"].([]any); ok {
							for _, value := range emails {
								if err := validateInvitationEmail(value.(string)); err != nil {
									return err
								}
							}
						}
					}
				}
				body, err := resourceBody(cmd, input)
				if err != nil {
					return err
				}
				mutation := method != http.MethodGet
				if family == "group-sync-config" && action == "get" {
					// v1 GET creates missing configuration; v2 GET is an ordinary read.
					mutation = version == "v1"
					cmd.Annotations["side_effect"] = "read"
					if mutation {
						cmd.Annotations["side_effect"] = "write"
					}
				}
				return apiRequest(cmd, method, segments, query, body, mutation)
			}
		}
	}
	return handlers
}

func validateInvitationEmail(value string) error {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return errors.New("invalid invitation email address")
	}
	return nil
}
