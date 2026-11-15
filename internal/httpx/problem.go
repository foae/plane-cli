package httpx

import (
	"io"
	"mime"
	"net/http"

	"github.com/foae/plane-cli/internal/spec"
)

// Error diagnostics are optional: malformed, oversized or unknown problems retain
// the status-based error. Never retain server prose or arbitrary error members.
func safeProblemCode(response *http.Response) string {
	const maxProblemBytes = 64 << 10
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || media != "application/problem+json" {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxProblemBytes+1))
	if err != nil || len(raw) > maxProblemBytes {
		return ""
	}
	value, err := spec.DecodeWithLimit(raw, maxProblemBytes)
	if err != nil {
		return ""
	}
	problem, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	code, _ := problem["code"].(string)
	category, _ := problem["type"].(string)
	if code == "service_unavailable" && category == code && (response.StatusCode == 502 || response.StatusCode == 504) {
		return code
	}
	if category == "" || problemStatus(code, category) != response.StatusCode {
		return ""
	}
	return code
}

// P00 errors and type-mode guides define these exact code/category/status pairs.
// Unknown codes stay private; do not infer diagnostic safety from their spelling.
func problemStatus(code, category string) int {
	switch code {
	case "count_pagination_disabled", "ordering_not_cursor_eligible", "workflow_outside_allowlist":
		if category == "invalid_request" {
			return 400
		}
	case "workflow_transition_denied", "workflow_mandated", "workflow_assigned_by_workspace":
		if category == "forbidden" {
			return 403
		}
	case "states_managed_at_workspace", "states_managed_at_project", "templates_managed_at_workspace",
		"work_item_types_managed_at_workspace", "work_item_types_managed_at_project", "governance_migration_in_progress",
		"workflow_in_use", "workflow_state_orphans", "workflow_state_dependencies", "workflow_allowlist_excludes_in_use", "workflow_pins_unavailable":
		if category == "conflict" {
			return 409
		}
	case "listing_authorization_misconfigured":
		if category == "server_error" {
			return 500
		}
	}
	if code != category {
		return 0
	}
	switch code {
	case "invalid_request":
		return 400
	case "unauthorized":
		return 401
	case "payment_required":
		return 402
	case "forbidden":
		return 403
	case "not_found":
		return 404
	case "method_not_allowed":
		return 405
	case "not_acceptable":
		return 406
	case "conflict":
		return 409
	case "payload_too_large":
		return 413
	case "unsupported_media_type":
		return 415
	case "rate_limited":
		return 429
	case "server_error":
		return 500
	case "service_unavailable":
		return 503
	}
	return 0
}
