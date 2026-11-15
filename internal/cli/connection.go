package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/config"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

func localError(cmd *cobra.Command, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var structured *output.Error
	if errors.As(err, &structured) {
		return structured
	}
	// Storage errors can contain local paths; only controlled guidance is public.
	return &output.Error{Code: "configuration_error", Message: "Could not read or update configuration; check its version, permissions and selected connection", Command: commandName(cmd), Exit: 1}
}
func settingFlags(cmd *cobra.Command) map[string]string {
	values := map[string]string{}
	for _, name := range []string{"connection", "base-url", "api-version", "workspace", "project", "auth-mode", "credential-backend", "timeout", "request-timeout", "ca-file"} {
		if cmd.Flags().Lookup(name) != nil && cmd.Flags().Changed(name) {
			values[name], _ = cmd.Flags().GetString(name)
		}
	}
	return values
}
func connectionHandlers(out io.Writer) map[string]handler {
	handlers := map[string]handler{}
	for _, action := range []string{"add", "update", "list", "show", "use", "delete"} {
		handlers["connection-"+action] = func(cmd *cobra.Command, args []string, _ map[string]any) error {
			if action == "list" {
				f, e := config.Read()
				if e != nil {
					return localError(cmd, e)
				}
				return output.Success(out, commandName(cmd), f)
			}
			if action == "show" {
				f, e := config.Read()
				if e != nil {
					return localError(cmd, e)
				}
				flags := settingFlags(cmd)
				if len(args) > 0 {
					flags["connection"] = args[0]
				}
				r, e := config.Resolve(f, flags, os.Getenv)
				if e != nil {
					return settingsError(cmd, e)
				}
				return output.Success(out, commandName(cmd), r)
			}
			yes, _ := cmd.Flags().GetBool("yes")
			dry, _ := cmd.Flags().GetBool("dry-run")
			if action == "delete" && !yes && !dry {
				return &output.Error{Code: "confirmation_required", Message: "Connection deletion requires --yes", Command: commandName(cmd), Exit: 2}
			}
			name := args[0]
			if strings.TrimSpace(name) == "" {
				return errors.New("empty connection name")
			}
			flags := settingFlags(cmd)
			// Validate supplied settings before creating local directories or locks.
			if action == "add" {
				_, e := validateConnection(connectionValues(config.Connection{AuthMode: "api-key", Backend: "file"}, flags))
				if e != nil {
					return &output.Error{Code: "usage_error", Message: e.Error(), Command: commandName(cmd), Exit: 2}
				}
			}
			if dry {
				f, e := config.Read()
				if e != nil {
					return localError(cmd, e)
				}
				current, exists := f.Connections[name]
				if (action == "add" && exists) || (action != "add" && !exists) {
					return localError(cmd, errors.New("connection existence conflict"))
				}
				if action == "update" {
					if _, e := validateConnection(connectionValues(current, flags)); e != nil {
						return &output.Error{Code: "usage_error", Message: e.Error(), Command: commandName(cmd), Exit: 2}
					}
				}
				return output.Success(out, commandName(cmd), map[string]any{"connection": name, "dry_run": true, "requires_confirmation": action == "delete"})
			}
			path, e := config.Directory()
			if e != nil {
				return localError(cmd, e)
			}
			store, e := config.Open(path, true)
			if e != nil {
				return localError(cmd, e)
			}
			defer func() { _ = store.Close() }()
			// Configuration-only use never waits on a connection lock.
			if action != "use" {
				unlock, e := store.ConnectionLock(cmd.Context(), name)
				if e != nil {
					return localError(cmd, e)
				}
				defer func() { _ = unlock() }()
			}
			// Secret cleanup may contact a keyring and must precede the config lock.
			if action == "delete" || action == "update" {
				f, err := store.Load()
				if err != nil {
					return localError(cmd, err)
				}
				old, exists := f.Connections[name]
				if !exists {
					return localError(cmd, errors.New("connection not found"))
				}
				next, err := validateConnection(connectionValues(old, flags))
				if err != nil {
					return &output.Error{Code: "usage_error", Message: err.Error(), Command: commandName(cmd), Exit: 2}
				}
				rebind := next.BaseURL != old.BaseURL || next.AuthMode != old.AuthMode || next.Backend != old.Backend
				if action == "delete" || rebind {
					manager := auth.Manager{Store: store, Keyring: auth.NativeKeyring()}
					if err := manager.CleanupLocked(cmd.Context(), old); err != nil {
						return authError(cmd, err)
					}
				}
			}
			warning := false
			e = store.Update(cmd.Context(), func(f *config.File) error {
				old, exists := f.Connections[name]
				switch action {
				case "use":
					if !exists {
						return errors.New("connection not found")
					}
					f.Default = name
				case "delete":
					if !exists {
						return errors.New("connection not found")
					}
					delete(f.Connections, name)
					if f.Default == name {
						f.Default = ""
					}
				case "add", "update":
					if (action == "add" && exists) || (action == "update" && !exists) {
						return errors.New("connection existence conflict")
					}
					next := old
					if action == "add" {
						id, e := config.NewIdentity()
						if e != nil {
							return e
						}
						next = config.Connection{ID: id, Generation: 1, AuthMode: "api-key", Backend: "file"}
					}
					next = connectionValues(next, flags)
					var e error
					next, e = validateConnection(next)
					if e != nil {
						return &output.Error{Code: "usage_error", Message: e.Error(), Command: commandName(cmd), Exit: 2}
					}
					if action == "update" && (next.BaseURL != old.BaseURL || next.AuthMode != old.AuthMode || next.Backend != old.Backend) {
						next.Generation++
						next.CredentialRef = ""
						next.CleanupRef = ""
						next.CleanupBackend = ""
						next.StagedRef = ""
						next.StagedBackend = ""
					}
					warning = strings.HasPrefix(next.BaseURL, "http://") && (action == "add" || strings.HasPrefix(old.BaseURL, "https://"))
					f.Connections[name] = next
				}
				return nil
			})
			if e != nil {
				return localError(cmd, e)
			}
			if warning {
				// Informational diagnostics cannot change a committed command's status.
				_ = output.Warning(cmd.ErrOrStderr(), commandName(cmd), "http_connection", "HTTP does not encrypt traffic")
			}
			return output.Success(out, commandName(cmd), map[string]any{"connection": name, "status": "completed"})
		}
	}
	handlers["auth-status"] = func(cmd *cobra.Command, _ []string, _ map[string]any) error {
		r, e := effectiveSettings(cmd)
		if e != nil {
			return e
		}
		total, _ := r.Deadlines()
		ctx, cancel := context.WithTimeout(cmd.Context(), total)
		defer cancel()
		_, status, e := auth.ResolveStored(ctx, r, os.Getenv, false)
		if e != nil {
			return authError(cmd, e)
		}
		return output.Success(out, commandName(cmd), status)
	}
	return handlers
}
func connectionValues(c config.Connection, flags map[string]string) config.Connection {
	fields := map[string]*string{"base-url": &c.BaseURL, "api-version": &c.APIVersion, "workspace": &c.Workspace, "project": &c.Project, "auth-mode": &c.AuthMode, "credential-backend": &c.Backend, "timeout": &c.Timeout, "request-timeout": &c.RequestTimeout, "ca-file": &c.CAFile}
	for key, dst := range fields {
		if s, ok := flags[key]; ok {
			*dst = s
		}
	}
	return c
}

