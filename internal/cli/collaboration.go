package cli

import (
	"fmt"
	"net/url"

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// collaborationHandlers preserves native resource bodies and version-specific routes.
func collaborationHandlers() map[string]handler {
	query := func(input map[string]any) url.Values {
		values := url.Values{}
		if fields, ok := input["query"].(map[string]any); ok {
			for key, value := range fields {
				values.Set(key, fmt.Sprint(value))
			}
		}
		return values
	}
	return map[string]handler{
		"work-item-comment-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-comment-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", flags["id"].(string)}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-comment-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-comment-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", flags["id"].(string)}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", segments, query(input), body, false)
		},
		"work-item-comment-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", flags["id"].(string)}
			return apiRequest(cmd, "DELETE", segments, query(input), nil, true)
		},
		"work-item-comment-upsert": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", "upsert"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-comment-bulk-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", "bulk-create"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-comment-bulk-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", "bulk-update"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-comment-bulk-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "comments", "bulk-delete"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, true)
		},
		"work-item-link-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "links"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-link-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "links", flags["id"].(string)}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-link-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "links"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-link-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "links", flags["id"].(string)}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", segments, query(input), body, false)
		},
		"work-item-link-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "links", flags["id"].(string)}
			return apiRequest(cmd, "DELETE", segments, query(input), nil, true)
		},
		"work-item-activity-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "activities"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-activity-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "activities", flags["id"].(string)}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-worklog-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "worklogs"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-worklog-summary": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			var segments []string
			if flags["api-version"] == "v1" {
				segments = []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "total-worklogs"}
			} else {
				segments = []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "worklogs", "summary"}
			}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-worklog-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "worklogs", flags["id"].(string)}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-worklog-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "worklogs"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-worklog-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "worklogs", flags["id"].(string)}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", segments, query(input), body, false)
		},
		"work-item-worklog-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "worklogs", flags["id"].(string)}
			return apiRequest(cmd, "DELETE", segments, query(input), nil, true)
		},
		"work-item-subscriber-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "subscribers"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-subscriber-list-by-identifier": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "work-items", flags["work-item"].(string), "subscribers"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-subscriber-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "subscribers"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			destructive := false
			if fields, ok := input["body"].(map[string]any); ok {
				add, _ := fields["add"].([]any)
				remove, _ := fields["remove"].([]any)
				destructive = len(remove) != 0
				for _, a := range add {
					for _, r := range remove {
						if a == r {
							return &output.Error{Code: "usage_error", Message: "Subscriber add and remove sets must be disjoint", Command: commandName(cmd), Exit: 2}
						}
					}
				}
			}
			return apiRequest(cmd, "PATCH", segments, query(input), body, destructive)
		},
		"work-item-subscriber-update-by-identifier": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "work-items", flags["work-item"].(string), "subscribers"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			destructive := false
			if fields, ok := input["body"].(map[string]any); ok {
				add, _ := fields["add"].([]any)
				remove, _ := fields["remove"].([]any)
				destructive = len(remove) != 0
				for _, a := range add {
					for _, r := range remove {
						if a == r {
							return &output.Error{Code: "usage_error", Message: "Subscriber add and remove sets must be disjoint", Command: commandName(cmd), Exit: 2}
						}
					}
				}
			}
			return apiRequest(cmd, "PATCH", segments, query(input), body, destructive)
		},
		"work-item-vote-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "votes"}
			return apiRequest(cmd, "GET", segments, query(input), nil, false)
		},
		"work-item-vote-cast": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "votes"}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query(input), body, false)
		},
		"work-item-vote-retract": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			segments := []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "votes"}
			return apiRequest(cmd, "DELETE", segments, query(input), nil, true)
		},
	}
}
