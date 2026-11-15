package cli

import (
	"fmt"
	"net/url"

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// portfolioReleaseHandlers keeps catalog, workspace and project routes separate.
func portfolioReleaseHandlers() map[string]handler {
	workspaceReleases := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases"}
	}
	workspaceRelease := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string)}
	}
	workspaceChangelog := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "changelog"}
	}
	workspaceReleaseLabels := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "labels"}
	}
	workspaceReleaseWorkItems := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "work-items"}
	}
	tagCatalog := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", "tags"}
	}
	catalogTag := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", "tags", flags["tag"].(string)}
	}
	labelCatalog := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", "labels"}
	}
	catalogLabel := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", "labels", flags["label"].(string)}
	}
	workspaceComments := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "comments"}
	}
	workspaceComment := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "comments", flags["comment"].(string)}
	}
	workspaceLinks := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "links"}
	}
	workspaceLink := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "releases", flags["release"].(string), "links", flags["link"].(string)}
	}
	projectReleases := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases"}
	}
	projectRelease := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string)}
	}
	projectChangelog := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "changelog"}
	}
	projectReleaseLabels := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "labels"}
	}
	projectReleaseWorkItems := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "work-items"}
	}
	projectComments := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "comments"}
	}
	projectComment := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "comments", flags["comment"].(string)}
	}
	projectLinks := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "links"}
	}
	projectLink := func(version string, flags map[string]any) []string {
		return []string{"api", version, "workspaces", flags["workspace"].(string), "projects", flags["project"].(string), "releases", flags["release"].(string), "links", flags["link"].(string)}
	}

	handlers := make(map[string]handler)
	for _, operation := range []struct {
		name, method string
		v1, v2       bool
		route        func(string, map[string]any) []string
		manage       bool
	}{
		{"release-list", "GET", true, true, workspaceReleases, false},
		{"release-get", "GET", true, true, workspaceRelease, false},
		{"release-changelog-get", "GET", true, true, workspaceChangelog, false},
		{"release-create", "POST", true, true, workspaceReleases, false},
		{"release-update", "PATCH", true, true, workspaceRelease, false},
		{"release-changelog-update", "PATCH", true, true, workspaceChangelog, false},
		{"release-delete", "DELETE", true, true, workspaceRelease, false},
		{"release-label-list", "GET", true, true, workspaceReleaseLabels, false},
		{"release-label-manage", "POST", false, true, workspaceReleaseLabels, true},
		{"release-work-item-list", "GET", true, true, workspaceReleaseWorkItems, false},
		{"release-work-item-manage", "POST", false, true, workspaceReleaseWorkItems, true},
		{"release-tag-list", "GET", true, true, tagCatalog, false},
		{"release-tag-get", "GET", true, true, catalogTag, false},
		{"release-tag-create", "POST", true, true, tagCatalog, false},
		{"release-tag-update", "PATCH", true, true, catalogTag, false},
		{"release-tag-delete", "DELETE", true, true, catalogTag, false},
		{"release-label-catalog-list", "GET", true, true, labelCatalog, false},
		{"release-label-catalog-get", "GET", true, true, catalogLabel, false},
		{"release-label-catalog-create", "POST", true, true, labelCatalog, false},
		{"release-label-catalog-update", "PATCH", true, true, catalogLabel, false},
		{"release-label-catalog-delete", "DELETE", true, true, catalogLabel, false},
		{"release-comment-list", "GET", true, true, workspaceComments, false},
		{"release-comment-get", "GET", true, true, workspaceComment, false},
		{"release-comment-create", "POST", true, true, workspaceComments, false},
		{"release-comment-update", "PATCH", true, true, workspaceComment, false},
		{"release-comment-delete", "DELETE", true, true, workspaceComment, false},
		{"release-link-list", "GET", true, true, workspaceLinks, false},
		{"release-link-get", "GET", true, true, workspaceLink, false},
		{"release-link-create", "POST", true, true, workspaceLinks, false},
		{"release-link-update", "PATCH", true, true, workspaceLink, false},
		{"release-link-delete", "DELETE", true, true, workspaceLink, false},
		{"release-project-list", "GET", true, true, projectReleases, false},
		{"release-project-get", "GET", true, true, projectRelease, false},
		{"release-project-create", "POST", true, true, projectReleases, false},
		{"release-project-update", "PATCH", true, true, projectRelease, false},
		{"release-project-delete", "DELETE", true, true, projectRelease, false},
		{"release-project-changelog-get", "GET", false, true, projectChangelog, false},
		{"release-project-changelog-update", "PATCH", false, true, projectChangelog, false},
		{"release-project-label-list", "GET", false, true, projectReleaseLabels, false},
		{"release-project-label-manage", "POST", false, true, projectReleaseLabels, true},
		{"release-project-work-item-list", "GET", false, true, projectReleaseWorkItems, false},
		{"release-project-work-item-manage", "POST", false, true, projectReleaseWorkItems, true},
		{"release-project-comment-list", "GET", false, true, projectComments, false},
		{"release-project-comment-get", "GET", false, true, projectComment, false},
		{"release-project-comment-create", "POST", false, true, projectComments, false},
		{"release-project-comment-update", "PATCH", false, true, projectComment, false},
		{"release-project-comment-delete", "DELETE", false, true, projectComment, false},
		{"release-project-link-list", "GET", false, true, projectLinks, false},
		{"release-project-link-get", "GET", false, true, projectLink, false},
		{"release-project-link-create", "POST", false, true, projectLinks, false},
		{"release-project-link-update", "PATCH", false, true, projectLink, false},
		{"release-project-link-delete", "DELETE", false, true, projectLink, false},
		{"release-work-item-add", "POST", true, false, workspaceReleaseWorkItems, false},
		{"release-work-item-remove", "DELETE", true, false, workspaceReleaseWorkItems, false},
		{"release-label-add", "POST", true, false, workspaceReleaseLabels, false},
		{"release-label-remove", "DELETE", true, false, workspaceReleaseLabels, false},
	} {
		handlers[operation.name] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			flags := input["flags"].(map[string]any)
			version := flags["api-version"].(string)
			switch version {
			case "v1":
				if !operation.v1 {
					return &output.Error{Code: "usage_error", Message: "Unsupported release operation version", Exit: 2}
				}
			case "v2":
				if !operation.v2 {
					return &output.Error{Code: "usage_error", Message: "Unsupported release operation version", Exit: 2}
				}
			default:
				return &output.Error{Code: "usage_error", Message: "Unsupported release operation version", Exit: 2}
			}
			segments := operation.route(version, flags)
			query := url.Values{}
			if fields, ok := input["query"].(map[string]any); ok {
				for key, value := range fields {
					query.Set(key, fmt.Sprint(value))
				}
			}
			destructive := operation.method == "DELETE"
			if operation.manage {
				if body, ok := input["body"].(map[string]any); ok {
					if remove, ok := body["remove"].([]any); ok {
						destructive = len(remove) != 0
					}
				}
			}
			body, err := resourceBody(cmd, input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, operation.method, segments, query, body, destructive)
		}
	}
	return handlers
}
