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

type p08MembershipFixture struct {
	Name     string          `json:"name"`
	Args     []string        `json:"args"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Query    string          `json:"query"`
	Body     json.RawMessage `json:"body"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"`
	Mutation bool            `json:"mutation"`
}

func p08MembershipFixtures(t *testing.T) []p08MembershipFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/p08-membership-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []p08MembershipFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func p08MembershipCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func p08MembershipArgs(t *testing.T, f p08MembershipFixture, base string) []string {
	t.Helper()
	args := append(append([]string{}, f.Args...), "--base-url", base)
	if string(f.Body) != "null" {
		args = append(args, "--body-file", workItemBodyFile(t, string(f.Body)))
	}
	return args
}

func TestP08MembershipRequests(t *testing.T) {
	fixtures := p08MembershipFixtures(t)
	if len(fixtures) != 49 {
		t.Fatalf("operation fixture count = %d; want 49", len(fixtures))
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "membership-fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != f.Method || r.URL.EscapedPath() != "/prefix"+f.Path || r.URL.RawQuery != f.Query {
					t.Errorf("request = %s %s?%s; want %s %s?%s", r.Method, r.URL.EscapedPath(), r.URL.RawQuery, f.Method, "/prefix"+f.Path, f.Query)
				}
				if r.Header.Get("X-Api-Key") != "membership-fixture-key" || r.Header.Get("Authorization") != "" {
					t.Error("selected credential header not preserved")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(f.Body) == "null" {
					if len(body) != 0 {
						t.Error("unexpected request body")
					}
				} else if !reflect.DeepEqual(workItemJSON(t, string(body)), workItemJSON(t, string(f.Body))) {
					t.Error("native body fields changed")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(f.Status)
				if f.Status != 204 {
					_, _ = w.Write(f.Response)
				}
			}))
			defer server.Close()
			code, stdout, stderr := invoke(t, p08MembershipCatalog(t), p08MembershipArgs(t, f, server.URL+"/prefix"), adminMembershipHandlers())
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			result := workItemJSON(t, stdout).(map[string]any)
			if !reflect.DeepEqual(result["data"], workItemJSON(t, string(f.Response))) {
				t.Fatal("native response changed, including nested v1 roster or nullable role")
			}
			meta := result["meta"].(map[string]any)
			if meta["http_status"] != json.Number(fmt.Sprint(f.Status)) {
				t.Fatalf("metadata = %s", stdout)
			}
		})
	}
}

func TestP08MembershipSafety(t *testing.T) {
	for _, f := range p08MembershipFixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			transportEnv(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(500) }))
			defer server.Close()
			var args []string
			for _, arg := range p08MembershipArgs(t, f, server.URL) {
				if arg != "--yes" {
					args = append(args, arg)
				}
			}
			if f.Mutation {
				code, out, stderr := invoke(t, p08MembershipCatalog(t), args, adminMembershipHandlers())
				if code != 2 || out != "" || calls != 0 || !strings.Contains(stderr, "confirmation_required") {
					t.Fatalf("confirmation bypassed: code=%d calls=%d stderr=%s", code, calls, stderr)
				}
			}
			code, out, stderr := invoke(t, p08MembershipCatalog(t), append(args, "--dry-run"), adminMembershipHandlers())
			if code != 0 || calls != 0 || stderr != "" {
				t.Fatalf("dry-run: code=%d calls=%d stderr=%s", code, calls, stderr)
			}
			data := workItemJSON(t, out).(map[string]any)["data"].(map[string]any)
			if data["requires_confirmation"] != f.Mutation {
				t.Fatalf("wrong mutation classification: %s", out)
			}
			for _, sensitive := range []string{"Private invitation note", "fixture@example.invalid"} {
				if strings.Contains(out+stderr, sensitive) {
					t.Fatal("body value leaked in dry-run diagnostics")
				}
			}
		})
	}
}

