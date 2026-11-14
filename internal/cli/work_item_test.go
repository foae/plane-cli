package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/spec"
)

const workItemTestProject = "4af68566-94a4-4eb3-94aa-50dc9427067b"

func workItemCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func workItemBodyFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func workItemJSON(t *testing.T, value string) any {
	t.Helper()
	decoded, err := spec.Decode([]byte(value))
	if err != nil {
		t.Fatalf("invalid fixture JSON: %v", err)
	}
	return decoded
}

// Expected routes and bodies here are pinned independently to the official
// operation pages, not derived from the descriptor route under test.
func TestWorkItemOperationRequests(t *testing.T) {
	const uuid = "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"
	const project = "4af68566-94a4-4eb3-94aa-50dc9427067b"
	type fixture struct {
		name, version, action, method, path, query, body, response string
		args                                                       []string
		status                                                     int
	}
	fixtures := []fixture{
		{name: "v1-create", version: "v1", action: "create", method: "POST", path: "/projects/" + project + "/work-items/", body: `{"name":"Legacy","description_html":"<p></p>","estimate_point":"0","is_draft":false,"point":0,"sequence_id":9007199254740993,"sort_order":0,"assignees":[],"labels":[],"type_id":"type","parent":null,"deleted_at":"","description_stripped":"","priority":"none","start_date":"","target_date":"","completed_at":null,"archived_at":null,"last_activity_at":"","external_source":"","external_id":"","created_by":"author","state":"state","type":"type"}`, args: []string{"--yes"}, status: 201},
		{name: "v1-list", version: "v1", action: "list", method: "GET", path: "/projects/" + project + "/work-items/", query: "cursor=20%3A1%3A0&expand=state&external_id=0&external_source=jira&fields=id%2Cname&order_by=-created_at&per_page=20", args: []string{"--scope", "project", "--cursor", "20:1:0", "--expand", "state", "--external-id", "0", "--external-source", "jira", "--fields", "id,name", "--order-by=-created_at", "--per-page", "20"}, response: `{"results":[],"next_page_results":false,"next_cursor":"20:2:0"}`},
		{name: "v1-get", version: "v1", action: "get", method: "GET", path: "/projects/" + project + "/work-items/" + uuid + "/", query: "expand=assignees&external_id=x&external_source=jira&fields=id&order_by=-created_at", args: []string{"--id", uuid, "--expand", "assignees", "--external-id", "x", "--external-source", "jira", "--fields", "id", "--order-by=-created_at"}},
		{name: "v1-identifier", version: "v1", action: "get-by-identifier", method: "GET", path: "/work-items/ENG-12/", args: []string{"--identifier", "ENG-12"}},
		{name: "v1-search", version: "v1", action: "search", method: "GET", path: "/work-items/search/", query: "limit=10&project_id=" + project + "&search=login+loop&workspace_search=false", args: []string{"--search", "login loop", "--limit", "10", "--project-id", project, "--workspace-search", "false"}, response: `{"issues":[{"id":"item","sequence_id":12}]}`},
		{name: "v1-advanced-search", version: "v1", action: "advanced-search", method: "POST", path: "/work-items/advanced-search/", body: `{"query":"","filters":{"priority":["high"],"unknown_extension":false},"limit":0,"workspace_search":false,"project_id":"project"}`, response: `[[{"id":"item","sequence_id":12}]]`},
		{name: "v1-update", version: "v1", action: "update", method: "PATCH", path: "/projects/" + project + "/work-items/" + uuid + "/", body: `{"name":"","estimate_point":"0","description_html":"","point":0,"is_draft":false,"assignees":[],"labels":[],"parent":null}`, args: []string{"--id", uuid}},
		{name: "v1-delete", version: "v1", action: "delete", method: "DELETE", path: "/projects/" + project + "/work-items/" + uuid + "/", args: []string{"--id", uuid, "--yes"}, status: 204},
		{name: "v2-create", version: "v2", action: "create", method: "POST", path: "/projects/" + project + "/work-items/", query: "expand=state&fields=id%2Cname", body: `{"name":"Fix login","description_html":"<p>Exact HTML</p>","priority":"none","state":"In Progress","type":"Bug","parent":"ENG-1","assignees":["ana@example.com"],"labels":[],"estimate":"0","start_date":"2026-01-01","target_date":"2026-01-02","external_id":"","external_source":"","custom_fields":{"zero":0,"false":false,"null":null,"empty":[]}}`, args: []string{"--fields", "id,name", "--expand", "state"}, status: 201},
		{name: "v2-list-project", version: "v2", action: "list", method: "GET", path: "/projects/" + project + "/work-items/", query: "archived=only&assignee_id__isnull=false&count=false&cursor=a%2Bb%3D&fields=id%2Cname&order_by=created_at&paginate=cursor&per_page=200&priority__in=urgent%2Chigh&search=login&state_group=started", args: []string{"--scope", "project", "--archived", "only", "--assignee-id-isnull=false", "--count=false", "--cursor", "a+b=", "--fields", "id,name", "--order-by", "created_at", "--paginate", "cursor", "--per-page", "200", "--priority-in", "urgent,high", "--search", "login", "--state-group", "started"}, response: `{"data":[],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`},
		{name: "v2-list-workspace", version: "v2", action: "list", method: "GET", path: "/work-items/", query: "initiative_id=initiative&offset=0&project_id=" + project + "&project_id__in=a%2Cb&sequence_id=9007199254740993", args: []string{"--scope", "workspace", "--project-id", project, "--project-id-in", "a,b", "--initiative-id", "initiative", "--offset", "0", "--sequence-id", "9007199254740993"}, response: `{"data":[],"next":null,"pagination":{"style":"offset"}}`},
		{name: "v2-get", version: "v2", action: "get", method: "GET", path: "/projects/" + project + "/work-items/eng-12/", query: "expand=state&fields=id%2Ccustom_fields", args: []string{"--id", "eng-12", "--fields", "id,custom_fields", "--expand", "state"}},
		{name: "v2-identifier", version: "v2", action: "get-by-identifier", method: "GET", path: "/work-items/eng-12/", query: "expand=state&fields=all", args: []string{"--identifier", "eng-12", "--fields", "all", "--expand", "state"}},
		{name: "v2-update", version: "v2", action: "update", method: "PATCH", path: "/projects/" + project + "/work-items/ENG-12/", body: `{"name":"","description_html":"","assignee_ids":[],"label_ids":[],"type_id":null,"parent_id":null,"estimate_point_id":null,"start_date":null,"target_date":null,"external_id":"","custom_fields":{"count":0,"active":false,"value":null}}`, args: []string{"--id", "ENG-12"}},
		{name: "v2-delete", version: "v2", action: "delete", method: "DELETE", path: "/projects/" + project + "/work-items/ENG-12/", query: "fields=id", args: []string{"--id", "ENG-12", "--yes", "--fields", "id"}, status: 204},
		{name: "v2-upsert", version: "v2", action: "upsert", method: "POST", path: "/projects/ENG/work-items/upsert/", body: `{"name":"Imported","external_source":"jira","external_id":"JIRA-1","description_html":null,"cycle_id":null,"module_ids":[],"state":null,"type":null,"parent":null,"estimate":null,"target_date":null}`, args: []string{"--project", "ENG"}, status: 201},
		{name: "v2-bulk-create", version: "v2", action: "bulk-create", method: "POST", path: "/projects/ENG/work-items/bulk-create/", body: `{"items":[{"name":"First","assignee_ids":[],"custom_fields":{"zero":0}}],"all_or_none":false}`, args: []string{"--project", "ENG"}, response: `{"results":[{"index":0,"result":"created","id":"item"}],"succeeded":1,"failed":0}`},
		{name: "v2-bulk-update", version: "v2", action: "bulk-update", method: "POST", path: "/projects/ENG/work-items/bulk-update/", body: `{"items":[{"id":"` + uuid + `","target_date":null,"labels":[]}],"all_or_none":true}`, args: []string{"--project", "ENG"}, response: `{"results":[{"index":0,"result":"updated","id":"item"}],"succeeded":1,"failed":0}`},
		{name: "v2-bulk-delete", version: "v2", action: "bulk-delete", method: "POST", path: "/projects/ENG/work-items/bulk-delete/", body: `{"ids":["` + uuid + `"],"all_or_none":false}`, args: []string{"--project", "ENG", "--yes"}, response: `{"results":[{"index":0,"result":"deleted","id":"item"}],"succeeded":1,"failed":0}`},
		{name: "v2-archive", version: "v2", action: "archive", method: "POST", path: "/projects/" + project + "/work-items/ENG-12/archive/", args: []string{"--id", "ENG-12", "--yes"}, response: `{"id":"item","archived_at":"2026-01-01T00:00:00Z"}`},
		{name: "v2-unarchive", version: "v2", action: "unarchive", method: "POST", path: "/projects/" + project + "/work-items/ENG-12/unarchive/", args: []string{"--id", "ENG-12"}, response: `{"id":"item","archived_at":null}`},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != f.method || r.URL.EscapedPath() != "/prefix/api/"+f.version+"/workspaces/team"+f.path || r.URL.RawQuery != f.query {
					t.Errorf("request: %s %s?%s", r.Method, r.URL.EscapedPath(), r.URL.RawQuery)
				}
				if r.Header.Get("X-Api-Key") != "fixture-token" {
					t.Error("missing selected API key")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if f.body == "" {
					if len(body) != 0 {
						t.Errorf("unexpected body: %s", body)
					}
				} else if !reflect.DeepEqual(workItemJSON(t, string(body)), workItemJSON(t, f.body)) {
					t.Errorf("body: %s; want %s", body, f.body)
				}
				w.Header().Set("Content-Type", "application/json")
				if f.action == "upsert" {
					w.Header().Set("X-Plane-Upsert", "created")
				}
				status := f.status
				if status == 0 {
					status = 200
				}
				w.WriteHeader(status)
				if status != 204 {
					response := f.response
					if response == "" {
						response = `{"id":"item","unknown":9007199254740993}`
					}
					_, _ = io.WriteString(w, response)
				}
			}))
			defer server.Close()
			args := []string{"work-item", f.action, "--api-version", f.version, "--base-url", server.URL + "/prefix", "--workspace", "team"}
			if f.action != "get-by-identifier" && f.action != "search" && f.action != "advanced-search" && !strings.Contains(f.name, "list-workspace") {
				args = append(args, "--project", project)
			}
			args = append(args, f.args...)
			if f.body != "" {
				args = append(args, "--body-file", workItemBodyFile(t, f.body))
			}
			code, out, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, out, stderr)
			}
			result := workItemJSON(t, out).(map[string]any)
			meta := result["meta"].(map[string]any)
			if meta["api_version"] != f.version || meta["http_status"] != json.Number(fmt.Sprint(func() int {
				if f.status == 0 {
					return 200
				}
				return f.status
			}())) {
				t.Fatalf("metadata: %s", out)
			}
			if f.status == 204 && result["data"] != nil {
				t.Fatalf("204 body: %s", out)
			}
			if f.response == "" && f.status != 204 && !strings.Contains(out, "9007199254740993") {
				t.Fatalf("unknown large integer lost: %s", out)
			}
		})
	}
}

