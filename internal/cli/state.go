package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// State and label routes share only local path/query preparation, never discovery calls.
type stateLabelTarget struct {
	version   string
	workspace string
	project   string
	id        string
	scope     string
}

func stateLabelInput(cmd *cobra.Command, input map[string]any, projectRequired, idRequired, scoped bool) (stateLabelTarget, error) {
	settings, err := effectiveSettings(cmd)
	if err != nil {
		return stateLabelTarget{}, err
	}
	t := stateLabelTarget{version: settings.APIVersion, workspace: settings.Workspace, project: settings.Project}
	flags, _ := input["flags"].(map[string]any)
	if v, ok := flags["workspace"].(string); ok {
		t.workspace = v
	}
	if v, ok := flags["project"].(string); ok {
		t.project = v
	}
	t.id, _ = flags["id"].(string)
	t.scope, _ = flags["scope"].(string)
	missing := ""
	if t.workspace == "" {
		missing = "workspace"
	}
	if scoped {
		if t.scope != "project" && t.scope != "workspace" {
			missing = "scope"
		}
		if t.scope == "workspace" && t.version != "v2" {
			return t, unavailable(commandName(cmd))
		}
		projectRequired = t.scope == "project"
	}
	if projectRequired && t.project == "" {
		missing = "project"
	}
	if idRequired && t.id == "" {
		missing = "id"
	}
	if missing != "" {
		return t, &output.Error{Code: "usage_error", Message: "Missing or invalid --" + missing, Command: commandName(cmd), Exit: 2}
	}
	return t, nil
}

func (t stateLabelTarget) states() []string {
	p := []string{"api", t.version, "workspaces", t.workspace}
	if t.scope == "project" {
		p = append(p, "projects", t.project)
	}
	return append(p, "states")
}
func (t stateLabelTarget) projectStates() []string {
	return []string{"api", t.version, "workspaces", t.workspace, "project-states"}
}
func (t stateLabelTarget) labels() []string {
	return []string{"api", t.version, "workspaces", t.workspace, "projects", t.project, "labels"}
}
func (t stateLabelTarget) projectLabels() []string {
	return []string{"api", t.version, "workspaces", t.workspace, "project-labels"}
}

func stateLabelQuery(input map[string]any) url.Values {
	q := url.Values{}
	flags, _ := input["flags"].(map[string]any)
	for flag, wire := range map[string]string{
		"fields": "fields", "expand": "expand", "cursor": "cursor", "per-page": "per_page",
		"offset": "offset", "paginate": "paginate", "count": "count", "order-by": "order_by",
		"name": "name", "group": "group", "group-in": "group__in", "is-default": "is_default",
		"external-id": "external_id", "external-source": "external_source", "search": "search",
		"parent-id": "parent_id", "parent-is-null": "parent_id__isnull", "name-in": "name__in",
		"filter-project": "project_id",
	} {
		if v, ok := flags[flag]; ok {
			q.Set(wire, fmt.Sprint(v))
		}
	}
	return q
}

func stateLabelBody(input map[string]any) (json.RawMessage, error) {
	return json.Marshal(input["body"])
}

func stateLabelHandlers() map[string]handler {
	h := map[string]handler{
		"state-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, true)
			if err != nil {
				return err
			}
			path := t.states()
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"state-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, true, true)
			if err != nil {
				return err
			}
			path := t.states()
			path = append(path, t.id)
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"state-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, true)
			if err != nil {
				return err
			}
			path := t.states()
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"state-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, true, true)
			if err != nil {
				return err
			}
			path := t.states()
			path = append(path, t.id)
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", path, stateLabelQuery(input), body, false)
		},
		"state-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, true, true)
			if err != nil {
				return err
			}
			path := t.states()
			path = append(path, t.id)
			return apiRequest(cmd, "DELETE", path, stateLabelQuery(input), nil, true)
		},
		"state-upsert": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, true)
			if err != nil {
				return err
			}
			if t.scope != "project" {
				return unavailable(commandName(cmd))
			}
			path := t.states()
			path = append(path, "upsert")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"state-bulk-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, true)
			if err != nil {
				return err
			}
			if t.scope != "project" {
				return unavailable(commandName(cmd))
			}
			path := t.states()
			path = append(path, "bulk-create")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"state-bulk-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, true)
			if err != nil {
				return err
			}
			if t.scope != "project" {
				return unavailable(commandName(cmd))
			}
			path := t.states()
			path = append(path, "bulk-update")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"state-bulk-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, true)
			if err != nil {
				return err
			}
			if t.scope != "project" {
				return unavailable(commandName(cmd))
			}
			path := t.states()
			path = append(path, "bulk-delete")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, true)
		},
		"project-state-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, false, false)
			if err != nil {
				return err
			}
			path := t.projectStates()
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"project-state-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectStates()
			path = append(path, t.id)
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"project-state-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, false, false)
			if err != nil {
				return err
			}
			path := t.projectStates()
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"project-state-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectStates()
			path = append(path, t.id)
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", path, stateLabelQuery(input), body, false)
		},
		"project-state-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectStates()
			path = append(path, t.id)
			return apiRequest(cmd, "DELETE", path, stateLabelQuery(input), nil, true)
		},
		"project-state-mark-default": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectStates()
			path = append(path, t.id)
			path = append(path, "mark-default")
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), nil, false)
		},
	}
	for id, run := range labelHandlers() {
		h[id] = run
	}
	return h
}
