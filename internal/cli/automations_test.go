package cli

import (
	"context"
	"encoding/json"
	"errors"
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

type p09AutomationFixture struct {
	ID       string          `json:"id"`
	Args     []string        `json:"args"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Query    map[string]any  `json:"query"`
	Body     json.RawMessage `json:"body"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"`
}

func p09AutomationFixtures(t *testing.T) []p09AutomationFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/p09-automations-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []p09AutomationFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func p09AutomationCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func p09AutomationArgs(t *testing.T, fixture p09AutomationFixture, base string) []string {
	t.Helper()
	args := append(append([]string{}, fixture.Args...), "--base-url", base)
	if string(fixture.Body) != "null" {
		args = append(args, "--body-file", workItemBodyFile(t, string(fixture.Body)))
	}
	return args
}

// All 38 captured operations have independent scope, path, method and native
// response assertions. Draft creation never issues a second status request.
func TestP09AutomationRequests(t *testing.T) {
	fixtures := p09AutomationFixtures(t)
	if len(fixtures) != 38 {
		t.Fatalf("operation fixture count = %d; want 38", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			wantQuery := url.Values{}
			for key, value := range fixture.Query {
				wantQuery.Set(key, fmt.Sprint(value))
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != fixture.Method || r.URL.EscapedPath() != "/prefix"+fixture.Path || !reflect.DeepEqual(r.URL.Query(), wantQuery) {
					t.Errorf("request = %s %s; want %s /prefix%s?%s", r.Method, r.URL, fixture.Method, fixture.Path, wantQuery.Encode())
				}
				if r.Header.Get("X-Api-Key") != "fixture-token" {
					t.Error("missing selected credential")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(fixture.Body) == "null" {
					if len(body) != 0 {
						t.Errorf("unexpected request body: %s", body)
					}
				} else if !reflect.DeepEqual(workItemJSON(t, string(body)), workItemJSON(t, string(fixture.Body))) {
					t.Errorf("body = %s; want %s", body, fixture.Body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(fixture.Status)
				if fixture.Status != http.StatusNoContent {
					_, _ = w.Write(fixture.Response)
				}
			}))
			defer server.Close()
			code, stdout, stderr := invoke(t, p09AutomationCatalog(t), p09AutomationArgs(t, fixture, server.URL+"/prefix"), automationHandlers())
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			result := workItemJSON(t, stdout).(map[string]any)
			if !reflect.DeepEqual(result["data"], workItemJSON(t, string(fixture.Response))) {
				t.Fatalf("native response changed: %s; want %s", stdout, fixture.Response)
			}
			meta := result["meta"].(map[string]any)
			if meta["api_version"] != "v2" || meta["http_status"] != json.Number(fmt.Sprint(fixture.Status)) || meta["status"] != "completed" {
				t.Fatalf("native response metadata changed: %s", stdout)
			}
		})
	}
}

func TestP09AutomationConfirmationAndDryRun(t *testing.T) {
	for _, fixture := range p09AutomationFixtures(t) {
		if fixture.Method == http.MethodGet {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			transportEnv(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(500)
			}))
			defer server.Close()
			args := []string{}
			for _, arg := range p09AutomationArgs(t, fixture, server.URL) {
				if arg != "--yes" {
					args = append(args, arg)
				}
			}
			code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
			if code != 2 || calls != 0 || stdout != "" || !strings.Contains(stderr, "confirmation_required") {
				t.Fatalf("confirmation: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			code, stdout, stderr = invoke(t, p09AutomationCatalog(t), append(args, "--dry-run"), automationHandlers())
			if code != 0 || calls != 0 || stderr != "" || !strings.Contains(stdout, `"requires_confirmation":true`) {
				t.Fatalf("dry-run: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestP09AutomationNativeFailures(t *testing.T) {
	for _, scope := range []string{"project", "workspace"} {
		for _, failure := range []struct {
			status, exit int
			code         string
		}{
			{400, 5, "invalid_request"},
			{402, 5, "payment_required"},
			{403, 3, "forbidden"},
			{404, 5, "not_found"},
			{409, 5, "conflict"},
			{429, 5, "rate_limited"},
		} {
			t.Run(scope+"/"+failure.code, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/problem+json")
					w.Header().Set("Retry-After", "7")
					w.WriteHeader(failure.status)
					_, _ = fmt.Fprintf(w, `{"status":%d,"code":%q,"detail":"reflected-secret-canary","title":"reflected-secret-canary"}`, failure.status, failure.code)
				}))
				defer server.Close()
				args := []string{"automation", scope, "edge", "create", "--api-version", "v2", "--workspace", "Fixture-Team", "--automation", "9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88", "--yes", "--body-file", workItemBodyFile(t, `{"source_node_id":"f960d3c2-8524-4a41-b8eb-055ce4be2a7f","target_node_id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"}`), "--base-url", server.URL}
				if scope == "project" {
					args = append(args, "--project", "ENG")
				}
				code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
				if code != failure.exit || stdout != "" || calls != 1 || !strings.Contains(stderr, fmt.Sprintf(`"http_status":%d`, failure.status)) || strings.Contains(stderr, "reflected-secret-canary") {
					t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
				}
			})
		}
	}
}

// Approved interpretation: explicit input declarations beat conflicting examples;
// the response fixtures independently preserve null config/node_type unchanged.
func TestP09AutomationInputContracts(t *testing.T) {
	for _, scope := range []string{"project", "workspace"} {
		for _, fixture := range []struct {
			resource, action, body string
		}{
			{"", "create", `{"name":"Example","scope":"other"}`},
			{"", "create", `{"name":"Example","scope":"work-item","project_ids":[["f960d3c2-8524-4a41-b8eb-055ce4be2a7f"]]}`},
			{"node", "create", `{"name":"Example","handler_name":"record_created","node_type":"trigger","config":null}`},
			{"node", "create", `{"name":"Example","handler_name":"record_created","node_type":"unknown"}`},
			{"node", "create", `{"name":"Example","handler_name":"record_created"}`},
			{"node", "create", `{"name":"Example","handler_name":"record_created","node_type":"trigger","config":{}}`},
			{"edge", "create", `{"source_node_id":"f960d3c2-8524-4a41-b8eb-055ce4be2a7f"}`},
			{"", "set-status", `{}`},
			{"", "set-status", `{"is_enabled":null}`},
		} {
			t.Run(scope+"/"+fixture.resource+"/"+fixture.action+"/"+fixture.body, func(t *testing.T) {
				transportEnv(t)
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(500)
				}))
				defer server.Close()
				args := []string{"automation", scope}
				if fixture.resource != "" {
					args = append(args, fixture.resource)
				}
				args = append(args, fixture.action, "--api-version", "v2", "--workspace", "Fixture-Team", "--yes", "--body-file", workItemBodyFile(t, fixture.body), "--base-url", server.URL)
				if scope == "project" {
					args = append(args, "--project", "ENG")
				}
				if fixture.resource != "" {
					args = append(args, "--automation", "9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88")
				}
				if fixture.action == "set-status" {
					args = append(args, "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53")
				}
				code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
				if code != 2 || calls != 0 || stdout != "" || !strings.Contains(stderr, "usage_error") {
					t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
				}
			})
		}
	}
}