func TestWorkItemLifecycle(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			step := 0
			name := "Created"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods := []string{"POST", "GET", "PATCH", "GET", "DELETE"}
				paths := []string{"/work-items/", "/work-items/id/", "/work-items/id/", "/work-items/", "/work-items/id/"}
				if step >= len(methods) {
					t.Error("unexpected request")
					w.WriteHeader(500)
					return
				}
				if r.Method != methods[step] || r.URL.Path != "/api/"+version+"/workspaces/team/projects/"+workItemTestProject+paths[step] {
					t.Errorf("step %d: %s %s", step, r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				switch step {
				case 0, 2:
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					name, _ = body["name"].(string)
					if step == 0 {
						w.WriteHeader(201)
					}
					_, _ = fmt.Fprintf(w, `{"id":"id","name":%q}`, name)
				case 1:
					_, _ = fmt.Fprintf(w, `{"id":"id","name":%q}`, name)
				case 3:
					if version == "v1" {
						_, _ = fmt.Fprintf(w, `{"results":[{"id":"id","name":%q}],"next_page_results":false,"next_cursor":"20:1:0"}`, name)
					} else {
						_, _ = fmt.Fprintf(w, `{"data":[{"id":"id","name":%q}],"next":null,"pagination":{"style":"offset"}}`, name)
					}
				case 4:
					w.WriteHeader(204)
				}
				step++
			}))
			defer server.Close()
			for _, operation := range []struct {
				action string
				args   []string
			}{{"create", []string{"--name", "Created"}}, {"get", []string{"--id", "id"}}, {"update", []string{"--id", "id", "--name", "Updated"}}, {"list", []string{"--scope", "project"}}, {"delete", []string{"--id", "id", "--yes"}}} {
				args := append([]string{"work-item", operation.action, "--api-version", version, "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject}, operation.args...)
				code, out, errout := invoke(t, workItemCatalog(t), args, workItemHandlers())
				if code != 0 || errout != "" {
					t.Fatalf("%s: %d %s %s", operation.action, code, out, errout)
				}
				if (operation.action == "update" || operation.action == "list") && !strings.Contains(out, "Updated") {
					t.Fatalf("updated state not returned: %s", out)
				}
			}
			if step != 5 {
				t.Fatalf("steps=%d", step)
			}
		})
	}
}

