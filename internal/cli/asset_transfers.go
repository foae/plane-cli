package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/httpx"
	"github.com/foae/plane-cli/internal/output"
	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

type assetSession struct {
	api, storage           *httpx.Client
	header, token, version string
}

func openAssetSession(cmd *cobra.Command) (assetSession, error) {
	var s assetSession
	r, err := effectiveSettings(cmd)
	if err != nil {
		return s, err
	}
	s.version = r.APIVersion
	credential, status, err := auth.ResolveStored(cmd.Context(), r, os.Getenv, true)
	if err != nil {
		return s, authError(cmd, err)
	}
	if status.Status == "missing" {
		return s, assetFailure(cmd, "authentication_required", "Run auth login or set the matching credential environment variable", 3)
	}
	_, request := r.Deadlines()
	s.header = credential.Header
	s.token = credential.Token
	s.api, err = httpx.New(r.BaseURL, request, r.CAFile)
	if err != nil {
		return s, assetFailure(cmd, "configuration_error", "Could not initialize asset transport", 1)
	}
	// A distinct client never receives Plane credentials, cookies or API headers.
	// Streaming is bounded by the invocation context, not the JSON request timeout.
	s.storage, err = httpx.New("http://storage.invalid", 0, r.CAFile)
	if err != nil {
		return s, assetFailure(cmd, "configuration_error", "Could not initialize storage transport", 1)
	}
	return s, nil
}