func TestP08MembershipFailures(t *testing.T) {
	for _, f := range p08MembershipFixtures(t) {
		for _, status := range []int{401, 402, 403, 404} {
			t.Run(fmt.Sprintf("%s/%d", f.Name, status), func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "membership-fixture-key")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"type":"forbidden","code":"forbidden","detail":"fixture@example.invalid Private invitation note","email":"fixture@example.invalid"}`)
				}))
				defer server.Close()
				code, out, stderr := invoke(t, p08MembershipCatalog(t), p08MembershipArgs(t, f, server.URL), adminMembershipHandlers())
				if code == 0 || calls != 1 || out != "" || !strings.Contains(stderr, fmt.Sprint(status)) {
					t.Fatalf("failure/fallback: code=%d calls=%d stderr=%s", code, calls, stderr)
				}
				for _, sensitive := range []string{"fixture@example.invalid", "Private invitation note", "membership-fixture-key"} {
					if strings.Contains(out+stderr, sensitive) {
						t.Fatal("sensitive value leaked in error diagnostics")
					}
				}
			})
		}
	}
}

func TestP08MembershipValidation(t *testing.T) {
	const id = "550e8400-e29b-41d4-a716-446655440000"
	for _, tc := range []struct {
		name string
		args []string
		body string
		want int
	}{
		{"v1-remove-blocked", []string{"member", "workspace", "remove", "--api-version", "v1"}, "", 7},
		{"v2-invitation-update-unavailable", []string{"invitation", "update", "--api-version", "v2", "--id", id}, `{}`, 7},
		{"v1-bulk-unavailable", []string{"invitation", "bulk-create", "--api-version", "v1"}, `{"emails":["fixture@example.invalid"]}`, 7},
		{"v2-by-key-unavailable", []string{"group-sync", "project-mapping", "update-by-key", "--api-version", "v2", "--project-key", "eng", "--idp-group-name", "private-group"}, `{"role":"member"}`, 7},
		{"v2-project-list-strict-uuid", []string{"member", "project", "list", "--api-version", "v2", "--project", "ENG"}, "", 2},
		{"v2-create-email", []string{"invitation", "create", "--api-version", "v2"}, `{"email":"sensitive-invalid-email","role":"custom-role"}`, 2},
		{"v2-bulk-empty", []string{"invitation", "bulk-create", "--api-version", "v2"}, `{"emails":[]}`, 2},
		{"v2-bulk-oversized", []string{"invitation", "bulk-create", "--api-version", "v2"}, `{"emails":[` + strings.Repeat(`"fixture@example.invalid",`, 100) + `"fixture@example.invalid"]}`, 2},
		{"v2-bulk-invalid-address", []string{"invitation", "bulk-create", "--api-version", "v2"}, `{"emails":["sensitive-invalid-email"]}`, 2},
		{"v2-bulk-null-message", []string{"invitation", "bulk-create", "--api-version", "v2"}, `{"emails":["fixture@example.invalid"],"message":null}`, 2},
		{"v2-bulk-role", []string{"invitation", "bulk-create", "--api-version", "v2"}, `{"emails":["fixture@example.invalid"],"role":"sensitive-invalid-role"}`, 2},
		{"v1-empty-project-update", []string{"group-sync", "project-mapping", "update", "--api-version", "v1", "--id", id}, `{}`, 2},
		{"v1-empty-workspace-update", []string{"group-sync", "workspace-mapping", "update", "--api-version", "v1", "--id", id}, `{}`, 2},
		{"v1-empty-by-key", []string{"group-sync", "project-mapping", "update-by-key", "--api-version", "v1", "--project-key", "ENG", "--idp-group-name", "private-group"}, `{}`, 2},
		{"v1-exclusive-target", []string{"group-sync", "project-mapping", "create", "--api-version", "v1"}, `{"idp_group_name":"private-group","role":"member","project":"` + id + `","all_projects":true}`, 2},
		{"v1-role-integer", []string{"member", "project", "create", "--api-version", "v1", "--project", id}, `{"member":"` + id + `","role":"member"}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(500) }))
			defer server.Close()
			args := append(append([]string{}, tc.args...), "--workspace", "team", "--base-url", server.URL, "--yes", "--dry-run")
			if tc.body != "" {
				args = append(args, "--body-file", workItemBodyFile(t, tc.body))
			}
			code, out, stderr := invoke(t, p08MembershipCatalog(t), args, adminMembershipHandlers())
			if code != tc.want || out != "" || calls != 0 {
				t.Fatalf("code=%d want=%d calls=%d stdout=%s stderr=%s", code, tc.want, calls, out, stderr)
			}
			for _, value := range []string{"sensitive-invalid-email", "sensitive-invalid-role", "fixture@example.invalid", "private-group"} {
				if strings.Contains(out+stderr, value) {
					t.Fatal("sensitive input leaked in validation diagnostic")
				}
			}
		})
	}
}