func TestWorkItemLocalValidationAndSafety(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	cases := []struct {
		name, version, action, body string
		args                        []string
		exit                        int
	}{
		{"missing-name", "v2", "create", `{}`, nil, 2},
		{"duplicate-json", "v2", "create", `{"name":"first","name":"second"}`, nil, 2},
		{"trailing-json", "v2", "create", `{"name":"first"} {}`, nil, 2},
		{"wrong-shape", "v2", "create", `[]`, nil, 2},
		{"unknown-write", "v2", "create", `{"name":"x","description":"wrong"}`, nil, 2},
		{"relation-conflict", "v2", "create", `{"name":"x","state":"Done","state_id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"}`, nil, 2},
		{"legacy-estimate-number", "v1", "create", `{"name":"x","estimate_point":0}`, nil, 2},
		{"legacy-description", "v1", "update", `{"description":"wrong"}`, []string{"--id", "id"}, 2},
		{"body-and-field", "v2", "create", `{"name":"x"}`, []string{"--name", "other"}, 2},
		{"bulk-empty", "v2", "bulk-create", `{"items":[]}`, nil, 2},
		{"bulk-oversize", "v2", "bulk-create", `{"items":[` + strings.TrimSuffix(strings.Repeat(`{"name":"x"},`, 51), ",") + `]}`, nil, 2},
		{"bulk-bad-id", "v2", "bulk-update", `{"items":[{"id":"ENG-1","name":"x"}]}`, nil, 2},
		{"bulk-delete-bad-id", "v2", "bulk-delete", `{"ids":["ENG-1"]}`, []string{"--yes"}, 2},
		{"upsert-key", "v2", "upsert", `{"name":"x","external_id":"x"}`, nil, 2},
		{"read-only", "v2", "update", `{"archived_at":"2026-01-01"}`, []string{"--id", "id"}, 2},
		{"delete-confirmation", "v2", "delete", "", []string{"--id", "id"}, 2},
		{"archive-confirmation", "v2", "archive", "", []string{"--id", "id"}, 2},
		{"bulk-delete-confirmation", "v2", "bulk-delete", `{"ids":["b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"]}`, nil, 2},
		{"legacy-body-archive", "v1", "update", `{"archived_at":"2026-01-01T00:00:00Z"}`, []string{"--id", "id"}, 2},
		{"unavailable-bulk", "v1", "bulk-create", `{"items":[{"name":"x"}]}`, nil, 7},
		{"unavailable-search", "v2", "search", "", []string{"--search", "x"}, 7},
		{"unavailable-workspace", "v1", "list", "", []string{"--scope", "workspace"}, 2},
		{"scope-required", "v2", "list", "", nil, 2},
		{"project-filter-scope", "v2", "list", "", []string{"--scope", "project", "--project-id", "project"}, 2},
		{"v1-page-bound", "v1", "list", "", []string{"--scope", "project", "--per-page", "101"}, 2},
		{"offset-bound", "v2", "list", "", []string{"--scope", "project", "--offset", "10001"}, 2},
		{"table-write", "v2", "create", `{"name":"x"}`, []string{"--output", "table"}, 2},
		{"project-key-not-create-id", "v2", "create", `{"name":"x"}`, []string{"--project", "ENG"}, 2},
		{"project-key-not-list-id", "v2", "list", "", []string{"--scope", "project", "--project", "ENG"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := []string{"work-item", c.action, "--api-version", c.version, "--base-url", server.URL, "--workspace", "team"}
			if c.action != "search" {
				args = append(args, "--project", workItemTestProject)
			}
			args = append(args, c.args...)
			if c.body != "" {
				args = append(args, "--body-file", workItemBodyFile(t, c.body))
			}
			code, out, errout := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if code != c.exit || out != "" || errout == "" {
				t.Fatalf("code=%d want=%d out=%s err=%s", code, c.exit, out, errout)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("local failure sent %d requests", calls.Load())
	}
	args := []string{"work-item", "archive", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject, "--id", "ENG-1", "--dry-run"}
	code, out, errout := invoke(t, workItemCatalog(t), args, workItemHandlers())
	if code != 0 || errout != "" || !strings.Contains(out, `"requires_confirmation":true`) || calls.Load() != 0 {
		t.Fatalf("dry run: %d %s %s calls=%d", code, out, errout, calls.Load())
	}
}

func TestWorkItemBulkPartialAndAPIErrors(t *testing.T) {
	for _, c := range []struct {
		name, action, body, response string
		status, exit                 int
		args                         []string
	}{
		{"bulk-create-mixed", "bulk-create", `{"items":[{"name":"good"},{"name":"bad"}]}`, `{"results":[{"index":0,"result":"created","id":"created"},{"index":1,"result":"failed","type":"invalid_request","code":"invalid_request","detail":"One or more fields failed validation.","errors":[{"field":"name","code":"unique","message":"A work item with this name already exists."}]}],"succeeded":1,"failed":1}`, 200, 6, nil},
		{"bulk-update-mixed", "bulk-update", `{"items":[{"id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"},{"id":"9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88"}]}`, `{"results":[{"index":0,"result":"updated","id":"updated"},{"index":1,"result":"failed","type":"not_found","code":"not_found","detail":"No work item matches the given query."}],"succeeded":1,"failed":1}`, 200, 6, nil},
		{"bulk-delete-mixed", "bulk-delete", `{"ids":["b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53","9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88"]}`, `{"results":[{"index":0,"result":"deleted","id":"deleted"},{"index":1,"result":"failed","type":"not_found","code":"not_found","detail":"No work item matches the given query."}],"succeeded":1,"failed":1}`, 200, 6, []string{"--yes"}},
		{"atomic-conflict", "bulk-create", `{"items":[{"name":"bad"}],"all_or_none":true}`, `{"type":"conflict","code":"conflict","detail":"No rows were written because all_or_none was set and at least one row failed.","errors":[{"field":"items.0.name","code":"unique","message":"Conflict"}]}`, 409, 5, nil},
		{"feature", "get", "", `{"type":"payment_required","code":"payment_required","detail":"Plan unavailable"}`, 402, 5, []string{"--id", "ENG-12"}},
		{"permission", "delete", "", `{"type":"forbidden","code":"forbidden","detail":"Not allowed"}`, 403, 3, []string{"--id", "ENG-12", "--yes"}},
		{"archived-not-found", "get", "", `{"type":"not_found","code":"not_found","detail":"Not found"}`, 404, 5, []string{"--id", "ENG-12"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(c.status)
				_, _ = io.WriteString(w, c.response)
			}))
			defer server.Close()
			args := append([]string{"work-item", c.action, "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject}, c.args...)
			if c.body != "" {
				args = append(args, "--body-file", workItemBodyFile(t, c.body))
			}
			code, out, errout := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if code != c.exit || errout == "" || calls != 1 {
				t.Fatalf("code=%d out=%s err=%s calls=%d", code, out, errout, calls)
			}
			if c.exit == 6 {
				result := workItemJSON(t, out).(map[string]any)
				if result["meta"].(map[string]any)["status"] != "partial" || !reflect.DeepEqual(result["data"], workItemJSON(t, c.response)) {
					t.Fatalf("native partial results lost: %s", out)
				}
			} else if out != "" {
				t.Fatalf("unexpected error stdout: %s", out)
			}
		})
	}
}

func TestWorkItemPaginationAndSparseTable(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("fields") != "id,name" {
					t.Error("fields changed during traversal")
				}
				if version == "v1" {
					if calls == 1 {
						_, _ = io.WriteString(w, `{"results":[],"next_page_results":true,"next_cursor":"20:1:0"}`)
					} else {
						if r.URL.Query().Get("cursor") != "20:1:0" {
							t.Error("wrong v1 cursor")
						}
						_, _ = io.WriteString(w, `{"results":[{"id":"second"}],"next_page_results":false,"next_cursor":"20:2:0"}`)
					}
				} else {
					if calls == 1 {
						_, _ = io.WriteString(w, `{"data":[],"next":50,"pagination":{"style":"offset"}}`)
					} else {
						if r.URL.Query().Get("offset") != "50" {
							t.Error("wrong v2 offset")
						}
						_, _ = io.WriteString(w, `{"data":[{"id":"second"}],"next":null,"pagination":{"style":"offset"}}`)
					}
				}
			}))
			defer server.Close()
			args := []string{"work-item", "list", "--api-version", version, "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject, "--scope", "project", "--fields", "id,name", "--all", "--max-pages", "2"}
			code, out, errout := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if code != 0 || errout != "" || calls != 2 {
				t.Fatalf("pagination: %d %s %s calls=%d", code, out, errout, calls)
			}
			result := workItemJSON(t, out).(map[string]any)
			if len(result["data"].([]any)) != 2 {
				t.Fatalf("native pages not preserved: %s", out)
			}
		})
	}
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"id","name":"Hello\u001b[31mWorld","unexpected":true}`)
	}))
	defer server.Close()
	code, out, errout := invoke(t, workItemCatalog(t), []string{"work-item", "get", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject, "--id", "id", "--output", "table"}, workItemHandlers())
	if code != 0 || errout != "" || strings.Contains(out, "\x1b") || !strings.Contains(out, "id") || !strings.Contains(out, "Hello") {
		t.Fatalf("sparse table: %d %q %s", code, out, errout)
	}
}

func TestWorkItemRequestQueryEscaping(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"issues":[]}`)
	}))
	defer server.Close()
	value := "a&b=é + /"
	code, out, errout := invoke(t, workItemCatalog(t), []string{"work-item", "search", "--api-version", "v1", "--base-url", server.URL, "--workspace", "team", "--search", value}, workItemHandlers())
	if code != 0 || errout != "" || got.Get("search") != value || len(got) != 1 {
		t.Fatalf("query: %d %s %s got=%v", code, out, errout, got)
	}
}

