package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestP08IntegrationRequests(t *testing.T) {
	raw, err := os.ReadFile("testdata/p08-integrations-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		ID          string          `json:"id"`
		Command     []string        `json:"command"`
		Method      string          `json:"method"`
		Route       string          `json:"route"`
		Status      int             `json:"status"`
		Response    json.RawMessage `json:"response"`
		Query       map[string]any  `json:"query"`
		Body        json.RawMessage `json:"body"`
		Destructive bool            `json:"destructive"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.ID, func(t *testing.T) {
			for _, rejected := range []bool{false, true} {
				t.Run(fmt.Sprint(rejected), func(t *testing.T) {
					transportEnv(t)
					t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
					id := "11111111-1111-4111-8111-111111111111"
					wantPath := strings.NewReplacer("{slug}", "fixture-workspace", "{pk}", id, "{artifact_id}", id, "{webhook_id}", id).Replace(f.Route)
					wantQuery := url.Values{}
					for key, value := range f.Query {
						wantQuery.Set(key, fmt.Sprint(value))
					}
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						if r.Method != f.Method || r.URL.Path != wantPath || !reflect.DeepEqual(r.URL.Query(), wantQuery) {
							t.Errorf("request %s %s; want %s %s?%s", r.Method, r.URL, f.Method, wantPath, wantQuery.Encode())
						}
						if r.Header.Get("X-API-Key") != "fixture-key" {
							t.Error("missing authentication")
						}
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						if string(f.Body) == "null" {
							if len(body) != 0 {
								t.Errorf("unexpected body %s", body)
							}
						} else {
							var got, want any
							if json.Unmarshal(body, &got) != nil || json.Unmarshal(f.Body, &want) != nil || !reflect.DeepEqual(got, want) {
								t.Errorf("body %s; want %s", body, f.Body)
							}
						}
						w.Header().Set("Content-Type", "application/json")
						if rejected {
							w.WriteHeader(403)
							_, _ = io.WriteString(w, `{"type":"about:blank","status":403,"code":"forbidden","detail":"Insufficient scope"}`)
							return
						}
						w.WriteHeader(f.Status)
						if f.Status != 204 {
							_, _ = w.Write(f.Response)
						}
					}))
					defer server.Close()
					args := append(append([]string{}, f.Command...), "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture-workspace")
					if strings.Contains(f.Route, "{pk}") || strings.Contains(f.Route, "{artifact_id}") {
						args = append(args, "--id", id)
					}
					if strings.Contains(f.Route, "{webhook_id}") {
						args = append(args, "--webhook", id)
					}
					for key, value := range f.Query {
						args = append(args, "--"+strings.ReplaceAll(key, "_", "-")+"="+fmt.Sprint(value))
					}
					if string(f.Body) != "null" {
						args = append(args, "--body-file", collaborationBodyFile(t, f.Body))
					}
					if f.Destructive {
						code, out, stderr := invoke(t, catalog, args, nil)
						if code != 2 || calls != 0 || out != "" {
							t.Fatalf("confirmation code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
						code, out, stderr = invoke(t, catalog, append(append([]string{}, args...), "--dry-run"), nil)
						if code != 0 || calls != 0 || !strings.Contains(out, `"requires_confirmation":true`) {
							t.Fatalf("dry-run code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
						args = append(args, "--yes")
					}
					code, out, stderr := invoke(t, catalog, args, nil)
					if rejected {
						if code != 3 || calls != 1 || out != "" {
							t.Fatalf("rejection code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
						return
					}
					if code != 0 || calls != 1 || stderr != "" {
						t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
					}
					if f.Status != 204 {
						var envelope struct {
							Data json.RawMessage `json:"data"`
						}
						expected := f.Response
						if strings.HasPrefix(f.ID, "v2.webhook-logs.") {
							expected = json.RawMessage(strings.NewReplacer(`"request_headers": "example"`, `"request_headers": "[REDACTED]"`, `"response_headers": "example"`, `"response_headers": "[REDACTED]"`, `"request_headers":"example"`, `"request_headers":"[REDACTED]"`, `"response_headers":"example"`, `"response_headers":"[REDACTED]"`).Replace(string(expected)))
						}
						var got, want any
						if json.Unmarshal([]byte(out), &envelope) != nil || json.Unmarshal(envelope.Data, &got) != nil || json.Unmarshal(expected, &want) != nil || !reflect.DeepEqual(got, want) {
							t.Fatalf("native response changed: %s", out)
						}
					}
				})
			}
		})
	}
}

func TestP08WebhookProtectedSecret(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"hook","secret_key":"one-time-fixture-secret"}`)
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "secret.json")
	args := []string{"webhook", "regenerate-secret", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--id", "11111111-1111-4111-8111-111111111111", "--yes", "--secret-output-file", target}
	code, out, stderr := invoke(t, catalog, args, nil)
	if code != 0 || calls != 1 || strings.Contains(out+stderr, "one-time-fixture-secret") {
		t.Fatalf("delivery code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
	stored, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(stored), "one-time-fixture-secret") {
		t.Fatalf("protected secret missing: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file is not owner-only: %v", err)
	}
	code, out, stderr = invoke(t, catalog, args, nil)
	if code != 1 || calls != 1 || out != "" {
		t.Fatalf("overwrite preflight code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
}