func validateConnection(c config.Connection) (config.Connection, error) {
	return config.Validate(c)
}

func settingsError(cmd *cobra.Command, err error) error {
	return &output.Error{Code: "configuration_error", Message: err.Error(), Command: commandName(cmd), Exit: 1}
}

func effectiveSettings(cmd *cobra.Command) (config.Resolved, error) {
	flags := settingFlags(cmd)
	if cmd.Annotations["transport"] == "storage" {
		// Signed storage destinations do not require a configured Plane API origin.
		flags["base-url"] = "http://storage.invalid"
	}
	f, err := config.Read()
	if err != nil {
		// Explicit settings can operate without a usable local store, but an
		// explicitly selected connection must never silently disappear.
		_, selected := flags["connection"]
		if !selected && os.Getenv("PLANE_CLI_CONNECTION") == "" {
			empty := config.File{Version: config.Version, Connections: map[string]config.Connection{}}
			if r, resolveErr := config.Resolve(empty, flags, os.Getenv); resolveErr == nil {
				return r, nil
			}
		}
		return config.Resolved{}, localError(cmd, err)
	}
	r, err := config.Resolve(f, flags, os.Getenv)
	if err != nil {
		return r, settingsError(cmd, err)
	}
	return r, nil
}

func effectiveAPIVersion(cmd *cobra.Command) (string, error) {
	flags := settingFlags(cmd)
	version, explicit := flags["api-version"]
	if !explicit {
		version = os.Getenv("PLANE_CLI_API_VERSION")
	}
	if !explicit && version == "" {
		f, err := config.Read()
		if err != nil {
			return "", localError(cmd, err)
		}
		name := f.Default
		if value := os.Getenv("PLANE_CLI_CONNECTION"); value != "" {
			name = value
		}
		if value, ok := flags["connection"]; ok {
			name = value
		}
		c, ok := f.Connections[name]
		if !ok {
			return "", settingsError(cmd, errors.New("select an existing connection or supply an API version"))
		}
		version = c.APIVersion
	}
	if version != "v1" && version != "v2" {
		return "", settingsError(cmd, errors.New("API version must be v1 or v2"))
	}
	return version, nil
}
