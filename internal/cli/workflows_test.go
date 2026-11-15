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
	"regexp"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

const workflowFixtureID = "11111111-1111-4111-8111-111111111111"

type workflowRequestFixture struct {
	ID          string          `json:"id"`
	Command     []string        `json:"command"`
	Method      string          `json:"method"`
	Route       string          `json:"route"`
	Status      int             `json:"status"`
	Response    json.RawMessage `json:"response"`
	Query       map[string]any  `json:"query"`
	Body        json.RawMessage `json:"body"`
	Destructive bool            `json:"destructive"`
	Secret      bool            `json:"secret"`
}

func workflowFixtureArgs(t *testing.T, f workflowRequestFixture, server string) []string {
	t.Helper()
	args := append(append([]string{}, f.Command...), "--api-version", "v2", "--base-url", server, "--workspace", "fixture-workspace")
	for parameter, flag := range map[string]string{"project_id": "project", "workflow_id": "workflow", "transition_id": "transition", "pk": "id", "state_id": "id"} {
		if strings.Contains(f.Route, "{"+parameter+"}") {
			value := workflowFixtureID
			if parameter == "project_id" {
				value = "ENG"
			}
			args = append(args, "--"+flag, value)
		}
	}
	for key, value := range f.Query {
		args = append(args, "--"+strings.ReplaceAll(key, "_", "-")+"="+fmt.Sprint(value))
	}
	if string(f.Body) != "null" && len(f.Body) != 0 {
		args = append(args, "--body-file", collaborationBodyFile(t, f.Body))
	}
	return args
}

func TestP09WorkflowRequests(t *testing.T) {
	raw, err := os.ReadFile("testdata/p09-workflows-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []workflowRequestFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 51 {
		t.Fatalf("expected 51 operation fixtures, got %d", len(fixtures))
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
					wantPath := strings.NewReplacer("{slug}", "fixture-workspace", "{project_id}", "ENG", "{workflow_id}", workflowFixtureID, "{transition_id}", workflowFixtureID, "{pk}", workflowFixtureID, "{state_id}", workflowFixtureID).Replace(f.Route)
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
					args := workflowFixtureArgs(t, f, server.URL)
					if f.Destructive {
						code, out, stderr := invoke(t, catalog, args, nil)
						if code != 2 || calls != 0 || out != "" || !strings.Contains(stderr, "confirmation_required") {
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
						if code != 3 || calls != 1 || out != "" || !strings.Contains(stderr, `"code":"auth_error"`) || !strings.Contains(stderr, `"http_status":403`) {
							t.Fatalf("permission code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
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
						var got, want any
						expected := f.Response
						if !f.Secret {
							expected = regexp.MustCompile(`"(secret|secret_plaintext)"\s*:\s*"[^"]*"`).ReplaceAll(expected, []byte(`"$1":"[REDACTED]"`))
						}
						if json.Unmarshal([]byte(out), &envelope) != nil || json.Unmarshal(envelope.Data, &got) != nil || json.Unmarshal(expected, &want) != nil || !reflect.DeepEqual(got, want) {
							t.Fatalf("native response changed beyond secret redaction: %s", out)
						}
					}
				})
			}
		})
	}
}

func TestP09WorkflowGraphAndNativeFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []string
		body    string
		status  int
		native  string
		exit    int
	}{
		{"catalog-orphans", []string{"workflow", "workspace", "update"}, `{"states":[{"state_id":"` + workflowFixtureID + `","allow_issue_creation":false}],"default_state_id":"` + workflowFixtureID + `","state_mapping":{}}`, 409, "workflow_state_orphans", 5},
		{"catalog-state-dependencies", []string{"workflow", "workspace", "state", "remove"}, `{"new_default_state_id":"` + workflowFixtureID + `","state_mapping":{}}`, 409, "workflow_state_dependencies", 5},
		{"governance-conflict", []string{"workflow", "workspace", "update"}, `{"description":null}`, 409, "states_managed_at_project", 5},
		{"migration-conflict", []string{"workflow", "project", "update"}, `{"is_active":true}`, 409, "governance_migration_in_progress", 5},
		{"unknown-before-governance", []string{"workflow", "workspace", "update"}, `{"name":"Rename"}`, 404, "not_found", 5},
		{"permission-before-governance", []string{"workflow", "workspace", "update"}, `{"name":"Rename"}`, 403, "forbidden", 3},
		{"duplicate-create-pair", []string{"workflow", "workspace", "transition", "create"}, `{"state_id":"` + workflowFixtureID + `","transition_state_id":"` + workflowFixtureID + `"}`, 409, "conflict", 5},
		{"chain-name-conflict", []string{"workflow", "workspace", "update"}, `{"name":"Duplicate","states":[{"state_id":"` + workflowFixtureID + `"}],"default_state_id":"` + workflowFixtureID + `"}`, 500, "server_error", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				got, _ := io.ReadAll(r.Body)
				var actual, want any
				if json.Unmarshal(got, &actual) != nil || json.Unmarshal([]byte(tc.body), &want) != nil || !reflect.DeepEqual(actual, want) {
					t.Errorf("graph body changed: %s", got)
				}
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprintf(w, `{"type":"about:blank","status":%d,"code":%q,"detail":"Fixture rejection","orphans":{"orphans":[],"invalid_targets":[]}}`, tc.status, tc.native)
			}))
			defer server.Close()
			args := append(append([]string{}, tc.command...), "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(tc.body)))
			if tc.command[1] == "project" {
				args = append(args, "--project", "ENG")
			}
			if len(tc.command) == 4 {
				args = append(args, "--workflow", workflowFixtureID)
			}
			if tc.command[len(tc.command)-1] != "create" {
				args = append(args, "--id", workflowFixtureID)
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != tc.exit || calls != 1 || out != "" || !strings.Contains(stderr, fmt.Sprintf(`"http_status":%d`, tc.status)) || strings.Contains(stderr, `"code":"`+tc.native+`"`) || strings.Contains(stderr, "Fixture rejection") {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP09WorkflowStateTypeScopeDifference(t *testing.T) {
	for _, scope := range []string{"project", "workspace"} {
		t.Run(scope, func(t *testing.T) {
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
				_, _ = io.WriteString(w, `{"type":"custom-saved-upstream"}`)
			}))
			defer server.Close()
			args := []string{"workflow", scope, "state", "update", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--workflow", workflowFixtureID, "--id", workflowFixtureID, "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(`{"type":"custom-saved-upstream","allow_issue_creation":false}`))}
			if scope == "project" {
				args = append(args, "--project", "ENG")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if scope == "project" {
				if code != 2 || calls != 0 {
					t.Fatalf("project enum validation code=%d calls=%d err=%s", code, calls, stderr)
				}
			} else if code != 0 || calls != 1 || !strings.Contains(out, "custom-saved-upstream") {
				t.Fatalf("catalog passthrough code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP09WorkflowReadPostAndMutationUncertainty(t *testing.T) {
	for _, preview := range []bool{false, true} {
		t.Run(fmt.Sprint(preview), func(t *testing.T) {
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
				_, _ = io.WriteString(w, `{"incomplete":`)
			}))
			defer server.Close()
			action := "update"
			if preview {
				action = "preview-fallback"
			}
			args := []string{"workflow", "project", action, "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--project", "ENG", "--body-file", collaborationBodyFile(t, json.RawMessage(`{}`))}
			if !preview {
				args = append(args, "--id", workflowFixtureID, "--yes")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			unknown := strings.Contains(stderr, `"outcome_unknown":true`)
			if code != 5 || calls != 1 || out != "" || unknown == preview {
				t.Fatalf("uncertainty code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP09WorkflowScriptHookDoesNotRequireSigningSecret(t *testing.T) {
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
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":"script-hook","handler_name":"run_script","config":{"script_id":"`+workflowFixtureID+`"}}`)
	}))
	defer server.Close()
	args := []string{"workflow", "workspace", "hook", "create", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(`{"phase":"pre","handler_name":"run_script","config":{"script_id":"`+workflowFixtureID+`","execution_variables":{}}}`))}
	code, out, stderr := invoke(t, catalog, args, nil)
	if code != 0 || calls != 1 || !strings.Contains(out, "script-hook") {
		t.Fatalf("script create code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
}

func TestP09WorkflowRelocationAcknowledgementAndRetarget(t *testing.T) {
	for _, tc := range []struct {
		name     string
		command  []string
		body     string
		response string
		status   int
	}{
		{"replace-chain", []string{"workflow", "workspace", "update"}, `{"states":[{"state_id":"` + workflowFixtureID + `"}],"default_state_id":"` + workflowFixtureID + `","state_mapping":{"22222222-2222-4222-8222-222222222222":"` + workflowFixtureID + `"}}`, `{"id":"` + workflowFixtureID + `","states":[{"id":"` + workflowFixtureID + `"}]}`, 200},
		{"remove-state", []string{"workflow", "workspace", "state", "remove"}, `{"new_default_state_id":"` + workflowFixtureID + `","state_mapping":{"22222222-2222-4222-8222-222222222222":"` + workflowFixtureID + `"}}`, "", 204},
		{"retarget-existing-pair", []string{"workflow", "workspace", "transition", "update"}, `{"state_id":{"ignored":true},"transition_state_id":"` + workflowFixtureID + `","member_ids":[],"rejection_state_id":null,"required_approvals":null}`, `{"id":"transition","transition_state_id":"` + workflowFixtureID + `","member_ids":[],"rejection_state_id":null,"required_approvals":null}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				raw, _ := io.ReadAll(r.Body)
				var got, want any
				if json.Unmarshal(raw, &got) != nil || json.Unmarshal([]byte(tc.body), &want) != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("body changed: %s", raw)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.status != 204 {
					_, _ = io.WriteString(w, tc.response)
				}
			}))
			defer server.Close()
			args := append(append([]string{}, tc.command...), "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--id", workflowFixtureID, "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(tc.body)))
			if len(tc.command) == 4 {
				args = append(args, "--workflow", workflowFixtureID)
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || calls != 1 || stderr != "" {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
			var envelope struct {
				Data json.RawMessage `json:"data"`
				Meta struct {
					HTTPStatus int `json:"http_status"`
				} `json:"meta"`
			}
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatal(err)
			}
			wantResponse := tc.response
			if tc.status == 204 {
				wantResponse = "null"
			}
			var got, want any
			if json.Unmarshal(envelope.Data, &got) != nil || json.Unmarshal([]byte(wantResponse), &want) != nil || !reflect.DeepEqual(got, want) || envelope.Meta.HTTPStatus != tc.status {
				t.Fatalf("native response changed: %s", out)
			}
		})
	}
}

func TestP09WorkflowCursorOrderingAndFilters(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("paginate") != "cursor" || r.URL.Query().Get("order_by") != "created_at" || r.URL.Query().Get("phase") != "post" {
			t.Errorf("lost cursor filters: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = io.WriteString(w, `{"data":[{"id":"first"}],"next_cursor":"fixture-next","has_more":true,"pagination":{"style":"cursor"}}`)
		} else {
			if r.URL.Query().Get("cursor") != "fixture-next" {
				t.Errorf("lost cursor: %s", r.URL)
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"second"}],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`)
		}
	}))
	defer server.Close()
	args := []string{"workflow", "workspace", "hook", "list", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--paginate", "cursor", "--order-by", "created_at", "--phase", "post", "--all", "--max-pages", "2"}
	code, out, stderr := invoke(t, catalog, args, nil)
	if code != 0 || calls != 2 || !strings.Contains(out, `"id":"second"`) {
		t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
}

func TestP09WorkflowHookMissingNestedSecret(t *testing.T) {
	for _, response := range []string{`{}`, `{"config":null}`, `{"config":{}}`, `{"config":{"secret_plaintext":null}}`, `{"config":{"secret_plaintext":42}}`, `{"config":{"secret_plaintext":""}}`} {
		for _, file := range []bool{false, true} {
			t.Run(response+fmt.Sprint(file), func(t *testing.T) {
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
					w.WriteHeader(201)
					_, _ = io.WriteString(w, response)
				}))
				defer server.Close()
				target := filepath.Join(t.TempDir(), "secret.json")
				args := []string{"workflow", "workspace", "hook", "create", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(`{"phase":"post","handler_name":"send_webhook","config":{"url":"https://hooks.example.com"}}`))}
				if file {
					args = append(args, "--secret-output-file", target)
				}
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 6 || calls != 1 || out != "" || !strings.Contains(stderr, `"code":"output_error"`) || !strings.Contains(stderr, `"remote_changed":true`) || !strings.Contains(stderr, `"secret_delivered":false`) {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("missing secret installed: %v", err)
				}
			})
		}
	}
}

func TestP09WorkflowHookProtectedReceipt(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	response := `{"id":"hook","config":{"secret_plaintext":"synthetic-one-time-secret","headers":[{"key":"Authorization","value":"synthetic-auth"},{"key":"X-Custom","value":"synthetic-custom","is_secret":true},{"key":"Accept","value":"application/json","is_secret":false}],"nested":{"secret":"synthetic-extra-secret","secret_plaintext":"synthetic-extra-plaintext","precise":9007199254740993}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, response)
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "secret.json")
	args := []string{"workflow", "project", "hook", "create", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--project", "ENG", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--yes", "--secret-output-file", target, "--body-file", collaborationBodyFile(t, json.RawMessage(`{"phase":"pre","handler_name":"send_webhook","config":{"url":"https://hooks.example.com"}}`))}
	code, out, stderr := invoke(t, catalog, args, nil)
	if code != 0 || calls != 1 || stderr != "" || strings.Contains(out, "synthetic-") || !strings.Contains(out, "9007199254740993") {
		t.Fatalf("receipt code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
	stored, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(stored), "synthetic-one-time-secret") || !strings.Contains(string(stored), "9007199254740993") {
		t.Fatalf("protected result missing or imprecise: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("unprotected result: %v", err)
	}
	code, out, stderr = invoke(t, catalog, args, nil)
	if code != 1 || calls != 1 || out != "" {
		t.Fatalf("overwrite preflight code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
	code, out, stderr = invoke(t, catalog, []string{"workflow", "project", "hook", "get", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--project", "ENG", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--id", workflowFixtureID}, nil)
	if code != 0 || strings.Contains(out, "synthetic-") || !strings.Contains(out, "application/json") {
		t.Fatalf("ordinary headers code=%d out=%s err=%s", code, out, stderr)
	}
}

func TestP09WorkflowScriptHookRejectsSecretDeliveryFlags(t *testing.T) {
	for _, flags := range [][]string{{"--secret-output-file", "unused.json"}, {"--overwrite=false"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
			defer server.Close()
			args := []string{"workflow", "workspace", "hook", "create", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(`{"phase":"pre","handler_name":"run_script","config":{"script_id":"`+workflowFixtureID+`"}}`))}
			args = append(args, flags...)
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 2 || calls != 0 || out != "" || !strings.Contains(stderr, `"code":"usage_error"`) {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP09WorkflowHookUnsafeDeliveryFlags(t *testing.T) {
	for _, scope := range []string{"project", "workspace"} {
		for _, flags := range [][]string{{"--fields", "id"}, {"--fields", "config", "--dry-run"}, {"--overwrite", "--dry-run"}} {
			t.Run(scope+strings.Join(flags, " "), func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
				defer server.Close()
				args := []string{"workflow", scope, "hook", "create", "--api-version", "v2", "--base-url", server.URL, "--workspace", "fixture", "--workflow", workflowFixtureID, "--transition", workflowFixtureID, "--yes", "--body-file", collaborationBodyFile(t, json.RawMessage(`{"phase":"pre","handler_name":"send_webhook","config":{"url":"https://hooks.example.com"}}`))}
				if scope == "project" {
					args = append(args, "--project", "ENG")
				}
				args = append(args, flags...)
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 2 || calls != 0 || out != "" || !strings.Contains(stderr, `"code":"usage_error"`) {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
			})
		}
	}
}
