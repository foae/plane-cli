package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/foae/plane-cli/internal/output"
	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

type handler func(*cobra.Command, []string, map[string]any) error

func HandlerIDs() []string {
	ids := []string{"commands", "schema", "version", "update-check", "connection-add", "connection-update", "connection-list", "connection-show", "connection-use", "connection-delete", "auth-status", "auth-login", "auth-complete", "auth-logout", "auth-migrate", "user-me"}
	for id := range projectHandlers() {
		ids = append(ids, id)
	}
	for id := range epicHandlers() {
		ids = append(ids, id)
	}
	for id := range workItemHandlers() {
		ids = append(ids, id)
	}
	for id := range stateLabelHandlers() {
		ids = append(ids, id)
	}
	for id := range triageHandlers() {
		ids = append(ids, id)
	}
	for id := range collaborationHandlers() {
		ids = append(ids, id)
	}
	for id := range schedulingHandlers() {
		ids = append(ids, id)
	}
	for id := range typeGovernanceHandlers() {
		ids = append(ids, id)
	}
	for id := range workspaceTypesHandlers() {
		ids = append(ids, id)
	}
	for id := range projectTypesHandlers() {
		ids = append(ids, id)
	}
	for _, group := range []map[string]handler{contentPagesHandlers(), contentCollectionHandlers(), contentAssetsHandlers(), assetTransferHandlers(), portfolioHandlers(), portfolioCustomerHandlers(), portfolioTemplateHandlers(), portfolioReleaseHandlers(), adminIntegrationHandlers(), adminMembershipHandlers(), adminPermissionsHandlers(), workflowHandlers(), automationHandlers(), workflowGovernanceHandlers()} {
		for id := range group {
			ids = append(ids, id)
		}
	}
	return ids
}

func commandName(cmd *cobra.Command) string {
	if cmd == nil || cmd.Parent() == nil {
		return "plane-cli"
	}
	return strings.TrimPrefix(cmd.CommandPath(), "plane-cli ")
}

