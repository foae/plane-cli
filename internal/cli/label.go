package cli

import "github.com/spf13/cobra"

func labelHandlers() map[string]handler {
	return map[string]handler{
		"label-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, false)
			if err != nil {
				return err
			}
			path := t.labels()
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"label-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, true, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, t.id)
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"label-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, false)
			if err != nil {
				return err
			}
			path := t.labels()
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"label-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, true, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, t.id)
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", path, stateLabelQuery(input), body, false)
		},
		"label-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, true, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, t.id)
			return apiRequest(cmd, "DELETE", path, stateLabelQuery(input), nil, true)
		},
		"label-upsert": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, "upsert")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"label-bulk-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, "bulk-create")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"label-bulk-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, "bulk-update")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"label-bulk-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, true, false, false)
			if err != nil {
				return err
			}
			path := t.labels()
			path = append(path, "bulk-delete")
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, true)
		},
		"project-label-list": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, false, false)
			if err != nil {
				return err
			}
			path := t.projectLabels()
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"project-label-get": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectLabels()
			path = append(path, t.id)
			return apiRequest(cmd, "GET", path, stateLabelQuery(input), nil, false)
		},
		"project-label-create": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, false, false)
			if err != nil {
				return err
			}
			path := t.projectLabels()
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "POST", path, stateLabelQuery(input), body, false)
		},
		"project-label-update": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectLabels()
			path = append(path, t.id)
			body, err := stateLabelBody(input)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "PATCH", path, stateLabelQuery(input), body, false)
		},
		"project-label-delete": func(cmd *cobra.Command, _ []string, input map[string]any) error {
			t, err := stateLabelInput(cmd, input, false, true, false)
			if err != nil {
				return err
			}
			path := t.projectLabels()
			path = append(path, t.id)
			return apiRequest(cmd, "DELETE", path, stateLabelQuery(input), nil, true)
		},
	}
}