func TestP09AutomationVersionAvailability(t *testing.T) {
	transportEnv(t)
	code, stdout, stderr := invoke(t, p09AutomationCatalog(t), []string{"automation", "workspace", "list", "--workspace", "Fixture-Team", "--api-version", "v1"}, automationHandlers())
	if code != 7 || stdout != "" || !strings.Contains(stderr, "operation_unavailable") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestP09AutomationPagination(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/v2/workspaces/Fixture-Team/automations/" || r.URL.Query().Get("count") != "false" || r.URL.Query().Get("fields") != "id" {
			t.Errorf("unexpected pagination request %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = io.WriteString(w, `{"data":[],"next":2,"previous":null,"pagination":{"style":"offset"}}`)
			return
		}
		if r.URL.Query().Get("offset") != "2" {
			t.Errorf("missing native next offset: %s", r.URL)
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"opaque","run_count":0}],"next":null,"previous":0,"pagination":{"style":"offset"}}`)
	}))
	defer server.Close()
	args := []string{"automation", "workspace", "list", "--api-version", "v2", "--workspace", "Fixture-Team", "--base-url", server.URL, "--count=false", "--fields", "id", "--all"}
	code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
	if code != 0 || calls != 2 || stderr != "" || !strings.Contains(stdout, `"pages":2`) || !strings.Contains(stdout, `"complete":true`) || !strings.Contains(stdout, `"run_count":0`) {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}

func TestP09AutomationMalformedMutationOutcome(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":`)
	}))
	defer server.Close()
	args := []string{"automation", "workspace", "create", "--api-version", "v2", "--workspace", "Fixture-Team", "--base-url", server.URL, "--yes", "--name", "Example", "--scope", "work-item"}
	code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
	if code != 5 || calls != 1 || stdout != "" || !strings.Contains(stderr, `"outcome_unknown":true`) || strings.Contains(stderr, "completed") {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}