func assetTransferHandlers() map[string]handler {
	handlers := map[string]handler{}
	for _, group := range []string{"attachment", "workspace-page-attachment", "user-asset", "workspace-asset"} {
		handlers[group+"-upload"] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return uploadAsset(cmd, group, input)
		}
	}
	for _, group := range []string{"workspace-page-attachment", "workspace-asset"} {
		handlers[group+"-download"] = func(cmd *cobra.Command, _ []string, input map[string]any) error {
			return downloadAsset(cmd, group, input)
		}
	}
	return handlers
}
func assetTransferPath(group string, flags map[string]any) []string {
	version := flags["api-version"].(string)
	if group == "user-asset" {
		return []string{"api", version, "users", "me", "assets"}
	}
	path := []string{"api", version, "workspaces", flags["workspace"].(string)}
	switch group {
	case "attachment":
		return append(path, "projects", flags["project"].(string), "work-items", flags["work-item"].(string), "attachments")
	case "workspace-page-attachment":
		if version == "v2" {
			return append(path, "pages", flags["page"].(string), "attachments")
		}
	}
	return append(path, "assets")
}
func transferPartial(cmd *cobra.Command, version, id, phase string, cause error) error {
	e := assetFailure(cmd, "partial_failure", "Asset transfer stopped; no retry or rollback attempted", 6)
	e.APIVersion = version
	e.AssetID = id
	e.RemoteChanged = true
	e.Phase = phase
	e.Recovery = "Inspect the asset before explicitly retrying confirmation or deleting the incomplete asset with --yes."
	if phase == "delivery" {
		e.Recovery = "Upload confirmed. Inspect the asset using its documented get operation; do not repeat the upload or confirmation."
	}
	var remote *httpx.Error
	if errors.As(cause, &remote) {
		e.HTTPStatus = remote.Status
		e.OutcomeUnknown = remote.OutcomeUnknown
		e.RetryAfterSeconds = remote.RetryAfterSeconds
	}
	return e
}
func uploadAsset(cmd *cobra.Command, group string, input map[string]any) error {
	flags := input["flags"].(map[string]any)
	version := flags["api-version"].(string)
	path := assetTransferPath(group, flags)
	local, _ := flags["file"].(string)
	// Nonblocking open prevents a FIFO (including a replacement race) from
	// hanging before the descriptor's regular-file check.
	file, err := os.OpenFile(local, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return assetFailure(cmd, "usage_error", "Cannot open upload file", 2)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return assetFailure(cmd, "usage_error", "Upload source must be a regular file", 2)
	}
	if version == "v2" && info.Size() < 1 {
		return assetFailure(cmd, "usage_error", "Upload file must contain at least one byte", 2)
	}
	name := filepath.Base(local)
	if name == "." || name == ".." || strings.ContainsAny(name, "/\\") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return assetFailure(cmd, "usage_error", "Unsafe upload filename", 2)
	}
	body, err := resourceBody(cmd, input)
	if err != nil {
		return err
	}
	values := map[string]any{}
	if len(body) > 0 {
		decoded, decodeErr := spec.Decode(body)
		var ok bool
		values, ok = decoded.(map[string]any)
		if decodeErr != nil || !ok {
			return assetFailure(cmd, "usage_error", "Upload metadata must be an object", 2)
		}
	}
	values["name"] = name
	values["size"] = info.Size()
	if group == "workspace-page-attachment" && version == "v1" {
		values["entity_type"] = "PAGE_DESCRIPTION"
		values["entity_identifier"] = flags["page"]
	}
	body, err = json.Marshal(values)
	if err != nil {
		return assetFailure(cmd, "usage_error", "Invalid upload metadata", 2)
	}
	dry, _ := cmd.Flags().GetBool("dry-run")
	if dry {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"dry_run": true, "api_version": version, "phases": []string{"metadata", "storage", "confirmation"}, "size": info.Size(), "body_present": true})
	}
	s, err := openAssetSession(cmd)
	if err != nil {
		return err
	}
	response, err := s.api.Do(cmd.Context(), http.MethodPost, path, nil, body, s.header, s.token)
	if err != nil {
		return apiTransportError(cmd, version, err)
	}
	var upload struct {
		AssetID    string `json:"asset_id"`
		UploadData struct {
			URL    string            `json:"url"`
			Fields map[string]string `json:"fields"`
		} `json:"upload_data"`
		Attachment struct {
			Size *float64 `json:"size"`
		} `json:"attachment"`
	}
	if json.Unmarshal(response.Data, &upload) != nil || upload.AssetID == "" || len(upload.UploadData.Fields) == 0 || upload.UploadData.URL == "" {
		return transferPartial(cmd, version, upload.AssetID, "metadata", nil)
	}
	if upload.Attachment.Size != nil && *upload.Attachment.Size < float64(info.Size()) {
		return transferPartial(cmd, version, upload.AssetID, "size_limit", nil)
	}
	if err = s.storage.Upload(cmd.Context(), upload.UploadData.URL, upload.UploadData.Fields, name, file, info.Size()); err != nil {
		return transferPartial(cmd, version, upload.AssetID, "storage", err)
	}
	current, err := file.Stat()
	if err != nil || current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return transferPartial(cmd, version, upload.AssetID, "source_changed", nil)
	}
	confirmPath := append(path, upload.AssetID)
	if group == "workspace-page-attachment" && version == "v1" {
		confirmPath = []string{"api", version, "workspaces", flags["workspace"].(string), "pages", flags["page"].(string), "attachments", upload.AssetID}
	}
	confirmation := json.RawMessage(`{"is_uploaded":true}`)
	if version == "v2" && (group == "user-asset" || group == "workspace-asset") {
		confirmation = nil
	}
	confirmed, err := s.api.Do(cmd.Context(), http.MethodPatch, confirmPath, nil, confirmation, s.header, s.token)
	if err != nil {
		return transferPartial(cmd, version, upload.AssetID, "confirmation", err)
	}
	data := map[string]any{"asset_id": upload.AssetID, "size": info.Size(), "phase": "confirmed", "native": redactAssetData(confirmed.Data)}
	if err = output.Success(cmd.OutOrStdout(), commandName(cmd), data); err != nil {
		return transferPartial(cmd, version, upload.AssetID, "delivery", err)
	}
	return nil
}
func downloadAsset(cmd *cobra.Command, group string, input map[string]any) error {
	flags := input["flags"].(map[string]any)
	version := flags["api-version"].(string)
	id := flags["id"].(string)
	local, _ := flags["output-file"].(string)
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	yes, _ := cmd.Flags().GetBool("yes")
	dry, _ := cmd.Flags().GetBool("dry-run")
	if dry {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"dry_run": true, "api_version": version, "requires_confirmation": overwrite, "phases": []string{"metadata", "download", "installation"}})
	}
	if overwrite && !yes {
		return assetFailure(cmd, "confirmation_required", "Overwriting a destination requires --yes", 2)
	}
	destination, err := prepareAssetDestination(local, overwrite, yes)
	if err != nil {
		return assetFailure(cmd, "output_error", "Cannot prepare download destination; no request sent", 1)
	}
	defer destination.discard()
	s, err := openAssetSession(cmd)
	if err != nil {
		return err
	}
	var path []string
	if group == "workspace-page-attachment" {
		path = []string{"api", version, "workspaces", flags["workspace"].(string), "pages", flags["page"].(string), "attachments", id}
	} else {
		path = append(assetTransferPath(group, flags), id)
	}
	metadata, err := s.api.DoRead(cmd.Context(), http.MethodGet, path, nil, nil, s.header, s.token)
	if err != nil {
		return apiTransportError(cmd, version, err)
	}
	var data struct {
		Size     *json.Number `json:"size"`
		AssetURL string       `json:"asset_url"`
	}
	if json.Unmarshal(metadata.Data, &data) != nil {
		return assetFailure(cmd, "invalid_response", "Invalid attachment metadata", 5)
	}
	expected := int64(-1)
	if data.Size != nil {
		var size float64
		if size, err = data.Size.Float64(); err != nil || size < 0 || size >= float64(int64(^uint64(0)>>1)) || size != float64(int64(size)) {
			return assetFailure(cmd, "invalid_response", "Invalid attachment size", 5)
		}
		expected = int64(size)
	}
	target := data.AssetURL
	if group == "workspace-page-attachment" {
		target, err = s.api.DownloadLocation(cmd.Context(), append(path, "download"), s.header, s.token)
		if err != nil {
			return apiTransportError(cmd, version, err)
		}
	}
	size, err := s.storage.Download(cmd.Context(), target, destination.file, expected)
	if err != nil {
		return apiTransportError(cmd, version, err)
	}
	if err = destination.install(); err != nil {
		return assetFailure(cmd, "output_error", "Could not install downloaded file; destination not replaced", 1)
	}
	if err = output.Success(cmd.OutOrStdout(), commandName(cmd), map[string]any{"asset_id": id, "output_file": local, "size": size}); err != nil {
		return assetFailure(cmd, "output_error", "Download installed but result could not be delivered", 1)
	}
	return nil
}