func TestP08MembershipProjectIdentifiers(t *testing.T) {
	for _, fixture := range p08MembershipFixtures(t) {
		if !strings.HasPrefix(fixture.Name, "v2:members:") || !strings.Contains(fixture.Path, "/projects/") || strings.Contains(fixture.Name, "list-project") {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "membership-fixture-key")
			calls := 0
			f := fixture
			f.Args = append([]string{}, fixture.Args...)
			for i := range f.Args {
				if f.Args[i] == "--project" {
					f.Path = strings.Replace(f.Path, "/projects/"+f.Args[i+1]+"/", "/projects/ENG/", 1)
					f.Args[i+1] = "ENG"
				}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != f.Path {
					t.Errorf("identifier path = %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(f.Status)
				if f.Status != 204 {
					_, _ = w.Write(f.Response)
				}
			}))
			defer server.Close()
			code, _, stderr := invoke(t, p08MembershipCatalog(t), p08MembershipArgs(t, f, server.URL), adminMembershipHandlers())
			if code != 0 || calls != 1 || stderr != "" {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr)
			}
		})
	}
}

func TestP08MembershipPagination(t *testing.T) {
	for _, style := range []string{"offset", "cursor"} {
		t.Run(style, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "membership-fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				q := r.URL.Query()
				if q.Get("search") != "private-filter" || q.Get("order_by") != "id" || q.Get("per_page") != "201" || q.Get("fields") != "id" {
					t.Error("pagination dropped explicit filters or rewrote upstream-clamped per_page")
				}
				w.Header().Set("Content-Type", "application/json")
				if style == "cursor" {
					if q.Get("paginate") != "cursor" {
						t.Error("cursor mode changed")
					}
					if calls == 1 {
						if q.Get("cursor") != "" {
							t.Error("invented first cursor")
						}
						_, _ = io.WriteString(w, `{"data":[],"next_cursor":"opaque+/=","has_more":true,"pagination":{"style":"cursor"}}`)
					} else {
						if q.Get("cursor") != "opaque+/=" {
							t.Error("opaque cursor changed")
						}
						_, _ = io.WriteString(w, `{"data":[{"id":"second"}],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`)
					}
				} else if calls == 1 {
					_, _ = io.WriteString(w, `{"data":[],"next":7,"previous":null,"pagination":{"style":"offset"}}`)
				} else {
					if q.Get("offset") != "7" {
						t.Error("native offset changed")
					}
					_, _ = io.WriteString(w, `{"data":[{"id":"second"}],"next":null,"previous":0,"pagination":{"style":"offset"}}`)
				}
			}))
			defer server.Close()
			args := []string{"invitation", "list", "--api-version", "v2", "--workspace", "team", "--base-url", server.URL, "--search", "private-filter", "--fields", "id", "--order-by", "id", "--per-page", "201", "--all", "--max-pages", "2"}
			if style == "cursor" {
				args = append(args, "--paginate", "cursor")
			}
			code, out, stderr := invoke(t, p08MembershipCatalog(t), args, adminMembershipHandlers())
			if code != 0 || calls != 2 || stderr != "" || !strings.Contains(out, "second") {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP08MembershipOfflineSchema(t *testing.T) {
	for _, path := range [][]string{{"member", "workspace", "remove"}, {"member", "project", "create"}, {"invitation", "bulk-create"}, {"group-sync", "config", "get"}, {"group-sync", "project-mapping", "update-by-key"}} {
		version := "v2"
		if path[len(path)-1] == "update-by-key" {
			version = "v1"
		}
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			transportEnv(t)
			args := append(append([]string{"schema"}, path...), "--api-version", version)
			code, out, stderr := invoke(t, p08MembershipCatalog(t), args, adminMembershipHandlers())
			if code != 0 || out == "" || stderr != "" {
				t.Fatalf("offline schema: code=%d stdout=%s stderr=%s", code, out, stderr)
			}
		})
	}
}
