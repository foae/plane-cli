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

type p07TemplateFixture struct {
	Name     string          `json:"name"`
	Args     []string        `json:"args"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Query    string          `json:"query"`
	Body     json.RawMessage `json:"body"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"`
}

func p07TemplateFixtures(t *testing.T) []p07TemplateFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/p07-templates-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []p07TemplateFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func p07TemplateCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

// The fixture routes and payloads are pinned to all forty official operation
// contracts, independently of the registered handler implementation.
func TestP07TemplateOperationRequests(t *testing.T) {
	for _, fixture := range p07TemplateFixtures(t) {
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
					t.Error("missing selected API key")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(fixture.Body) == "null" {
					if len(body) != 0 {
						t.Errorf("unexpected body: %s", body)
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
			args := append(append([]string{}, fixture.Args...), "--base-url", server.URL+"/prefix")
			if string(fixture.Body) != "null" {
				args = append(args, "--body-file", workItemBodyFile(t, string(fixture.Body)))
			}
			if fixture.Method == "PATCH" && strings.Contains(string(fixture.Body), `"access"`) {
				args = append(args, "--yes")
			}
			code, stdout, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			result := workItemJSON(t, stdout).(map[string]any)
			want := workItemJSON(t, string(fixture.Response))
			if !reflect.DeepEqual(result["data"], want) {
				t.Fatalf("native response changed: %s; want %s", stdout, fixture.Response)
			}
			meta := result["meta"].(map[string]any)
			if meta["api_version"] != "v2" || meta["http_status"] != json.Number(fmt.Sprint(fixture.Status)) {
				t.Fatalf("metadata: %s", stdout)
			}
		})
	}
}

func TestP07TemplateConfirmation(t *testing.T) {
	for _, fixture := range p07TemplateFixtures(t) {
		if fixture.Method != "DELETE" && (fixture.Method != "PATCH" || (!strings.Contains(string(fixture.Body), "template_data") && !strings.Contains(string(fixture.Body), `"access"`))) {
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
			for _, arg := range fixture.Args {
				if arg != "--yes" {
					args = append(args, arg)
				}
			}
			args = append(args, "--base-url", server.URL)
			if string(fixture.Body) != "null" {
				args = append(args, "--body-file", workItemBodyFile(t, string(fixture.Body)))
			}
			code, _, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
			if code != 2 || calls != 0 || !strings.Contains(stderr, "confirmation_required") {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr)
			}
		})
	}
}

func TestP07TemplateNativeFailures(t *testing.T) {
	for _, fixture := range []struct {
		name, resource, action, scope, code string
		status                              int
	}{
		{"entitlement", "project-template", "list", "", "payment_required", 402},
		{"governed-project", "page-template", "update", "project", "templates_managed_at_workspace", 409},
		{"wrong-scope", "page-template", "get", "workspace", "not_found", 404},
		{"migration", "work-item-template", "use", "project", "governance_migration_in_progress", 409},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(fixture.status)
				_, _ = fmt.Fprintf(w, `{"type":"conflict","code":%q,"detail":"Native failure"}`, fixture.code)
			}))
			defer server.Close()
			args := []string{fixture.resource, fixture.action, "--api-version", "v2", "--base-url", server.URL, "--workspace", "team"}
			if fixture.scope != "" {
				args = append(args, "--scope", fixture.scope)
			}
			if fixture.scope == "project" {
				args = append(args, "--project", "ENG")
			}
			if fixture.action != "list" {
				args = append(args, "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53")
			}
			if fixture.action == "update" || fixture.action == "use" {
				args = append(args, "--body-file", workItemBodyFile(t, `{}`))
			}
			code, stdout, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
			if code != 5 || calls != 1 || stdout != "" || !strings.Contains(stderr, fmt.Sprintf(`"http_status":%d`, fixture.status)) || !strings.Contains(stderr, `"code":"api_error"`) {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestP07TemplateInputContracts(t *testing.T) {
	for _, fixture := range []struct {
		name, resource, action, body string
		args                         []string
	}{
		{"scope-required", "view", "list", "", nil},
		{"project-required", "page-template", "list", "", []string{"--scope", "project"}},
		{"workspace-project-conflict", "view", "delete", "", []string{"--scope", "workspace", "--project", "ENG", "--id", "view", "--yes"}},
		{"workspace-cannot-use", "work-item-template", "use", `{}`, []string{"--scope", "workspace", "--id", "id"}},
		{"view-filter-is-string", "view", "create", `{"name":"View","filters":{"priority":["high"]}}`, []string{"--scope", "workspace"}},
		{"view-stored-query-read-only", "view", "create", `{"name":"View","query":"priority = high"}`, []string{"--scope", "workspace"}},
		{"view-write-access-is-string", "view", "create", `{"name":"View","access":1}`, []string{"--scope", "workspace"}},
		{"page-seed-name-required", "page-template", "create", `{"name":"Template","template_data":{}}`, []string{"--scope", "workspace"}},
		{"page-parent-shape", "page-template", "create", `{"name":"Template","template_data":{"name":"Page","parent":{"id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"}}}`, []string{"--scope", "workspace"}},
		{"project-use-network-integer", "project-template", "use", `{"name":"Project","identifier":"NEW","network":"2"}`, []string{"--id", "id"}},
		{"project-copy-id-required", "project-template", "copy", `{}`, nil},
		{"read-only-categories", "template-category", "create", `{}`, nil},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			transportEnv(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(500)
			}))
			defer server.Close()
			args := []string{fixture.resource, fixture.action, "--api-version", "v2", "--base-url", server.URL}
			if fixture.resource != "template-category" {
				args = append(args, "--workspace", "team")
			}
			args = append(args, fixture.args...)
			if fixture.body != "" {
				args = append(args, "--body-file", workItemBodyFile(t, fixture.body))
			}
			code, _, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
			if code != 2 || calls != 0 {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr)
			}
		})
	}
}

