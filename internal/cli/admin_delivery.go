package cli

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/httpx"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// Webhook creation and rotation return a secret that cannot be read again.
// Prepare protected storage before the mutation, never retry a delivery failure.
func apiWebhookSecretRequest(cmd *cobra.Command, segments []string, query url.Values, body json.RawMessage) error {
	r, err := effectiveSettings(cmd)
	if err != nil {
		return err
	}
	dry, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	target, err := httpx.RequestURL(r.BaseURL, segments, query)
	if err != nil {
		return &output.Error{Code: "usage_error", Message: "Invalid webhook request path", Command: commandName(cmd), Exit: 2}
	}
	if dry {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"method": "POST", "url": target, "body_present": len(body) != 0, "api_version": r.APIVersion, "requires_confirmation": true, "dry_run": true})
	}
	if !yes {
		return &output.Error{Code: "confirmation_required", Message: "This operation requires --yes", Command: commandName(cmd), Exit: 2}
	}
	path, _ := cmd.Flags().GetString("secret-output-file")
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	var destination *assetDestination
	if path != "" {
		destination, err = prepareAssetDestination(path, overwrite, yes)
		if err != nil {
			return &output.Error{Code: "output_error", Message: "Cannot prepare protected webhook output; no request sent", Command: commandName(cmd), Exit: 1}
		}
		defer destination.discard()
	} else if overwrite {
		return &output.Error{Code: "usage_error", Message: "--overwrite requires --secret-output-file", Command: commandName(cmd), Exit: 2}
	}
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}
	credential, status, err := auth.ResolveStored(ctx, r, os.Getenv, true)
	if err != nil {
		return authError(cmd, err)
	}
	if status.Status == "missing" {
		return &output.Error{Code: "authentication_required", Message: "Run auth login or set the matching credential environment variable", Command: commandName(cmd), Exit: 3}
	}
	_, request := r.Deadlines()
	client, err := httpx.New(r.BaseURL, request, r.CAFile)
	if err != nil {
		return &output.Error{Code: "configuration_error", Message: "Could not initialize webhook transport", Command: commandName(cmd), Exit: 1}
	}
	response, err := client.Do(ctx, "POST", segments, query, body, credential.Header, credential.Token)
	if err != nil {
		return apiTransportError(cmd, r.APIVersion, err)
	}
	deliveryFailure := func(delivered bool) error {
		return &output.Error{Code: "output_error", Message: "Could not deliver webhook result", Command: commandName(cmd), Exit: 6, APIVersion: r.APIVersion, HTTPStatus: response.Status, RemoteChanged: true, SecretDelivered: &delivered, Phase: "delivery", Recovery: "Inspect webhook list/get for the affected webhook. Reads do not return the secret; if delivery failed, an operator must explicitly rotate it with webhook regenerate-secret --yes. No retry or rollback was attempted."}
	}
	var secret struct {
		Key string `json:"secret_key"`
	}
	if json.Unmarshal(response.Data, &secret) != nil || secret.Key == "" {
		return deliveryFailure(false)
	}
	data := response.Data
	delivered := false
	if destination != nil {
		if err = output.API(destination.file, commandName(cmd), r.APIVersion, response.Status, response.Data); err != nil {
			return deliveryFailure(false)
		}
		if err = destination.install(); err != nil {
			return deliveryFailure(false)
		}
		delivered = true
		var native map[string]json.RawMessage
		if err = json.Unmarshal(redactAssetData(redactAdminResponse(cmd, response.Data)), &native); err != nil {
			return deliveryFailure(true)
		}
		delete(native, "secret_key")
		data, err = json.Marshal(map[string]any{"secret_output_file": path, "native": native})
		if err != nil {
			return deliveryFailure(true)
		}
	}
	if err = output.API(cmd.OutOrStdout(), commandName(cmd), r.APIVersion, response.Status, data); err != nil {
		return deliveryFailure(delivered)
	}
	return nil
}

// Delivery logs can embed signing credentials inside request headers or bodies.
// Creation/rotation bypass this ordinary-output path to deliver their one-time secret.
func redactAdminResponse(cmd *cobra.Command, raw json.RawMessage) json.RawMessage {
	if !strings.HasPrefix(commandName(cmd), "webhook ") {
		return raw
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return raw
	}
	var redact func(any)
	redact = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				switch strings.ToLower(key) {
				case "secret_key", "authorization", "proxy-authorization", "x-api-key", "cookie", "set-cookie", "access_token", "refresh_token", "x-plane-signature":
					value[key] = "[REDACTED]"
				case "request_headers", "response_headers":
					// Upstream logs may serialize headers as opaque text.
					if _, opaque := child.(string); opaque {
						value[key] = "[REDACTED]"
					} else {
						redact(child)
					}
				default:
					redact(child)
				}
			}
		case []any:
			for _, child := range value {
				redact(child)
			}
		}
	}
	redact(value)
	result, _ := json.Marshal(value)
	return result
}
