package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

type p08PermissionFixture struct {
	Name     string          `json:"name"`
	Args     []string        `json:"args"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Query    string          `json:"query"`
	Body     json.RawMessage `json:"body"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"`
}

func p08PermissionFixtures(t *testing.T) []p08PermissionFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/p08-permissions-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []p08PermissionFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func p08PermissionCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func p08PermissionHandlers() map[string]handler {
	handlers := adminPermissionsHandlers()
	for name, h := range typeGovernanceHandlers() {
		handlers[name] = h
	}
	return handlers
}

func p08PermissionArgs(t *testing.T, fixture p08PermissionFixture, base string) []string {
	t.Helper()
	args := append(append([]string{}, fixture.Args...), "--base-url", base)
	if string(fixture.Body) != "null" {
		args = append(args, "--body-file", workItemBodyFile(t, string(fixture.Body)))
	}
	return args
}

// All 23 native operations, including the pre-existing workspace feature read,
// have independently pinned requests and preserve false, null, arrays and slugs.
func TestP08PermissionRequests(t *testing.T) {
	fixtures := p08PermissionFixtures(t)
	if len(fixtures) != 23 {
		t.Fatalf("operation fixture count = %d; want 23", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != fixture.Method || r.URL.EscapedPath() != "/prefix"+fixture.Path || r.URL.RawQuery != fixture.Query {
					t.Errorf("request = %s %s?%s; want %s %s?%s", r.Method, r.URL.EscapedPath(), r.URL.RawQuery, fixture.Method, "/prefix"+fixture.Path, fixture.Query)
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
			code, stdout, stderr := invoke(t, p08PermissionCatalog(t), p08PermissionArgs(t, fixture, server.URL+"/prefix"), p08PermissionHandlers())
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			result := workItemJSON(t, stdout).(map[string]any)
			if !reflect.DeepEqual(result["data"], workItemJSON(t, string(fixture.Response))) {
				t.Fatalf("native response changed: %s; want %s", stdout, fixture.Response)
			}
			meta := result["meta"].(map[string]any)
			version := "v2"
			if strings.HasPrefix(fixture.Path, "/api/v1/") {
				version = "v1"
			}
			if meta["api_version"] != version || meta["http_status"] != json.Number(fmt.Sprint(fixture.Status)) {
				t.Fatalf("native response metadata: %s", stdout)
			}
		})
	}
}