func newRoot(catalog spec.Catalog, build Build, out io.Writer, extra map[string]handler) (*cobra.Command, error) {
	root := &cobra.Command{Use: "plane-cli", Short: "Plane CLI — offline command discovery", SilenceErrors: true, SilenceUsage: true}
	root.CompletionOptions.DisableDefaultCmd = true
	// Cobra's default help Run prints untrusted topics and may exit on write errors.
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Short: "Help about any command", RunE: func(cmd *cobra.Command, args []string) error {
		target := root
		for _, token := range args {
			var next *cobra.Command
			for _, child := range target.Commands() {
				if child.Name() == token {
					next = child
					break
				}
			}
			if next == nil {
				return errors.New("unknown help topic")
			}
			target = next
		}
		return target.Help()
	}})
	var version bool
	root.Flags().BoolVar(&version, "version", false, "Show structured build metadata")
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if len(args) != 0 {
			return errors.New("unexpected arguments")
		}
		if version {
			return output.Success(out, "version", build)
		}
		return cmd.Help()
	}
	handlers := map[string]handler{
		"version": func(cmd *cobra.Command, _ []string, _ map[string]any) error {
			return output.Success(out, commandName(cmd), build)
		},
		"commands":     commandsHandler(catalog, out),
		"schema":       schemaHandler(catalog, out),
		"update-check": updateCheckHandler(build, out, nil),
		"user-me": func(cmd *cobra.Command, _ []string, _ map[string]any) error {
			version, err := effectiveAPIVersion(cmd)
			if err != nil {
				return err
			}
			return apiRequest(cmd, "GET", []string{"api", version, "users", "me"}, nil, nil, false)
		},
	}
	for id, h := range connectionHandlers(out) {
		handlers[id] = h
	}
	for id, h := range authHandlers(out) {
		handlers[id] = h
	}
	for id, h := range projectHandlers() {
		handlers[id] = h
	}
	for id, h := range epicHandlers() {
		handlers[id] = h
	}
	for id, h := range workItemHandlers() {
		handlers[id] = h
	}
	for id, h := range stateLabelHandlers() {
		handlers[id] = h
	}
	for id, h := range triageHandlers() {
		handlers[id] = h
	}
	for id, h := range collaborationHandlers() {
		handlers[id] = h
	}
	for id, h := range schedulingHandlers() {
		handlers[id] = h
	}
	for id, h := range typeGovernanceHandlers() {
		handlers[id] = h
	}
	for id, h := range workspaceTypesHandlers() {
		handlers[id] = h
	}
	for id, h := range projectTypesHandlers() {
		handlers[id] = h
	}
	for _, group := range []map[string]handler{contentPagesHandlers(), contentCollectionHandlers(), contentAssetsHandlers(), assetTransferHandlers(), portfolioHandlers(), portfolioCustomerHandlers(), portfolioTemplateHandlers(), portfolioReleaseHandlers(), adminIntegrationHandlers(), adminMembershipHandlers(), adminPermissionsHandlers(), workflowHandlers(), automationHandlers(), workflowGovernanceHandlers()} {
		for id, h := range group {
			handlers[id] = h
		}
	}
	for id, h := range extra {
		handlers[id] = h
	}
	for _, descriptor := range catalog.Commands {
		d := descriptor
		h, ok := handlers[d.Handler]
		if !ok {
			return nil, errors.New("missing command handler")
		}
		parent := root
		for _, token := range d.Path[:len(d.Path)-1] {
			var group *cobra.Command
			for _, child := range parent.Commands() {
				if child.Name() == token {
					group = child
					break
				}
			}
			if group == nil {
				group = &cobra.Command{Use: token, Short: token + " commands", RunE: func(cmd *cobra.Command, args []string) error {
					if len(args) != 0 {
						return errors.New("unknown command")
					}
					return cmd.Help()
				}}
				parent.AddCommand(group)
			}
			parent = group
		}
		versions := make([]string, 0, len(d.Versions))
		for v := range d.Versions {
			versions = append(versions, v)
		}
		slices.Sort(versions)
		leaf := &cobra.Command{Use: d.Path[len(d.Path)-1], Short: d.Summary,
			Long: d.Summary + "\n\nAvailability: " + strings.Join(versions, ", ") + "\nSide effects: " + d.SideEffect + "\nInput shape: plane-cli schema " + strings.Join(d.Path, " ") + " --api-version <v1|v2>\nNext: plane-cli commands --api-version <v1|v2>", Example: strings.Join(d.Examples, "\n")}
		flags := make(map[string]spec.Flag)
		for _, version := range versions {
			for _, f := range d.Versions[version].Flags {
				if _, exists := flags[f.Name]; exists {
					continue
				}
				flags[f.Name] = f
				usage := f.Description
				for _, v := range versions {
					for _, vf := range d.Versions[v].Flags {
						if vf.Name == f.Name && vf.Required {
							usage += " (required in " + v + ")"
						}
					}
				}
				switch f.Type {
				case "string":
					leaf.Flags().String(f.Name, "", usage)
				case "boolean":
					leaf.Flags().Bool(f.Name, false, usage)
				case "integer":
					leaf.Flags().String(f.Name, "", usage+"; `integer` value")
				default:
					return nil, errors.New("unsupported flag descriptor")
				}
			}
		}
		leaf.RunE = func(cmd *cobra.Command, args []string) error {
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			version := "v2"
			if !d.Local {
				resolved, err := effectiveAPIVersion(cmd)
				if err != nil {
					return err
				}
				version = resolved
			}
			if cmd.Flags().Lookup("api-version") != nil && d.Local && (d.Handler == "schema" || d.Handler == "commands") {
				version, _ = cmd.Flags().GetString("api-version")
				if version != "v1" && version != "v2" {
					return errors.New("explicit API version required")
				}
			}
			variant, available := d.Versions[version]
			if !available {
				return unavailable(commandName(cmd))
			}
			cmd.Annotations = map[string]string{
				"pagination":           variant.Pagination,
				"table_columns":        strings.Join(variant.TableColumns, ","),
				"side_effect":          d.SideEffect,
				"empty_success_status": strconv.Itoa(variant.EmptySuccessStatus),
			}
			if variant.Route == "{upload_url}" {
				cmd.Annotations["transport"] = "storage"
			}
			if !d.Local {
				settings, err := effectiveSettings(cmd)
				if err != nil {
					return err
				}
				total, _ := settings.Deadlines()
				ctx, cancel := context.WithTimeout(cmd.Context(), total)
				defer cancel()
				cmd.SetContext(ctx)
			}
			if args == nil {
				args = []string{}
			}
			input := map[string]any{"arguments": args, "flags": map[string]any{}}
			defaults := map[string]string{}
			if !d.Local && (cmd.Flags().Lookup("workspace") != nil || cmd.Flags().Lookup("project") != nil) {
				settings, err := effectiveSettings(cmd)
				if err != nil {
					return err
				}
				defaults["workspace"], defaults["project"] = settings.Workspace, settings.Project
			}
			allowed := make(map[string]spec.Flag)
			for _, f := range variant.Flags {
				allowed[f.Name] = f
				if f.Required && !cmd.Flags().Changed(f.Name) && defaults[f.Name] == "" && (d.Local || f.Name != "api-version") {
					return &output.Error{Code: "usage_error", Message: "Missing required flag --" + f.Name, Command: commandName(cmd), Exit: 2}
				}
			}
			for name, f := range flags {
				if !d.Local && name == "api-version" {
					if err := bind(input, f.Binding, version); err != nil {
						return err
					}
					continue
				}
				if !cmd.Flags().Changed(name) && defaults[name] != "" {
					if _, ok := allowed[name]; ok {
						if err := bind(input, f.Binding, defaults[name]); err != nil {
							return err
						}
					}
					continue
				}
				if !cmd.Flags().Changed(name) {
					continue
				}
				if _, ok := allowed[name]; !ok {
					return errors.New("flag unavailable in selected version")
				}
				var value any
				switch f.Type {
				case "string":
					value, _ = cmd.Flags().GetString(name)
				case "boolean":
					value, _ = cmd.Flags().GetBool(name)
				case "integer":
					s, _ := cmd.Flags().GetString(name)
					if !integerLiteral(s) {
						return errors.New("invalid integer flag")
					}
					value = json.Number(s)
				}
				if err := bind(input, f.Binding, value); err != nil {
					return err
				}
			}
			if !d.Local {
				if err := prepareResourceBody(cmd, input, variant.Flags); err != nil {
					return err
				}
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				return err
			}
			if err := spec.ValidateWithResources(variant.Schema, encoded, catalog.Resources); err != nil {
				return err
			}
			return h(cmd, args, input)
		}
		parent.AddCommand(leaf)
	}
	return root, nil
}

func unavailable(command string) error {
	return &output.Error{Code: "operation_unavailable", Message: "Command is unavailable in this API version; use commands with an explicit --api-version to inspect availability", Command: command, Exit: 7}
}

func bind(input map[string]any, pointer string, value any) error {
	parts := strings.Split(pointer, "/")
	if len(parts) < 2 || parts[0] != "" {
		return errors.New("invalid flag binding")
	}
	m := input
	for i, part := range parts[1:] {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if i == len(parts)-2 {
			m[part] = value
			return nil
		}
		next, exists := m[part]
		if !exists {
			next = map[string]any{}
			m[part] = next
		}
		var ok bool
		m, ok = next.(map[string]any)
		if !ok {
			return errors.New("conflicting flag binding")
		}
	}
	return nil
}

func integerLiteral(s string) bool {
	s = strings.TrimPrefix(s, "-")
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
