package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/httpx"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

// A sibling temporary file keeps both installation modes on one filesystem.
// Link is the no-clobber commit: an intervening destination creation must win.
type assetDestination struct {
	file              *os.File
	target, temporary string
	overwrite         bool
}

func prepareAssetDestination(path string, overwrite, yes bool) (*assetDestination, error) {
	if path == "" || (overwrite && !yes) {
		return nil, errors.New("output path and explicit overwrite confirmation required")
	}
	info, err := os.Lstat(path)
	if err == nil {
		if !overwrite || !info.Mode().IsRegular() {
			return nil, errors.New("destination exists or is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("cannot inspect destination")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".plane-cli-transfer-*")
	if err != nil {
		return nil, errors.New("cannot prepare destination")
	}
	return &assetDestination{file: f, target: path, temporary: f.Name(), overwrite: overwrite}, nil
}
func (d *assetDestination) discard() { _ = d.file.Close(); _ = os.Remove(d.temporary) }
func (d *assetDestination) install() error {
	if err := d.file.Sync(); err != nil {
		return err
	}
	if err := d.file.Close(); err != nil {
		return err
	}
	if d.overwrite {
		return os.Rename(d.temporary, d.target)
	}
	return os.Link(d.temporary, d.target)
}

// Signed fields never reach ordinary results, even nested in native metadata.
func redactAssetData(raw json.RawMessage) json.RawMessage {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return json.RawMessage("null")
	}
	var redact func(any)
	redact = func(v any) {
		switch item := v.(type) {
		case map[string]any:
			for key, child := range item {
				switch strings.ToLower(key) {
				case "upload_data", "asset_url", "download_url", "signed_url", "presigned_url", "authorization", "x-api-key", "access_token", "refresh_token":
					item[key] = "[REDACTED]"
				default:
					redact(child)
				}
			}
		case []any:
			for _, child := range item {
				redact(child)
			}
		}
	}
	redact(value)
	result, _ := json.Marshal(value)
	return result
}

func assetFailure(cmd *cobra.Command, code, message string, exit int) *output.Error {
	return &output.Error{Code: code, Message: message, Command: commandName(cmd), Exit: exit}
}

func assetDeliveryFailure(cmd *cobra.Command, version string, response httpx.Response, mutation, delivered bool) error {
	exit := 1
	if mutation {
		exit = 6
	}
	e := assetFailure(cmd, "output_error", "Could not deliver asset result", exit)
	e.APIVersion = version
	e.HTTPStatus = response.Status
	e.RemoteChanged = mutation
	e.SecretDelivered = &delivered
	e.Phase = "delivery"
	var data struct {
		AssetID string `json:"asset_id"`
	}
	_ = json.Unmarshal(response.Data, &data)
	e.AssetID = data.AssetID
	e.Recovery = "Inspect the asset using its documented get operation if available; otherwise operator action is required. No automatic retry or rollback was attempted."
	return e
}

func apiAssetRequest(cmd *cobra.Command, method string, segments []string, query url.Values, body json.RawMessage, destructive, sensitive bool) error {
	r, err := effectiveSettings(cmd)
	if err != nil {
		return err
	}
	dry, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	target, err := httpx.RequestURL(r.BaseURL, segments, query)
	if err != nil {
		return assetFailure(cmd, "usage_error", "Invalid asset request path", 2)
	}
	if dry {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"method": method, "url": target, "body_present": len(body) != 0, "api_version": r.APIVersion, "requires_confirmation": destructive, "dry_run": true})
	}
	if destructive && !yes {
		return assetFailure(cmd, "confirmation_required", "This operation requires --yes", 2)
	}
	path, _ := cmd.Flags().GetString("secret-output-file")
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	var destination *assetDestination
	if sensitive && path != "" {
		destination, err = prepareAssetDestination(path, overwrite, yes)
		if err != nil {
			return assetFailure(cmd, "output_error", "Cannot prepare protected asset output; no request sent", 1)
		}
		defer destination.discard()
	} else if overwrite {
		return assetFailure(cmd, "usage_error", "--overwrite requires --secret-output-file", 2)
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
		return assetFailure(cmd, "authentication_required", "Run auth login or set the matching credential environment variable", 3)
	}
	_, request := r.Deadlines()
	client, err := httpx.New(r.BaseURL, request, r.CAFile)
	if err != nil {
		return assetFailure(cmd, "configuration_error", "Could not initialize asset transport", 1)
	}
	response, err := client.Do(ctx, method, segments, query, body, credential.Header, credential.Token)
	if err != nil {
		return apiTransportError(cmd, r.APIVersion, err)
	}
	mutation := method != "GET" && method != "HEAD"
	data := redactAssetData(response.Data)
	delivered := false
	if sensitive {
		if destination == nil {
			data = response.Data
		} else {
			if err = output.API(destination.file, commandName(cmd), r.APIVersion, response.Status, response.Data); err != nil {
				return assetDeliveryFailure(cmd, r.APIVersion, response, mutation, false)
			}
			if err = destination.install(); err != nil {
				return assetDeliveryFailure(cmd, r.APIVersion, response, mutation, false)
			}
			delivered = true
			data, err = json.Marshal(map[string]any{"secret_output_file": path, "native": json.RawMessage(data)})
			if err != nil {
				return assetDeliveryFailure(cmd, r.APIVersion, response, mutation, true)
			}
		}
	}
	if err = output.API(cmd.OutOrStdout(), commandName(cmd), r.APIVersion, response.Status, data); err != nil {
		if sensitive {
			return assetDeliveryFailure(cmd, r.APIVersion, response, mutation, delivered)
		}
		exit := 1
		if mutation {
			exit = 6
		}
		e := assetFailure(cmd, "output_error", "Could not deliver asset result", exit)
		e.RemoteChanged = mutation
		return e
	}
	return nil
}
