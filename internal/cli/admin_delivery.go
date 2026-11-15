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

// Prepare protected storage before issuing a one-time secret; never retry a
// delivery failure. Secret paths are fixed by the operation's public contract.
func apiIntentionalSecretRequest(cmd *cobra.Command, method string, segments []string, query url.Values, body json.RawMessage, secretPaths []string, recovery string) error {
	r, err := effectiveSettings(cmd)
	if err != nil {
		return err
	}
	dry, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	target, err := httpx.RequestURL(r.BaseURL, segments, query)
	if err != nil {
		return &output.Error{Code: "usage_error", Message: "Invalid secret issuance request path", Command: commandName(cmd), Exit: 2}
	}
	path, _ := cmd.Flags().GetString("secret-output-file")
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	if overwrite && path == "" {
		return &output.Error{Code: "usage_error", Message: "--overwrite requires --secret-output-file", Command: commandName(cmd), Exit: 2}
	}
	if dry {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"method": method, "url": target, "body_present": len(body) != 0, "api_version": r.APIVersion, "requires_confirmation": true, "dry_run": true})
	}
	if !yes {
		return &output.Error{Code: "confirmation_required", Message: "This operation requires --yes", Command: commandName(cmd), Exit: 2}
	}
	var destination *assetDestination
	if path != "" {
		destination, err = prepareAssetDestination(path, overwrite, yes)
		if err != nil {
			return &output.Error{Code: "output_error", Message: "Cannot prepare protected secret output; no request sent", Command: commandName(cmd), Exit: 1}
		}
		defer destination.discard()
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
		return &output.Error{Code: "configuration_error", Message: "Could not initialize secret issuance transport", Command: commandName(cmd), Exit: 1}
	}
	response, err := client.DoMutation(ctx, method, segments, query, body, credential.Header, credential.Token)
	if err != nil {
		return apiTransportError(cmd, r.APIVersion, err)
	}
	deliveryFailure := func(delivered bool) error {
		return &output.Error{Code: "output_error", Message: "Could not deliver secret issuance result", Command: commandName(cmd), Exit: 6, APIVersion: r.APIVersion, HTTPStatus: response.Status, RemoteChanged: true, SecretDelivered: &delivered, Phase: "delivery", Recovery: recovery}
	}
	var native map[string]any
	decoder := json.NewDecoder(bytes.NewReader(response.Data))
	decoder.UseNumber()
	if decoder.Decode(&native) != nil {
		return deliveryFailure(false)
	}
	for _, pointer := range secretPaths {
		var value any = native
		for _, key := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
			object, ok := value.(map[string]any)
			if !ok {
				return deliveryFailure(false)
			}
			value = object[key]
		}
		if secret, ok := value.(string); !ok || secret == "" {
			return deliveryFailure(false)
		}
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
	name := commandName(cmd)
	if !strings.HasPrefix(name, "webhook ") && !strings.HasPrefix(name, "automation ") && !strings.HasPrefix(name, "workflow ") {
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
			// Hook headers encode the credential name as data, not a property name.
			header, _ := value["key"].(string)
			secret, _ := value["is_secret"].(bool)
			switch strings.ToLower(header) {
			case "authorization", "proxy-authorization", "x-api-key", "cookie", "set-cookie", "x-plane-signature":
				secret = true
			}
			if secret {
				if _, present := value["value"]; present {
					value["value"] = "[REDACTED]"
				}
			}
			for key, child := range value {
				switch strings.ToLower(key) {
				case "secret", "secret_plaintext", "secret_key", "authorization", "proxy-authorization", "x-api-key", "cookie", "set-cookie", "access_token", "refresh_token", "x-plane-signature":
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