func TestWorkItemCursorTraversalAndStagedFailure(t *testing.T) {
	for _, failSecond := range []bool{false, true} {
		t.Run(fmt.Sprint(failSecond), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("paginate") != "cursor" || r.URL.Query().Get("order_by") != "created_at" || r.URL.Query().Get("search") != "same filter" {
					t.Error("cursor filters changed")
				}
				if calls == 1 {
					_, _ = io.WriteString(w, `{"data":[],"next_cursor":"opaque+token=","has_more":true,"pagination":{"style":"cursor"}}`)
					return
				}
				if r.URL.Query().Get("cursor") != "opaque+token=" {
					t.Error("opaque cursor changed")
				}
				if failSecond {
					w.WriteHeader(500)
					_, _ = io.WriteString(w, `{"type":"server_error","detail":"Unavailable"}`)
					return
				}
				_, _ = io.WriteString(w, `{"data":[{"id":"second"}],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`)
			}))
			defer server.Close()
			args := []string{"work-item", "list", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject, "--scope", "project", "--paginate", "cursor", "--order-by", "created_at", "--search", "same filter", "--all", "--max-pages", "2"}
			code, out, errout := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if failSecond {
				if code != 5 || out != "" || errout == "" {
					t.Fatalf("staged error: %d %s %s", code, out, errout)
				}
			} else if code != 0 || errout != "" || !strings.Contains(out, "second") {
				t.Fatalf("cursor traversal: %d %s %s", code, out, errout)
			}
			if calls != 2 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestWorkItemMutationUnknownOutcome(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	var calls atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		<-release
	}))
	defer server.Close()
	code, out, errout := invoke(t, workItemCatalog(t), []string{"work-item", "create", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--project", workItemTestProject, "--name", "Uncertain", "--request-timeout", "100ms"}, workItemHandlers())
	close(release)
	if code != 4 || out != "" || !strings.Contains(errout, `"outcome_unknown":true`) || calls.Load() != 1 {
		t.Fatalf("mutation timeout: %d %s %s calls=%d", code, out, errout, calls.Load())
	}
}

func TestWorkItemConfiguredScopeDefaults(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v2/workspaces/default-team/projects/"+workItemTestProject+"/work-items/item/" {
			t.Errorf("default path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"item"}`)
	}))
	defer server.Close()
	catalog := workItemCatalog(t)
	code, out, errout := invoke(t, catalog, []string{"connection", "add", "scope-defaults", "--base-url", server.URL, "--api-version", "v2", "--workspace", "default-team", "--project", workItemTestProject}, workItemHandlers())
	if code != 0 {
		t.Fatalf("configure defaults: %d %s %s", code, out, errout)
	}
	code, out, errout = invoke(t, catalog, []string{"work-item", "get", "--connection", "scope-defaults", "--id", "item"}, workItemHandlers())
	if code != 0 || errout != "" || calls != 1 {
		t.Fatalf("configured defaults: %d %s %s calls=%d", code, out, errout, calls)
	}
}

