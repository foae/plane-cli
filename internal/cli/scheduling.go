package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
	_ "time/tzdata" // Keep IANA validation usable without host zoneinfo.

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// Scheduling keeps legacy membership/archive routes separate from v2 actions.
func schedulingHandlers() map[string]handler {
	handlers := make(map[string]handler)
	actions := map[string][]string{
		"cycle":          {"list", "get", "create", "update", "delete", "upsert", "bulk-create", "bulk-update", "bulk-delete", "archive", "unarchive", "list-archived", "add-work-items", "remove-work-item", "list-work-items", "manage-work-items", "transfer-work-items"},
		"module":         {"list", "get", "create", "update", "delete", "upsert", "bulk-create", "bulk-update", "bulk-delete", "archive", "unarchive", "list-archived", "add-work-items", "remove-work-item", "list-work-items", "manage-work-items"},
		"milestone":      {"list", "get", "create", "update", "delete", "upsert", "bulk-create", "bulk-update", "bulk-delete", "list-work-items", "manage-work-items"},
		"estimate":       {"list", "get", "create", "update", "delete", "upsert", "bulk-create", "bulk-update", "bulk-delete"},
		"estimate-point": {"list", "get", "create", "update", "delete", "upsert", "bulk-create", "bulk-update", "bulk-delete"},
	}
	for family, verbs := range actions {
		for _, action := range verbs {
			handlers[family+"-"+action] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version, _ := flags["api-version"].(string)
				if err := validateSchedulingDates(cmd, input, family, action, version); err != nil {
					return err
				}
				workspace, _ := flags["workspace"].(string)
				project, _ := flags["project"].(string)
				id, _ := flags["id"].(string)
				segments := []string{"api", version, "workspaces", workspace, "projects", project}
				resource := family + "s"
				if family == "estimate-point" {
					estimate, _ := flags["estimate"].(string)
					segments = append(segments, "estimates", estimate)
					resource = "points"
					if version == "v1" {
						resource = "estimate-points"
					}
				}
				if version == "v1" && (action == "list-archived" || action == "unarchive") {
					resource = "archived-" + resource
				}
				segments = append(segments, resource)
				method := http.MethodGet
				destructive := false
				switch action {
				case "get", "update", "delete", "archive", "unarchive", "list-work-items", "add-work-items", "manage-work-items", "transfer-work-items":
					// v1 estimates address the project's active system, not a target id.
					if version != "v1" || family != "estimate" {
						segments = append(segments, id)
					}
				case "remove-work-item":
					parent, _ := flags[family].(string)
					segments = append(segments, parent)
				}
				switch action {
				case "create":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete":
					method, destructive = http.MethodDelete, true
				case "upsert", "bulk-create", "bulk-update", "bulk-delete":
					method = http.MethodPost
					segments = append(segments, action)
					destructive = action == "bulk-delete"
				case "archive":
					method, destructive = http.MethodPost, true
					segments = append(segments, "archive")
				case "unarchive":
					method = http.MethodPost
					if version == "v1" {
						method = http.MethodDelete
					}
					segments = append(segments, "unarchive")
				case "transfer-work-items":
					method, destructive = http.MethodPost, true
					suffix := "transfer"
					if version == "v1" {
						suffix = "transfer-issues"
					}
					segments = append(segments, suffix)
				case "list-work-items", "add-work-items", "remove-work-item", "manage-work-items":
					suffix := "work-items"
					if version == "v1" && family != "milestone" {
						suffix = family + "-issues"
					}
					segments = append(segments, suffix)
					if action == "remove-work-item" {
						method, destructive = http.MethodDelete, true
						segments = append(segments, id)
					} else if action != "list-work-items" {
						method = http.MethodPost
						body, _ := input["body"].(map[string]any)
						removed, _ := body["remove"].([]any)
						destructive = len(removed) > 0
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
				return apiRequest(cmd, method, segments, query, body, destructive)
			}
		}
	}
	return handlers
}

// Validate only formats resolved by the operation prose; never convert zones.
func validateSchedulingDates(cmd *cobra.Command, input map[string]any, family, action, version string) error {
	body, _ := input["body"].(map[string]any)
	rows := []map[string]any{body}
	if items, ok := body["items"].([]any); ok {
		rows = nil
		for _, item := range items {
			row, _ := item.(map[string]any)
			rows = append(rows, row)
		}
	}
	for _, row := range rows {
		if zone, present := row["timezone"].(string); present && version == "v2" && family == "cycle" {
			if _, err := time.LoadLocation(zone); err != nil || zone == "" || zone == "Local" {
				return &output.Error{Code: "usage_error", Message: "Invalid IANA timezone", Command: commandName(cmd), Exit: 2}
			}
		}
		for _, field := range []string{"start_date", "end_date", "target_date"} {
			value, present := row[field].(string)
			if !present {
				continue
			}
			layout := ""
			if version == "v2" && family == "cycle" && action != "upsert" {
				layout = time.RFC3339Nano
			} else if (version == "v2" && family == "module") || family == "milestone" {
				layout = "2006-01-02"
			}
			if layout != "" {
				if _, err := time.Parse(layout, value); err != nil {
					return &output.Error{Code: "usage_error", Message: "Invalid " + field + " format", Command: commandName(cmd), Exit: 2}
				}
			}
		}
		if version == "v2" && family == "module" {
			start, _ := row["start_date"].(string)
			target, _ := row["target_date"].(string)
			if start != "" && target != "" && target < start {
				return &output.Error{Code: "usage_error", Message: "target_date must not be earlier than start_date", Command: commandName(cmd), Exit: 2}
			}
		}
	}
	return nil
}