func TestP09AutomationProtectedSecret(t *testing.T) {
	for _, scope := range []string{"project", "workspace"} {
		t.Run(scope, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"secret":"synthetic-one-time-automation-secret"}`)
			}))
			defer server.Close()
			target := filepath.Join(t.TempDir(), "node-secret.json")
			args := []string{"automation", scope, "node", "regenerate-webhook-secret", "--api-version", "v2", "--workspace", "Fixture-Team", "--automation", "9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88", "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53", "--yes", "--base-url", server.URL, "--secret-output-file", target}
			if scope == "project" {
				args = append(args, "--project", "ENG")
			}
			code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
			if code != 0 || calls != 1 || strings.Contains(stdout+stderr, "synthetic-one-time-automation-secret") {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			stored, err := os.ReadFile(target)
			if err != nil || !strings.Contains(string(stored), "synthetic-one-time-automation-secret") {
				t.Fatalf("protected secret missing: %v", err)
			}
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("file is not owner-only: %v", err)
			}
			code, stdout, stderr = invoke(t, p09AutomationCatalog(t), args, automationHandlers())
			if code != 1 || calls != 1 || stdout != "" {
				t.Fatalf("preflight code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

type p09AutomationFailedWriter struct{}

func (p09AutomationFailedWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic output failure")
}

func TestP09AutomationSecretDeliveryFailure(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"secret":"synthetic-lost-automation-secret"}`)
	}))
	defer server.Close()
	var stderr strings.Builder
	args := []string{"automation", "workspace", "node", "regenerate-webhook-secret", "--api-version", "v2", "--workspace", "Fixture-Team", "--automation", "9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88", "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53", "--yes", "--base-url", server.URL}
	code := run(context.Background(), args, Streams{In: strings.NewReader(""), Out: p09AutomationFailedWriter{}, Err: &stderr}, Build{Version: "test"}, p09AutomationCatalog(t), automationHandlers())
	if code != 6 || calls != 1 || !strings.Contains(stderr.String(), `"remote_changed":true`) || !strings.Contains(stderr.String(), `"secret_delivered":false`) || strings.Contains(stderr.String(), "synthetic-lost-automation-secret") {
		t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr.String())
	}
}

func TestP09AutomationOrdinarySecretsRedacted(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"opaque","config":null,"node_type":null,"secret":"ordinary-secret-canary","extension":{"secret_plaintext":"nested-secret-canary","safe":false}}`)
	}))
	defer server.Close()
	args := []string{"automation", "workspace", "node", "get", "--api-version", "v2", "--workspace", "Fixture-Team", "--automation", "9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88", "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53", "--base-url", server.URL}
	code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
	if code != 0 || calls != 1 || stderr != "" || strings.Contains(stdout, "ordinary-secret-canary") || strings.Contains(stdout, "nested-secret-canary") || !strings.Contains(stdout, `"config":null`) || !strings.Contains(stdout, `"node_type":null`) || !strings.Contains(stdout, `"safe":false`) {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}

func TestP09AutomationExplicitFalseField(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/status/") || string(body) != `{"is_enabled":false}` {
			t.Errorf("request = %s %s body=%s", r.Method, r.URL, body)
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	args := []string{"automation", "workspace", "set-status", "--api-version", "v2", "--workspace", "Fixture-Team", "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53", "--base-url", server.URL, "--is-enabled=false", "--yes"}
	code, stdout, stderr := invoke(t, p09AutomationCatalog(t), args, automationHandlers())
	if code != 0 || calls != 1 || stderr != "" || !strings.Contains(stdout, `"data":null`) || !strings.Contains(stdout, `"http_status":204`) {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}
