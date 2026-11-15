package cli

import (
	"fmt"
	"net/url"

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// contentCollectionHandlers preserves generation-specific page placement IDs.
func contentCollectionHandlers() map[string]handler {
	return map[string]handler{
		"collection-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v2":
				segments = []string{"api", "v2", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string)}
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "GET", segments, query, body, false)
		},
		"collection-page-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v2":
				segments = []string{"api", "v2", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages"}
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "GET", segments, query, body, false)
		},
		"collection-page-move": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v2":
				segments = []string{"api", "v2", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages", flags["page"].(string)}
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages", flags["page-collection"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", segments, query, body, true)
		},
		"collection-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "GET", segments, query, body, false)
		},
		"collection-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query, body, false)
		},
		"collection-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", segments, query, body, false)
		},
		"collection-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "DELETE", segments, query, body, true)
		},
		"collection-member-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "members"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "GET", segments, query, body, false)
		},
		"collection-member-add": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "members"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query, body, true)
		},
		"collection-member-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "members", flags["member"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", segments, query, body, true)
		},
		"collection-member-remove": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "members", flags["member"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "DELETE", segments, query, body, true)
		},
		"collection-page-search": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages-search"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "GET", segments, query, body, false)
		},
		"collection-page-add": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages"}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			if body, ok := input["body"].(map[string]any); ok {
				pages, _ := body["page_ids"].([]any)
				if placement, ok := body["placement"].(map[string]any); ok && (placement["type"] == "before" || placement["type"] == "after") && len(pages) != 1 {
					return &output.Error{Code: "usage_error", Message: "before/after placement requires exactly one page", Exit: 2}
				}
				if orders, ok := body["sort_orders"].(map[string]any); ok {
					for id := range orders {
						found := false
						for _, page := range pages {
							if page == id {
								found = true
								break
							}
						}
						if !found {
							return &output.Error{Code: "usage_error", Message: "sort_orders keys must occur in page_ids", Exit: 2}
						}
					}
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", segments, query, body, true)
		},
		"collection-page-remove": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			var segments []string
			switch version {
			case "v1":
				segments = []string{"api", "v1", "workspaces", flags["workspace"].(string), "collections", flags["collection"].(string), "pages", flags["page-collection"].(string)}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported collection operation version", Exit: 2}
			}
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "DELETE", segments, query, body, true)
		},
	}
}