func TestP07TemplateNativeFilters(t *testing.T) {
	for _, fixture := range []struct {
		name, path, query string
		args              []string
	}{
		{"view", "/api/v2/workspaces/team/views/", "access=0&count=false&is_locked=false&owned_by_id=user&search=a%2Bb", []string{"view", "list", "--workspace", "team", "--scope", "workspace", "--access", "0", "--count=false", "--is-locked=false", "--owned-by-id", "user", "--search", "a+b"}},
		{"category", "/api/v2/template-categories/", "is_active=false&name=Engineering&order_by=id&paginate=cursor", []string{"template-category", "list", "--is-active=false", "--name", "Engineering", "--order-by", "id", "--paginate", "cursor"}},
		{"category-detail", "/api/v2/template-categories/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/", "fields=id%2Cname&is_active=false", []string{"template-category", "get", "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53", "--is-active=false", "--fields", "id,name"}},
		{"project-template", "/api/v2/workspaces/team/project-templates/", "category_id__in=a%2Cb&is_published=false&is_verified=true&short_id=AbC", []string{"project-template", "list", "--workspace", "team", "--category-id-in", "a,b", "--is-published=false", "--is-verified=true", "--short-id", "AbC"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != fixture.path || r.URL.RawQuery != fixture.query {
					t.Errorf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[],"next":null,"pagination":{"style":"offset"}}`)
			}))
			defer server.Close()
			args := append(append([]string{}, fixture.args...), "--api-version", "v2", "--base-url", server.URL)
			code, stdout, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
			if code != 0 || calls != 1 || stderr != "" {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestP07TemplateReadTables(t *testing.T) {
	for _, resource := range []string{"view", "work-item-template", "page-template", "project-template", "template-category"} {
		for _, action := range []string{"list", "get"} {
			t.Run(resource+"-"+action, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					row := `{"id":"id","name":"Visible name","access":0,"is_locked":false,"is_published":false,"short_id":"short","slug":"engineering","is_active":true}`
					if action == "list" {
						row = `{"data":[` + row + `],"next":null,"pagination":{"style":"offset"}}`
					}
					_, _ = io.WriteString(w, row)
				}))
				defer server.Close()
				args := []string{resource, action, "--api-version", "v2", "--base-url", server.URL, "--output", "table"}
				if resource != "template-category" {
					args = append(args, "--workspace", "team")
				}
				if resource == "view" || resource == "work-item-template" || resource == "page-template" {
					args = append(args, "--scope", "workspace")
				}
				if action == "get" {
					args = append(args, "--id", "id")
				}
				code, stdout, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
				if code != 0 || stderr != "" || !strings.Contains(stdout, "Visible name") {
					t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
				}
			})
		}
	}
}

func TestP07TemplateMetadataUpdateNoConfirmation(t *testing.T) {
	for _, resource := range []string{"work-item-template", "page-template", "project-template"} {
		t.Run(resource, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "PATCH" {
					t.Errorf("unexpected seed pre-read or method: %s", r.Method)
				}
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(workItemJSON(t, string(data)), workItemJSON(t, `{"short_description":null,"is_published":false}`)) {
					t.Errorf("metadata body changed: %s", data)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"template","short_description":null,"is_published":false,"template_data":{"name":"Unchanged seed","description_html":"<p>Keep me</p>"}}`)
			}))
			defer server.Close()
			args := []string{resource, "update", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--id", "template", "--body-file", workItemBodyFile(t, `{"short_description":null,"is_published":false}`)}
			if resource != "project-template" {
				args = append(args, "--scope", "workspace")
			}
			code, stdout, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
			if code != 0 || calls != 1 || stderr != "" || !strings.Contains(stdout, "Unchanged seed") {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestP07TemplateUseDefaultOverrides(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v2/workspaces/team/projects/ENG/work-item-templates/template/use/" {
			t.Errorf("unexpected apply request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if len(body) != 0 {
			t.Errorf("invented override body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":"new-item","name":"Seed name","description_html":"<p>Seed body</p>","priority":"high","state_id":"project-default","assignee_ids":[],"label_ids":[],"module_ids":[],"custom_fields":{}}`)
	}))
	defer server.Close()
	args := []string{"work-item-template", "use", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--scope", "project", "--project", "ENG", "--id", "template"}
	code, stdout, stderr := invoke(t, p07TemplateCatalog(t), args, portfolioTemplateHandlers())
	if code != 0 || calls != 1 || stderr != "" || !strings.Contains(stdout, "new-item") || !strings.Contains(stdout, "project-default") {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}
