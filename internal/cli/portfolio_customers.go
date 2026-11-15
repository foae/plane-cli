package cli

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// portfolioCustomerHandlers keeps property definitions separate from customer
// values and preserves the distinct association APIs of each generation.
func portfolioCustomerHandlers() map[string]handler {
	handlers := make(map[string]handler)
	for _, resource := range []string{"", "property", "property-value", "request", "work-item"} {
		actions := []string{"list", "get", "create", "update", "delete"}
		switch resource {
		case "":
			actions = append(actions, "upsert")
		case "property-value":
			actions = []string{"list", "get", "update", "set"}
		case "work-item":
			actions = []string{"list", "add", "remove", "manage"}
		}
		for _, action := range actions {
			id := "customer-" + action
			if resource != "" {
				id = "customer-" + resource + "-" + action
			}
			handlers[id] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
				flags := input["flags"].(map[string]any)
				version := flags["api-version"].(string)
				path := []string{"api", version, "workspaces", flags["workspace"].(string)}
				switch resource {
				case "":
					path = append(path, "customers")
					switch action {
					case "upsert":
						path = append(path, "upsert")
					case "get", "update", "delete":
						path = append(path, flags["customer"].(string))
					}
				case "property":
					path = append(path, "customer-properties")
					if action == "get" || action == "update" || action == "delete" {
						path = append(path, flags["property"].(string))
					}
				case "property-value":
					path = append(path, "customers", flags["customer"].(string), "property-values")
					if action == "get" || action == "update" {
						path = append(path, flags["property"].(string))
					}
				case "request":
					path = append(path, "customers", flags["customer"].(string), "requests")
					if action == "get" || action == "update" || action == "delete" {
						path = append(path, flags["request"].(string))
					}
				case "work-item":
					segment := "work-items"
					if version == "v1" {
						segment = "issues"
					}
					path = append(path, "customers", flags["customer"].(string), segment)
					if action == "remove" {
						path = append(path, flags["work-item"].(string))
					}
				}
				method := http.MethodGet
				switch action {
				case "create", "upsert", "add", "manage", "set":
					method = http.MethodPost
				case "update":
					method = http.MethodPatch
				case "delete", "remove":
					method = http.MethodDelete
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
				destructive := action == "delete" || action == "remove"
				if values, ok := input["body"].(map[string]any); ok {
					if resource == "work-item" && action == "manage" {
						remove, _ := values["remove"].([]any)
						destructive = len(remove) != 0
					}
					if resource == "request" && action == "update" {
						// The replacement list can unlink existing associations;
						// absence leaves links unchanged, while [] clears them.
						_, destructive = values["work_item_ids"]
					}
				}
				return apiRequest(cmd, method, path, query, body, destructive)
			}
		}
	}
	return handlers
}
