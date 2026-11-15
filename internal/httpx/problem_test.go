package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSafeProblemCodeAllowlist(t *testing.T) {
	groups := []struct {
		status   int
		category string
		codes    string
	}{
		{400, "invalid_request", "invalid_request count_pagination_disabled ordering_not_cursor_eligible workflow_outside_allowlist"},
		{401, "unauthorized", "unauthorized"},
		{402, "payment_required", "payment_required"},
		{403, "forbidden", "forbidden workflow_transition_denied workflow_mandated workflow_assigned_by_workspace"},
		{404, "not_found", "not_found"},
		{405, "method_not_allowed", "method_not_allowed"},
		{406, "not_acceptable", "not_acceptable"},
		{409, "conflict", "conflict states_managed_at_workspace states_managed_at_project templates_managed_at_workspace work_item_types_managed_at_workspace work_item_types_managed_at_project governance_migration_in_progress workflow_in_use workflow_state_orphans workflow_state_dependencies workflow_allowlist_excludes_in_use workflow_pins_unavailable"},
		{413, "payload_too_large", "payload_too_large"},
		{415, "unsupported_media_type", "unsupported_media_type"},
		{429, "rate_limited", "rate_limited"},
		{500, "server_error", "server_error listing_authorization_misconfigured"},
		{502, "service_unavailable", "service_unavailable"},
		{503, "service_unavailable", "service_unavailable"},
		{504, "service_unavailable", "service_unavailable"},
	}
	for _, group := range groups {
		for _, code := range strings.Fields(group.codes) {
			t.Run(code+"/"+http.StatusText(group.status), func(t *testing.T) {
				raw, err := json.Marshal(map[string]any{"type": group.category, "code": code, "detail": "canary-secret", "errors": map[string]any{"secret": "canary-secret"}})
				if err != nil {
					t.Fatal(err)
				}
				response := &http.Response{StatusCode: group.status, Header: http.Header{"Content-Type": {"application/problem+json; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(string(raw)))}
				if got := safeProblemCode(response); got != code {
					t.Fatalf("code = %q, want %q", got, code)
				}
			})
		}
	}
}

func TestSafeProblemCodeRejectsUntrustedBodies(t *testing.T) {
	for _, tc := range []struct {
		name, body, media string
		status            int
	}{
		{"unknown-code", `{"type":"conflict","code":"canary-secret"}`, "application/problem+json", 409},
		{"unknown-type", `{"type":"canary-secret","code":"conflict"}`, "application/problem+json", 409},
		{"wrong-type", `{"type":"forbidden","code":"workflow_state_orphans"}`, "application/problem+json", 409},
		{"wrong-status", `{"type":"conflict","code":"workflow_state_orphans"}`, "application/problem+json", 403},
		{"missing-type", `{"code":"workflow_state_orphans"}`, "application/problem+json", 409},
		{"wrong-code-type", `{"type":"conflict","code":["conflict"]}`, "application/problem+json", 409},
		{"duplicate-key", `{"type":"conflict","code":"conflict","code":"workflow_state_orphans"}`, "application/problem+json", 409},
		{"trailing-json", `{"type":"conflict","code":"conflict"}{}`, "application/problem+json", 409},
		{"non-object", `[]`, "application/problem+json", 409},
		{"malformed", `{"type":`, "application/problem+json", 409},
		{"invalid-utf8", "{\"type\":\"conflict\",\"code\":\"conflict\",\"detail\":\"\xff\"}", "application/problem+json", 409},
		{"oversized", `{"type":"conflict","code":"conflict","detail":"` + strings.Repeat("x", 64<<10) + `"}`, "application/problem+json", 409},
		{"json-not-problem", `{"type":"conflict","code":"conflict"}`, "application/json", 409},
		{"html", `<html>canary-secret</html>`, "text/html", 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {tc.media}}, Body: io.NopCloser(strings.NewReader(tc.body))}
			if got := safeProblemCode(response); got != "" {
				t.Fatalf("unexpected code = %q", got)
			}
		})
	}
}

func TestProblemCodeTransportPreservesErrorPolicy(t *testing.T) {
	for _, tc := range []struct {
		version, code, category string
		status, exit            int
		want                    string
	}{
		{"v2", "workflow_state_orphans", "conflict", 409, 5, "workflow_state_orphans"},
		{"v2", "workflow_mandated", "forbidden", 403, 3, "workflow_mandated"},
		{"v2", "unauthorized", "unauthorized", 401, 3, "unauthorized"},
		{"v2", "payment_required", "payment_required", 402, 5, "payment_required"},
		{"v2", "not_found", "not_found", 404, 5, "not_found"},
		{"v2", "rate_limited", "rate_limited", 429, 5, "rate_limited"},
		{"v2", "listing_authorization_misconfigured", "server_error", 500, 5, "listing_authorization_misconfigured"},
		{"v2", "canary-secret", "conflict", 409, 5, ""},
		{"v1", "conflict", "conflict", 409, 5, ""},
	} {
		t.Run(tc.version+"/"+tc.code, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": tc.category, "code": tc.code, "detail": "canary-secret", "errors": []any{"canary-secret"}})
			}))
			defer server.Close()
			client := clientFor(t, server.URL)
			_, err := client.DoMutation(context.Background(), "PATCH", []string{"api", tc.version, "workspaces", "example"}, nil, json.RawMessage(`{}`), "", "")
			var remote *Error
			if !errors.As(err, &remote) || remote.Exit != tc.exit || remote.Status != tc.status || remote.UpstreamCode != tc.want || remote.OutcomeUnknown != (tc.status >= 500) {
				t.Fatalf("unexpected error: %#v", err)
			}
			wantCode := "api_error"
			if tc.exit == 3 {
				wantCode = "auth_error"
			}
			if remote.Code != wantCode || remote.RetryAfterSeconds == nil || *remote.RetryAfterSeconds != 7 || strings.Contains(remote.Message, "canary") || calls.Load() != 1 {
				t.Fatalf("error policy changed: %#v; calls=%d", remote, calls.Load())
			}
		})
	}
}