func TestP08PermissionConfirmationAndDryRun(t *testing.T) {
	for _, fixture := range p08PermissionFixtures(t) {
		if fixture.Method == "GET" {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			transportEnv(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(500)
			}))
			defer server.Close()
			args := []string{}
			for _, arg := range p08PermissionArgs(t, fixture, server.URL) {
				if arg != "--yes" {
					args = append(args, arg)
				}
			}
			code, stdout, stderr := invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
			if code != 2 || calls != 0 || stdout != "" || !strings.Contains(stderr, "confirmation_required") {
				t.Fatalf("confirmation: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			code, stdout, stderr = invoke(t, p08PermissionCatalog(t), append(args, "--dry-run"), p08PermissionHandlers())
			if code != 0 || calls != 0 || stderr != "" || !strings.Contains(stdout, `"requires_confirmation":true`) {
				t.Fatalf("dry-run: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestP08PermissionNativeFailures(t *testing.T) {
	for _, failure := range []struct {
		name, upstream string
		status, exit   int
	}{
		{"insufficient-role", "forbidden", 403, 3},
		{"missing-or-hidden", "not_found", 404, 5},
		{"entitlement", "payment_required", 402, 5},
		{"migration", "governance_migration_in_progress", 409, 5},
		{"sparse-prose-error", "invalid_request", 400, 5},
	} {
		t.Run(failure.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(failure.status)
				_, _ = fmt.Fprintf(w, `{"type":"invalid_request","code":%q,"detail":"[REDACTED]"}`, failure.upstream)
			}))
			defer server.Close()
			args := []string{"permission-grant", "update", "--api-version", "v2", "--workspace", "Team", "--id", "d5c74e56-a54c-45eb-b121-e376ac185689", "--body-file", workItemBodyFile(t, `{}`), "--yes", "--base-url", server.URL}
			if failure.status == 402 {
				args = []string{"workspace-feature", "get", "--api-version", "v2", "--workspace", "Team", "--base-url", server.URL}
			}
			code, stdout, stderr := invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
			if code != failure.exit || stdout != "" || calls != 1 || !strings.Contains(stderr, fmt.Sprintf(`"http_status":%d`, failure.status)) {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestP08PermissionInputContracts(t *testing.T) {
	for _, fixture := range []struct {
		name string
		args []string
		body string
	}{
		{"missing-resource", []string{"permission", "check", "--action", "edit", "--resource-type", "workitem"}, ""},
		{"invalid-resource-uuid", []string{"permission", "relation", "--resource-type", "project", "--resource-id", "not-a-uuid"}, ""},
		{"project-get-no-sparse-contract", []string{"project-feature", "get", "--project", "ENG", "--fields", "all"}, ""},
		{"delete-no-sparse-contract", []string{"permission-grant", "delete", "--id", "d5c74e56-a54c-45eb-b121-e376ac185689", "--fields", "all"}, ""},
		{"effective-no-expand", []string{"permission", "workspace-effective", "--expand", "all"}, ""},
		{"feature-wrong-boolean", []string{"workspace-feature", "update"}, `{"is_wiki_enabled":"false"}`},
		{"feature-wrong-integer", []string{"workspace-feature", "update"}, `{"work_item_type_default_level":1.5}`},
		{"feature-no-null", []string{"project-feature", "update", "--project", "ENG"}, `{"is_epic_enabled":null}`},
		{"grant-invalid-permission-array", []string{"permission-grant", "update", "--id", "d5c74e56-a54c-45eb-b121-e376ac185689"}, `{"permissions_grant":[30]}`},
		{"grant-numeric-role", []string{"permission-grant", "update", "--id", "d5c74e56-a54c-45eb-b121-e376ac185689"}, `{"relation":15}`},
		{"grant-required-create-fields", []string{"permission-grant", "create"}, `{"relation":"guest"}`},
		{"grant-relation-length", []string{"permission-grant", "update", "--id", "d5c74e56-a54c-45eb-b121-e376ac185689"}, `{"relation":"` + strings.Repeat("x", 51) + `"}`},
		{"invalid-namespace", []string{"role", "list", "--namespace", "guessed"}, ""},
		{"invalid-max-pages", []string{"role", "list", "--all", "--max-pages", "0"}, ""},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			transportEnv(t)
			args := append(append([]string{}, fixture.args...), "--api-version", "v2", "--workspace", "Team", "--base-url", "http://127.0.0.1:1")
			if fixture.body != "" {
				args = append(args, "--body-file", workItemBodyFile(t, fixture.body))
			}
			code, stdout, stderr := invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
			if code != 2 || stdout != "" || strings.Contains(stderr, "authentication_required") {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
		})
	}
}

func TestP08PermissionVersionSpecificFlags(t *testing.T) {
	transportEnv(t)
	for _, args := range [][]string{
		{"workspace-feature", "update", "--api-version", "v1", "--is-wiki-enabled=false"},
		{"workspace-feature", "update", "--api-version", "v2", "--wiki=false"},
		{"project-feature", "update", "--api-version", "v1", "--is-epic-enabled=false", "--project", "ENG"},
		{"project-feature", "update", "--api-version", "v2", "--epics=false", "--project", "ENG"},
	} {
		args = append(args, "--workspace", "Team", "--dry-run", "--base-url", "http://127.0.0.1:1")
		code, stdout, stderr := invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
		if code != 2 || stdout != "" || strings.Contains(stderr, "authentication_required") {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", args, code, stdout, stderr)
		}
	}
	code, stdout, stderr := invoke(t, p08PermissionCatalog(t), []string{"role", "list", "--api-version", "v1", "--workspace", "Team"}, p08PermissionHandlers())
	if code != 7 || stdout != "" || strings.Contains(stderr, "authentication_required") {
		t.Fatalf("unavailable: code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestP08PermissionFeatureFieldFlags(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.Method != "PATCH" || r.URL.Path != "/api/v2/workspaces/Team/features/" || !reflect.DeepEqual(workItemJSON(t, string(body)), workItemJSON(t, `{"is_wiki_enabled":false,"work_item_type_default_level":0}`)) {
			t.Errorf("request = %s %s %s", r.Method, r.URL, body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"is_wiki_enabled":false,"work_item_type_default_level":0}`)
	}))
	defer server.Close()
	args := []string{"workspace-feature", "update", "--api-version", "v2", "--workspace", "Team", "--base-url", server.URL, "--is-wiki-enabled=false", "--work-item-type-default-level", "0", "--yes"}
	code, stdout, stderr := invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
	if code != 0 || calls != 1 || stderr != "" {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
	args = append(args, "--body-file", workItemBodyFile(t, `{}`))
	code, stdout, stderr = invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
	if code != 2 || calls != 1 || stdout != "" {
		t.Fatalf("body conflict: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}

func TestP08PermissionPagination(t *testing.T) {
	for _, resource := range []string{"role", "permission-scheme", "permission-grant"} {
		for _, style := range []string{"offset", "cursor"} {
			t.Run(resource+"-"+style, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					q := r.URL.Query()
					if q.Get("per_page") != "1" || q.Get("fields") != "id" || q.Get("order_by") != "id" {
						t.Errorf("lost pagination query: %s", r.URL)
					}
					if resource == "permission-grant" && (q.Get("resource_type") != "project" || q.Get("resource_id") != "82a503f0-cb9c-4460-a838-6116c4d51dfe") {
						t.Errorf("lost grant resource scope: %s", r.URL)
					}
					w.Header().Set("Content-Type", "application/json")
					if style == "cursor" {
						if calls == 1 {
							_, _ = io.WriteString(w, `{"data":[{"id":"first"}],"next_cursor":"opaque+/=","has_more":true,"pagination":{"style":"cursor"}}`)
						} else {
							if q.Get("cursor") != "opaque+/=" {
								t.Errorf("cursor changed: %s", r.URL)
							}
							_, _ = io.WriteString(w, `{"data":[{"id":"last"}],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`)
						}
					} else if calls == 1 {
						_, _ = io.WriteString(w, `{"data":[{"id":"first"}],"next":1,"previous":null,"total_count":2,"pagination":{"style":"offset"}}`)
					} else {
						if q.Get("offset") != "1" {
							t.Errorf("offset changed: %s", r.URL)
						}
						_, _ = io.WriteString(w, `{"data":[{"id":"last"}],"next":null,"previous":0,"total_count":2,"pagination":{"style":"offset"}}`)
					}
				}))
				defer server.Close()
				args := []string{resource, "list", "--api-version", "v2", "--workspace", "Team", "--base-url", server.URL, "--all", "--max-pages", "2", "--per-page", "1", "--order-by", "id", "--fields", "id"}
				if style == "cursor" {
					args = append(args, "--paginate", "cursor")
				}
				if resource == "permission-grant" {
					args = append(args, "--resource-type", "project", "--resource-id", "82a503f0-cb9c-4460-a838-6116c4d51dfe")
				}
				code, stdout, stderr := invoke(t, p08PermissionCatalog(t), args, p08PermissionHandlers())
				if code != 0 || calls != 2 || stderr != "" || !strings.Contains(stdout, `"complete":true`) {
					t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
				}
				result := workItemJSON(t, stdout).(map[string]any)
				if len(result["data"].([]any)) != 2 {
					t.Fatalf("native pages not retained: %s", stdout)
				}
			})
		}
	}
}