// Build once for the complete two-generation process lifecycle, then execute
// only the real CLI binary against isolated generation-specific fake servers.
func TestWorkItemProcessLifecycle(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "plane-cli")
	build := exec.Command("go", "build", "-o", binary, "./cmd/plane-cli")
	build.Dir = filepath.Join("..", "..")
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI lifecycle binary: %v\n%s", err, data)
	}
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			const item = "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"
			step := 0
			name := "Created"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods := []string{"POST", "GET", "PATCH", "GET", "DELETE"}
				updateID := item
				if version == "v2" {
					updateID = "ENG-12"
				}
				paths := []string{"", item + "/", updateID + "/", "", item + "/"}
				if step >= len(methods) {
					t.Error("unexpected lifecycle process request")
					w.WriteHeader(500)
					return
				}
				wantPath := "/api/" + version + "/workspaces/process-team/projects/" + workItemTestProject + "/work-items/" + paths[step]
				if r.Method != methods[step] || r.URL.Path != wantPath || r.URL.RawQuery != "" {
					t.Errorf("process step %d: %s %s", step, r.Method, r.URL)
				}
				if r.Header.Get("X-Api-Key") != "process-fixture-token" {
					t.Error("process API key missing")
				}
				w.Header().Set("Content-Type", "application/json")
				if step == 0 || step == 2 {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					wantName := "Created"
					if step == 2 {
						wantName = "Updated"
					}
					if !reflect.DeepEqual(body, map[string]any{"name": wantName}) {
						t.Errorf("process body: %#v", body)
					}
					name = wantName
				} else if data, err := io.ReadAll(r.Body); err != nil || len(data) != 0 {
					t.Errorf("unexpected process body: %s %v", data, err)
				}
				switch step {
				case 0:
					w.WriteHeader(201)
					_, _ = fmt.Fprintf(w, `{"id":%q,"identifier":"ENG-12","name":%q}`, item, name)
				case 1, 2:
					_, _ = fmt.Fprintf(w, `{"id":%q,"identifier":"ENG-12","name":%q}`, item, name)
				case 3:
					if version == "v1" {
						_, _ = fmt.Fprintf(w, `{"results":[{"id":%q,"name":%q}],"next_cursor":"20:1:0","next_page_results":false}`, item, name)
					} else {
						_, _ = fmt.Fprintf(w, `{"data":[{"id":%q,"name":%q}],"next":null,"pagination":{"style":"offset"}}`, item, name)
					}
				case 4:
					w.WriteHeader(204)
				}
				step++
			}))
			defer server.Close()
			home := t.TempDir()
			configDir := filepath.Join(privateTempDir(t), "config")
			updateID := item
			if version == "v2" {
				updateID = "ENG-12"
			}
			for _, operation := range []struct {
				action string
				args   []string
				status int
			}{
				{"create", []string{"--name", "Created"}, 201},
				{"get", []string{"--id", item}, 200},
				{"update", []string{"--id", updateID, "--name", "Updated"}, 200},
				{"list", []string{"--scope", "project"}, 200},
				{"delete", []string{"--id", item, "--yes"}, 204},
			} {
				args := append([]string{"work-item", operation.action, "--api-version", version, "--base-url", server.URL, "--workspace", "process-team", "--project", workItemTestProject}, operation.args...)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				process := exec.CommandContext(ctx, binary, args...)
				process.Dir = home
				process.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "PLANE_CLI_CONFIG_DIR=" + configDir, "PLANE_CLI_API_KEY=process-fixture-token", "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "NO_PROXY=*"}
				var out, errout bytes.Buffer
				process.Stdout = &out
				process.Stderr = &errout
				err := process.Run()
				cancel()
				if err != nil || errout.Len() != 0 {
					t.Fatalf("process %s: %v stdout=%s stderr=%s", operation.action, err, &out, &errout)
				}
				result := workItemJSON(t, out.String()).(map[string]any)
				meta := result["meta"].(map[string]any)
				if meta["command"] != "work-item "+operation.action || meta["api_version"] != version || meta["http_status"] != json.Number(fmt.Sprint(operation.status)) {
					t.Fatalf("process metadata: %s", &out)
				}
				if (operation.action == "update" || operation.action == "list") && !strings.Contains(out.String(), "Updated") {
					t.Fatalf("process updated state: %s", &out)
				}
				if operation.action == "delete" && result["data"] != nil {
					t.Fatalf("process 204 result: %s", &out)
				}
			}
			if step != 5 {
				t.Fatalf("process lifecycle steps=%d", step)
			}
		})
	}
}
