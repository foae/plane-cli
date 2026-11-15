package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

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
	_, request := r.Deadlines()
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}
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
	client.EmptySuccessStatus, _ = strconv.Atoi(cmd.Annotations["empty_success_status"])
	mutation := cmd.Annotations["side_effect"] != "none" && cmd.Annotations["side_effect"] != "read"
	var response httpx.Response
	if mutation {
		response, e = client.DoMutation(ctx, method, segments, query, body, credential.Header, credential.Token)
	} else {
		response, e = client.DoRead(ctx, method, segments, query, body, credential.Header, credential.Token)
	}
	if e != nil {
		return apiTransportError(cmd, r.APIVersion, e)
	}
	all, _ := cmd.Flags().GetBool("all")
	if all {
		return aggregatePages(cmd, ctx, client, segments, query, credential.Header, credential.Token, r.APIVersion, response)
	}
	response.Data = redactAdminResponse(cmd, response.Data)
	format, _ := cmd.Flags().GetString("output")
	if format == "table" {
		var rendered bytes.Buffer
		if e = tablePage(&rendered, response.Data, strings.Split(cmd.Annotations["table_columns"], ","), true); e != nil {
			return &output.Error{Code: "invalid_response", Message: "Cannot render table response", Command: commandName(cmd), Exit: 5}
		}
		if _, e = rendered.WriteTo(cmd.OutOrStdout()); e != nil {
			return &output.Error{Code: "output_error", Message: "Cannot deliver table result", Command: commandName(cmd), Exit: 1}
		}
		return nil
	}
	partial := false
	switch commandName(cmd) {
	case "project bulk-create", "project bulk-update",
		"work-item bulk-create", "work-item bulk-update", "work-item bulk-delete",
		"state bulk-create", "state bulk-update", "state bulk-delete",
		"label bulk-create", "label bulk-update", "label bulk-delete",
		"work-item-comment bulk-create", "work-item-comment bulk-update", "work-item-comment bulk-delete",
		"cycle bulk-create", "cycle bulk-update", "cycle bulk-delete",
		"module bulk-create", "module bulk-update", "module bulk-delete",
		"milestone bulk-create", "milestone bulk-update", "milestone bulk-delete",
		"estimate bulk-create", "estimate bulk-update", "estimate bulk-delete",
		"estimate point bulk-create", "estimate point bulk-update", "estimate point bulk-delete":
		var bulk struct {
			Failed *int `json:"failed"`
		}
		if json.Unmarshal(response.Data, &bulk) != nil || bulk.Failed == nil || *bulk.Failed < 0 {
			return &output.Error{Code: "invalid_response", Message: "API returned an invalid bulk outcome", Command: commandName(cmd), Exit: 5, APIVersion: r.APIVersion, HTTPStatus: response.Status, OutcomeUnknown: true}
		}
		partial = *bulk.Failed > 0
	}
	upsert := ""
	if strings.HasSuffix(commandName(cmd), " upsert") {
		upsert = response.Upsert
	}
	if e = output.APIResult(cmd.OutOrStdout(), commandName(cmd), r.APIVersion, response.Status, response.Data, partial, upsert); e != nil {
		exit := 1
		if mutation {
			exit = 6
		}
		return &output.Error{Code: "output_error", Message: "Could not deliver API result", Command: commandName(cmd), Exit: exit, APIVersion: r.APIVersion, HTTPStatus: response.Status, RemoteChanged: mutation}
	}
	if partial {
		return &output.Error{Code: "partial_failure", Message: "Some bulk operations failed; inspect individual results", Command: commandName(cmd), Exit: 6, APIVersion: r.APIVersion, HTTPStatus: response.Status}
	}
	return nil
}

func apiTransportError(cmd *cobra.Command, version string, err error) error {
	var remote *httpx.Error
	if errors.As(err, &remote) {
		return &output.Error{Code: remote.Code, Message: remote.Message, Command: commandName(cmd), Exit: remote.Exit, APIVersion: version, HTTPStatus: remote.Status, UpstreamCode: remote.UpstreamCode, RetryAfterSeconds: remote.RetryAfterSeconds, OutcomeUnknown: remote.OutcomeUnknown}
	}
	return &output.Error{Code: "transport_error", Message: "API request failed", Command: commandName(cmd), Exit: 4, APIVersion: version}
}
