package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/config"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

func authError(cmd *cobra.Command, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &output.Error{Code: "deadline_exceeded", Message: "Authentication deadline exceeded", Command: commandName(cmd), Exit: 4}
	}
	if errors.Is(err, context.Canceled) {
		return &output.Error{Code: "interrupted", Message: "Authentication interrupted", Command: commandName(cmd), Exit: 130}
	}
	var partial *auth.PartialError
	if errors.As(err, &partial) {
		return &output.Error{Code: "credential_persistence_failed", Message: partial.Error(), Command: commandName(cmd), Exit: 6, RemoteChanged: true}
	}
	var cleanup *auth.CleanupError
	if errors.As(err, &cleanup) {
		return &output.Error{Code: "credential_cleanup_failed", Message: cleanup.Error(), Command: commandName(cmd), Exit: 1}
	}
	var input *auth.InputError
	if errors.As(err, &input) {
		return &output.Error{Code: "usage_error", Message: input.Error(), Command: commandName(cmd), Exit: 2}
	}
	var login *auth.LoginError
	if errors.As(err, &login) {
		return &output.Error{Code: "authentication_error", Message: login.Error(), Command: commandName(cmd), Exit: 3}
	}
	if errors.Is(err, auth.ErrInvalidGrant) {
		return &output.Error{Code: "authentication_required", Message: "OAuth grant invalid or revoked; log in again", Command: commandName(cmd), Exit: 3}
	}
	return &output.Error{Code: "authentication_error", Message: "Authentication failed; check connection binding, protected input, backend access and pending login; start a fresh login if renewal was interrupted", Command: commandName(cmd), Exit: 3}
}
func authConnection(cmd *cobra.Command) (string, config.Connection, error) {
	f, err := config.Read()
	if err != nil {
		return "", config.Connection{}, err
	}
	name := f.Default
	if value := os.Getenv("PLANE_CLI_CONNECTION"); value != "" {
		name = value
	}
	if cmd.Flags().Changed("connection") {
		name, _ = cmd.Flags().GetString("connection")
	}
	c, ok := f.Connections[name]
	if !ok {
		return "", c, errors.New("select an existing named connection")
	}
	return name, c, nil
}
func protectedInput(ctx context.Context, path string, in io.Reader) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		type inputResult struct {
			data []byte
			err  error
		}
		done := make(chan inputResult, 1)
		// Readers are caller-owned. Cancellation must not close shared stdin;
		// the bounded read may finish later, but cannot persist credentials.
		go func() {
			data, err := io.ReadAll(io.LimitReader(in, 1<<20+1))
			done <- inputResult{data, err}
		}()
		select {
		case result := <-done:
			data, err = result.data, result.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	} else {
		store, e := config.Open(filepath.Dir(path), false)
		if e != nil {
			return "", &auth.InputError{Message: "Cannot open protected input directory; use an owner-only directory with safe ancestors, or stdin (-)"}
		}
		defer func() { _ = store.Close() }()
		data, err = store.Read(filepath.Base(path))
	}
	if err != nil {
		return "", &auth.InputError{Message: "Cannot read protected input; use a regular owner-only file without symlinks, or stdin (-)"}
	}
	if len(data) > 1<<20 {
		return "", &auth.InputError{Message: "Protected input exceeds 1 MiB"}
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if value == "" {
		return "", &auth.InputError{Message: "Protected input is empty"}
	}
	return value, nil
}
func authHandlers(out io.Writer) map[string]handler {
	handlers := map[string]handler{}
	for _, action := range []string{"login", "complete", "logout", "migrate"} {
		handlers["auth-"+action] = func(cmd *cobra.Command, _ []string, _ map[string]any) error {
			name, c, err := authConnection(cmd)
			if err != nil {
				return localError(cmd, err)
			}
			flag := func(key string) string { value, _ := cmd.Flags().GetString(key); return value }
			if action == "logout" || action == "migrate" {
				yes, _ := cmd.Flags().GetBool("yes")
				if !yes {
					return &output.Error{Code: "confirmation_required", Message: "Credential removal or migration requires --yes", Command: commandName(cmd), Exit: 2}
				}
			}
			if action == "login" && (c.Backend == "env" || os.Getenv("PLANE_CLI_CREDENTIAL_BACKEND") == "env") {
				return &output.Error{Code: "configuration_error", Message: "Login requires persistent credential backend file or keyring", Command: commandName(cmd), Exit: 1}
			}
			timeout := 5 * time.Minute
			var transport *auth.TransportOptions
			if action == "login" || action == "complete" {
				r, e := effectiveSettings(cmd)
				if e != nil {
					return e
				}
				if r.BaseURL != c.BaseURL || r.AuthMode != c.AuthMode || r.Backend != c.Backend {
					return settingsError(cmd, errors.New("login binding overrides require an explicit connection update"))
				}
				timeout, _ = r.Deadlines()
				_, request := r.Deadlines()
				transport = &auth.TransportOptions{RequestTimeout: request, CAFile: r.CAFile}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			m, err := auth.OpenManager()
			if err != nil {
				return localError(cmd, err)
			}
			defer func() { _ = m.Close() }()
			m.Transport = transport
			switch action {
			case "logout":
				err = m.Logout(ctx, name)
			case "migrate":
				err = m.Migrate(ctx, name, flag("backend"))
			case "complete":
				var callback string
				callback, err = protectedInput(ctx, flag("callback-file"), cmd.InOrStdin())
				if err == nil {
					err = m.Complete(ctx, name, flag("login-id"), callback)
				}
			case "login":
				if flag("api-key-file") != "" {
					if flag("flow") != "" || flag("client-secret-file") != "" {
						return &output.Error{Code: "usage_error", Message: "Token import and OAuth login options cannot be combined", Command: commandName(cmd), Exit: 2}
					}
					var token string
					token, err = protectedInput(ctx, flag("api-key-file"), cmd.InOrStdin())
					if err == nil {
						err = m.Import(ctx, name, token)
					}
				} else {
					flow := flag("flow")
					if flow != "user" && flow != "bot" {
						return &output.Error{Code: "usage_error", Message: "Supply --api-key-file or --flow user|bot", Command: commandName(cmd), Exit: 2}
					}
					if c.AuthMode != "oauth-"+flow {
						return settingsError(cmd, errors.New("connection auth mode must match OAuth flow"))
					}
					option := func(key, env, saved string) string {
						if cmd.Flags().Changed(key) {
							return flag(key)
						}
						if v := os.Getenv("PLANE_CLI_" + env); v != "" {
							return v
						}
						return saved
					}
					o := auth.LoginOptions{ClientID: option("client-id", "OAUTH_CLIENT_ID", c.OAuthClientID), RedirectURI: option("redirect-uri", "REDIRECT_URI", c.RedirectURI), Scopes: option("scopes", "SCOPES", c.Scopes), InstallationID: option("installation-id", "INSTALLATION_ID", c.InstallationID), ClientSecret: os.Getenv("PLANE_CLI_OAUTH_CLIENT_SECRET")}
					if source := flag("client-secret-file"); source != "" {
						if o.ClientSecret != "" {
							return &output.Error{Code: "usage_error", Message: "Client-secret file and environment are mutually exclusive", Command: commandName(cmd), Exit: 2}
						}
						o.ClientSecret, err = protectedInput(ctx, source, cmd.InOrStdin())
						if err != nil {
							return authError(cmd, err)
						}
					}
					headless, _ := cmd.Flags().GetBool("headless")
					if headless || flow == "bot" && o.InstallationID != "" {
						var id, consent string
						id, consent, err = m.Begin(ctx, name, o)
						if err == nil && id != "" {
							if c.Backend == "file" && c.CredentialRef == "" {
								_ = output.Warning(cmd.ErrOrStderr(), commandName(cmd), "plaintext_credentials", "File credentials are owner-only plaintext at rest")
							}
							return output.Success(out, commandName(cmd), map[string]any{"status": "pending", "connection": name, "login_id": id, "consent_url": consent})
						}
					} else {
						err = localConsent(ctx, cmd, m, name, o)
					}
				}
			}
			if err != nil {
				return authError(cmd, err)
			}
			if (action == "login" || action == "complete") && c.Backend == "file" && c.CredentialRef == "" || action == "migrate" && c.Backend != "file" && flag("backend") == "file" {
				_ = output.Warning(cmd.ErrOrStderr(), commandName(cmd), "plaintext_credentials", "File credentials are owner-only plaintext at rest")
			}
			data := map[string]any{"connection": name, "status": "completed"}
			if action != "logout" {
				current, e := m.Store.Load()
				if e != nil {
					return localError(cmd, e)
				}
				_, status, e := m.Resolve(ctx, config.Resolved{Name: name, Connection: current.Connections[name]}, false)
				if e != nil {
					return authError(cmd, e)
				}
				data["auth"] = status
			}
			return output.Success(out, commandName(cmd), data)
		}
	}
	return handlers
}

func localConsent(ctx context.Context, cmd *cobra.Command, m *auth.Manager, name string, o auth.LoginOptions) (result error) {
	u, err := auth.ValidateRedirect(o.RedirectURI)
	if err != nil {
		return err
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if u.Scheme != "http" || u.Port() == "" || u.Port() == "0" || host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return &auth.InputError{Message: "Local login requires a registered HTTP loopback URI with explicit nonzero port; use --headless otherwise"}
	}
	listener, err := net.Listen("tcp", u.Host)
	if err != nil {
		return &auth.LoginError{Message: "Could not bind registered callback listener; check that its address and port are available, or use --headless"}
	}
	defer func() { _ = listener.Close() }()
	id, consent, err := m.Begin(ctx, name, o)
	if err != nil {
		return err
	}
	defer func() {
		// Cleanup must survive cancellation, but remains independently bounded.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := m.Cancel(cleanup, name, id); err != nil {
			result = errors.Join(result, &auth.CleanupError{Phase: "Local login ended; pending credential cleanup failed; run auth logout --yes"})
		}
	}()
	results := make(chan error, 1)
	callbackPath := u.EscapedPath()
	if callbackPath == "" {
		callbackPath = "/"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.EscapedPath() != callbackPath || r.Host != u.Host {
			http.Error(w, "Unexpected callback", http.StatusBadRequest)
			return
		}
		callback := *u
		callback.RawQuery = r.URL.RawQuery
		err := m.Complete(ctx, name, id, callback.String())
		if err != nil {
			http.Error(w, "Authorization could not be completed", http.StatusBadRequest)
			_ = http.NewResponseController(w).Flush()
			var cleanup *auth.CleanupError
			if errors.Is(err, auth.ErrLoginEnded) || errors.As(err, &cleanup) {
				select {
				case results <- err:
				default:
				}
			}
			return
		}
		_, _ = io.WriteString(w, "Authorization complete. You may close this window.")
		_ = http.NewResponseController(w).Flush()
		select {
		case results <- nil:
		default:
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second}
	defer func() { _ = server.Close() }()
	go func() {
		err := server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			select {
			case results <- &auth.LoginError{Message: "Callback listener stopped; start a fresh login"}:
			default:
			}
		}
	}()
	if _, err = io.WriteString(cmd.ErrOrStderr(), "Authorize this registered app: "+consent+"\n"); err != nil {
		return err
	}
	select {
	case err = <-results:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
