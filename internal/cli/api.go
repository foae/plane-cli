package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/httpx"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// apiRequest is called only after the operation's complete input schema has
// passed. No credential or network work occurs before confirmation/dry-run.
func apiRequest(cmd *cobra.Command, method string, segments []string, query url.Values, body json.RawMessage, destructive bool) error {
	r, e := effectiveSettings(cmd)
	if e != nil {
		return e
	}
	dry, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	target, e := httpx.RequestURL(r.BaseURL, segments, query)
	if e != nil {
		return &output.Error{Code: "usage_error", Message: "Invalid API request path", Command: commandName(cmd), Exit: 2}
	}
	if dry {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"method": method, "url": target, "body_present": len(body) != 0, "api_version": r.APIVersion, "requires_confirmation": destructive, "dry_run": true})
	}
	if destructive && !yes {
		return &output.Error{Code: "confirmation_required", Message: "This operation requires --yes", Command: commandName(cmd), Exit: 2}
	}
	total, request := r.Deadlines()
	ctx, cancel := context.WithTimeout(cmd.Context(), total)
	defer cancel()
	credential, status, e := auth.ResolveStored(ctx, r, os.Getenv, true)
	if e != nil {
		return authError(cmd, e)
	}
	if status.Status == "missing" {
		return &output.Error{Code: "authentication_required", Message: "Run auth login or set the matching PLANE_CLI_API_KEY or PLANE_CLI_ACCESS_TOKEN", Command: commandName(cmd), Exit: 3}
	}
	client, e := httpx.New(r.BaseURL, request, r.CAFile)
	if e != nil {
		return &output.Error{Code: "configuration_error", Message: "Could not initialize API transport or CA certificates", Command: commandName(cmd), Exit: 1}
	}
	response, e := client.Do(ctx, method, segments, query, body, credential.Header, credential.Token)
	if e != nil {
		var remote *httpx.Error
		if errors.As(e, &remote) {
			return &output.Error{Code: remote.Code, Message: remote.Message, Command: commandName(cmd), Exit: remote.Exit, APIVersion: r.APIVersion, HTTPStatus: remote.Status, RetryAfterSeconds: remote.RetryAfterSeconds, OutcomeUnknown: remote.OutcomeUnknown}
		}
		return &output.Error{Code: "transport_error", Message: "API request failed", Command: commandName(cmd), Exit: 4, APIVersion: r.APIVersion}
	}
	if e = output.API(cmd.OutOrStdout(), commandName(cmd), r.APIVersion, response.Status, response.Data); e != nil {
		mutation := method != "GET" && method != "HEAD" && method != "OPTIONS"
		exit := 1
		if mutation {
			exit = 6
		}
		return &output.Error{Code: "output_error", Message: "Could not deliver API result", Command: commandName(cmd), Exit: exit, APIVersion: r.APIVersion, HTTPStatus: response.Status, RemoteChanged: mutation}
	}
	return nil
}
