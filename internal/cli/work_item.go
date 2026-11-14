package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// workItemHandlers keeps the two API generations' routes explicit. Bodies are
// validated against the selected operation descriptor before transport runs.
func workItemHandlers() map[string]handler {
	projectPath := func(input map[string]any) []string {
		flags := input["flags"].(map[string]any)
		return []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "work-items"}
	}
	workspacePath := func(input map[string]any) []string {
		flags := input["flags"].(map[string]any)
		return []string{"api", flags["api-version"].(string), "workspaces", flags["workspace"].(string), "work-items"}
	}
	query := func(input map[string]any) url.Values {
		values := url.Values{}
		if fields, ok := input["query"].(map[string]any); ok {
			for key, value := range fields {
				values.Set(key, fmt.Sprint(value))
			}
		}
		return values
	}
	id := func(input map[string]any) string { return input["flags"].(map[string]any)["id"].(string) }
	legacyArchiveOrDelete := func(input map[string]any) bool {
		if input["flags"].(map[string]any)["api-version"] != "v1" {
			return false
		}
		body, _ := input["body"].(map[string]any)
		return body["archived_at"] != nil || body["deleted_at"] != nil
	}
	return map[string]handler{
		"work-item-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			if input["flags"].(map[string]any)["scope"] == "workspace" {
				return apiRequest(cmd, http.MethodGet, workspacePath(input), query(input), nil, false)
			}
			return apiRequest(cmd, http.MethodGet, projectPath(input), query(input), nil, false)
		},
		"work-item-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return apiRequest(cmd, http.MethodGet, append(projectPath(input), id(input)), query(input), nil, false)
		},
		"work-item-get-by-identifier": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			identifier := input["flags"].(map[string]any)["identifier"].(string)
			return apiRequest(cmd, http.MethodGet, append(workspacePath(input), identifier), query(input), nil, false)
		},
		"work-item-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPost, projectPath(input), query(input), body, legacyArchiveOrDelete(input))
		},
		"work-item-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPatch, append(projectPath(input), id(input)), query(input), body, legacyArchiveOrDelete(input))
		},
		"work-item-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return apiRequest(cmd, http.MethodDelete, append(projectPath(input), id(input)), query(input), nil, true)
		},
		"work-item-upsert": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPost, append(projectPath(input), "upsert"), query(input), body, false)
		},
		"work-item-bulk-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPost, append(projectPath(input), "bulk-create"), nil, body, false)
		},
		"work-item-bulk-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPost, append(projectPath(input), "bulk-update"), nil, body, false)
		},
		"work-item-bulk-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPost, append(projectPath(input), "bulk-delete"), nil, body, true)
		},
		"work-item-archive": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return apiRequest(cmd, http.MethodPost, append(projectPath(input), id(input), "archive"), query(input), nil, true)
		},
		"work-item-unarchive": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return apiRequest(cmd, http.MethodPost, append(projectPath(input), id(input), "unarchive"), query(input), nil, false)
		},
		"work-item-search": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return apiRequest(cmd, http.MethodGet, append(workspacePath(input), "search"), query(input), nil, false)
		},
		"work-item-advanced-search": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, http.MethodPost, append(workspacePath(input), "advanced-search"), nil, body, false)
		},
	}
}
