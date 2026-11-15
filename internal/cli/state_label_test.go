package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

// Each row is an independently pinned official operation, including workspace
// state routes and bodyless default assignment. Expectations do not use handlers.
func TestStateLabelRequests(t *testing.T) {
	cases := []struct{ resource, action, version, scope, method, path, body string }{
		{`state`, `list`, `v2`, `project`, `GET`, `/api/v2/workspaces/acme/projects/ProjectID/states/`, ``},
		{`state`, `get`, `v2`, `project`, `GET`, `/api/v2/workspaces/acme/projects/ProjectID/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`state`, `create`, `v2`, `project`, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/states/`, `{"name":"Example","color":"#3f76ff"}`},
		{`state`, `update`, `v2`, `project`, `PATCH`, `/api/v2/workspaces/acme/projects/ProjectID/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sequence":0,"is_default":false}`},
		{`state`, `delete`, `v2`, `project`, `DELETE`, `/api/v2/workspaces/acme/projects/ProjectID/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`state`, `upsert`, `v2`, `project`, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/states/upsert/`, `{"color":"#3f76ff","name":"Example","external_source":"sync","external_id":"record"}`},
		{`state`, `bulk-create`, `v2`, `project`, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/states/bulk-create/`, `{"items":[{"name":"Example","color":"#3f76ff"}],"all_or_none":false}`},
		{`state`, `bulk-update`, `v2`, `project`, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/states/bulk-update/`, `{"items":[{"description":"","sequence":0,"is_default":false,"id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"}],"all_or_none":false}`},
		{`state`, `bulk-delete`, `v2`, `project`, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/states/bulk-delete/`, `{"ids":["b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"],"all_or_none":false}`},
		{`state`, `list`, `v2`, `workspace`, `GET`, `/api/v2/workspaces/acme/states/`, ``},
		{`state`, `get`, `v2`, `workspace`, `GET`, `/api/v2/workspaces/acme/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`state`, `create`, `v2`, `workspace`, `POST`, `/api/v2/workspaces/acme/states/`, `{"name":"Example","color":"#3f76ff","group":"started"}`},
		{`state`, `update`, `v2`, `workspace`, `PATCH`, `/api/v2/workspaces/acme/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sequence":0}`},
		{`state`, `delete`, `v2`, `workspace`, `DELETE`, `/api/v2/workspaces/acme/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`label`, `list`, `v2`, ``, `GET`, `/api/v2/workspaces/acme/projects/ProjectID/labels/`, ``},
		{`label`, `get`, `v2`, ``, `GET`, `/api/v2/workspaces/acme/projects/ProjectID/labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`label`, `create`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/labels/`, `{"name":"Example","parent_id":null}`},
		{`label`, `update`, `v2`, ``, `PATCH`, `/api/v2/workspaces/acme/projects/ProjectID/labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sort_order":0,"parent_id":null,"external_id":null,"external_source":null}`},
		{`label`, `delete`, `v2`, ``, `DELETE`, `/api/v2/workspaces/acme/projects/ProjectID/labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`label`, `upsert`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/labels/upsert/`, `{"name":"Example","external_source":"sync","external_id":"record","parent_id":null}`},
		{`label`, `bulk-create`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/labels/bulk-create/`, `{"items":[{"name":"Example","parent_id":null}],"all_or_none":false}`},
		{`label`, `bulk-update`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/labels/bulk-update/`, `{"items":[{"description":"","sort_order":0,"parent_id":null,"external_id":null,"external_source":null,"id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"}],"all_or_none":false}`},
		{`label`, `bulk-delete`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/projects/ProjectID/labels/bulk-delete/`, `{"ids":["b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"],"all_or_none":false}`},
		{`project-state`, `list`, `v2`, ``, `GET`, `/api/v2/workspaces/acme/project-states/`, ``},
		{`project-state`, `get`, `v2`, ``, `GET`, `/api/v2/workspaces/acme/project-states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`project-state`, `create`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/project-states/`, `{"name":"Example","color":"#3f76ff"}`},
		{`project-state`, `update`, `v2`, ``, `PATCH`, `/api/v2/workspaces/acme/project-states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sequence":0}`},
		{`project-state`, `delete`, `v2`, ``, `DELETE`, `/api/v2/workspaces/acme/project-states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`project-state`, `mark-default`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/project-states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/mark-default/`, ``},
		{`project-label`, `list`, `v2`, ``, `GET`, `/api/v2/workspaces/acme/project-labels/`, ``},
		{`project-label`, `get`, `v2`, ``, `GET`, `/api/v2/workspaces/acme/project-labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`project-label`, `create`, `v2`, ``, `POST`, `/api/v2/workspaces/acme/project-labels/`, `{"name":"Example"}`},
		{`project-label`, `update`, `v2`, ``, `PATCH`, `/api/v2/workspaces/acme/project-labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sort_order":0}`},
		{`project-label`, `delete`, `v2`, ``, `DELETE`, `/api/v2/workspaces/acme/project-labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`project-label`, `create`, `v1`, ``, `POST`, `/api/v1/workspaces/acme/project-labels/`, `{"name":"Example"}`},
		{`project-label`, `list`, `v1`, ``, `GET`, `/api/v1/workspaces/acme/project-labels/`, ``},
		{`project-label`, `get`, `v1`, ``, `GET`, `/api/v1/workspaces/acme/project-labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`project-label`, `update`, `v1`, ``, `PATCH`, `/api/v1/workspaces/acme/project-labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sort_order":0}`},
		{`project-label`, `delete`, `v1`, ``, `DELETE`, `/api/v1/workspaces/acme/project-labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`state`, `create`, `v1`, `project`, `POST`, `/api/v1/workspaces/acme/projects/ProjectID/states/`, `{"name":"Example","color":"#3f76ff"}`},
		{`state`, `list`, `v1`, `project`, `GET`, `/api/v1/workspaces/acme/projects/ProjectID/states/`, ``},
		{`state`, `get`, `v1`, `project`, `GET`, `/api/v1/workspaces/acme/projects/ProjectID/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`state`, `update`, `v1`, `project`, `PATCH`, `/api/v1/workspaces/acme/projects/ProjectID/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sequence":0,"default":false,"is_triage":false}`},
		{`state`, `delete`, `v1`, `project`, `DELETE`, `/api/v1/workspaces/acme/projects/ProjectID/states/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`label`, `create`, `v1`, ``, `POST`, `/api/v1/workspaces/acme/projects/ProjectID/labels/`, `{"name":"Example"}`},
		{`label`, `list`, `v1`, ``, `GET`, `/api/v1/workspaces/acme/projects/ProjectID/labels/`, ``},
		{`label`, `get`, `v1`, ``, `GET`, `/api/v1/workspaces/acme/projects/ProjectID/labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
		{`label`, `update`, `v1`, ``, `PATCH`, `/api/v1/workspaces/acme/projects/ProjectID/labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, `{"description":"","sort_order":0}`},
		{`label`, `delete`, `v1`, ``, `DELETE`, `/api/v1/workspaces/acme/projects/ProjectID/labels/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/`, ``},
	}
	for _, tt := range cases {
		t.Run(tt.version+"/"+tt.resource+"/"+tt.scope+"/"+tt.action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", tt.version)
			t.Setenv("PLANE_CLI_API_KEY", "state-label-canary")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tt.method || r.URL.RequestURI() != "/prefix"+tt.path {
					t.Errorf("request: %s %s", r.Method, r.URL.RequestURI())
				}
				if r.Header.Get("X-Api-Key") != "state-label-canary" {
					t.Error("missing API credential")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if tt.body == "" {
					if len(body) != 0 {
						t.Errorf("unexpected body: %s", body)
					}
				} else {
					var got, want any
					if err := json.Unmarshal(body, &got); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal([]byte(tt.body), &want); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("body: %s; want %s", body, tt.body)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if calls > 1 {
					w.WriteHeader(http.StatusConflict)
					_, _ = io.WriteString(w, `{"detail":"state-private-canary"}`)
					return
				}
				if tt.action == "delete" {
					w.WriteHeader(204)
					return
				}
				if tt.action == "create" {
					w.WriteHeader(201)
				}
				if strings.HasPrefix(tt.action, "bulk-") {
					result := map[string]string{"bulk-create": "created", "bulk-update": "updated", "bulk-delete": "deleted"}[tt.action]
					_, _ = w.Write([]byte(`{"results":[{"index":0,"result":"` + result + `","id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"}],"succeeded":1,"failed":0}`))
					return
				}
				data := `{"id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53","unknown":9007199254740993}`
				if tt.action == "list" {
					if tt.version == "v1" {
						data = `{"results":[` + data + `],"next_cursor":"20:0:0","next_page_results":false}`
					} else {
						data = `{"data":[` + data + `],"next":null,"pagination":{"style":"offset"}}`
					}
				}
				_, _ = w.Write([]byte(data))
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			args := []string{tt.resource, tt.action, "--base-url", server.URL + "/prefix", "--workspace", "acme"}
			if tt.resource == "label" || tt.scope == "project" {
				args = append(args, "--project", "ProjectID")
			}
			if tt.scope != "" {
				args = append(args, "--scope", tt.scope)
			}
			if tt.action == "get" || tt.action == "update" || tt.action == "delete" || tt.action == "mark-default" {
				args = append(args, "--id", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53")
			}
			if tt.body != "" {
				file := filepath.Join(t.TempDir(), "body.json")
				if err := os.WriteFile(file, []byte(tt.body), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--body-file", file)
			}
			if tt.action == "delete" || tt.action == "bulk-delete" {
				args = append(args, "--yes")
			}
			code, stdout, stderr := invoke(t, catalog, args, stateLabelHandlers())
			if code != 0 || calls != 1 || stderr != "" {
				t.Fatalf("result code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			if tt.action == "delete" {
				if !strings.Contains(stdout, `"data":null`) || !strings.Contains(stdout, `"http_status":204`) {
					t.Fatal(stdout)
				}
			} else if !strings.HasPrefix(tt.action, "bulk-") && !strings.Contains(stdout, `"unknown":9007199254740993`) {
				t.Fatal(stdout)
			}
			code, stdout, stderr = invoke(t, catalog, args, stateLabelHandlers())
			if code != 5 || calls != 2 || stdout != "" || !strings.Contains(stderr, "api_error") || strings.Contains(stderr, "state-private-canary") {
				t.Fatalf("unsafe upstream rejection: code=%d calls=%d out=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestStateLabelLocalValidation(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_API_KEY", "state-label-canary")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Error("invalid input reached HTTP") }))
	defer server.Close()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		body string
	}{
		{[]string{"state", "list", "--project", "ProjectID"}, ""},
		{[]string{"state", "create", "--scope", "workspace"}, `{"name":"Catalog","color":"#fff"}`},
		{[]string{"state", "create", "--scope", "workspace"}, `{"name":"Catalog","color":"#fff","group":"triage"}`},
		{[]string{"state", "update", "--scope", "workspace", "--id", "state"}, `{"is_default":true}`},
		{[]string{"label", "upsert", "--project", "ProjectID"}, `{"name":"Tag"}`},
		{[]string{"state", "upsert", "--scope", "project", "--project", "ProjectID"}, `{"name":"State","color":"#fff","external_id":null,"external_source":"sync"}`},
		{[]string{"label", "bulk-create", "--project", "ProjectID"}, `{"items":[]}`},
		{[]string{"state", "bulk-delete", "--scope", "project", "--project", "ProjectID", "--yes"}, `{"ids":["not-a-uuid"]}`},
		{[]string{"project-label", "delete", "--id", "label"}, ""},
		{[]string{"project-state", "create"}, `{"name":"Stage","color":"#fff","group":"started"}`},
		{[]string{"label", "create", "--project", "ProjectID"}, `{"name":"Tag","name":"Duplicate"}`},
		{[]string{"label", "create", "--project", "ProjectID"}, `{"name":"Tag","unknown":true}`},
		{[]string{"label", "create", "--project", "ProjectID", "--name", "Conflicting"}, `{"name":"Tag"}`},
	}
	for _, tt := range cases {
		args := append(append([]string{}, tt.args...), "--workspace", "acme", "--base-url", server.URL)
		if tt.body != "" {
			file := filepath.Join(t.TempDir(), "body.json")
			if err := os.WriteFile(file, []byte(tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			args = append(args, "--body-file", file)
		}
		code, stdout, stderr := invoke(t, catalog, args, stateLabelHandlers())
		if code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v: code=%d stdout=%s stderr=%s", tt.args, code, stdout, stderr)
		}
	}
	if calls != 0 {
		t.Fatalf("sent %d requests", calls)
	}
}

func TestStateLabelNativeBulkPartial(t *testing.T) {
	for _, resource := range []string{"state", "label"} {
		t.Run(resource, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			t.Setenv("PLANE_CLI_API_KEY", "state-label-canary")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"results":[{"index":0,"result":"updated","id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53"},{"index":1,"result":"failed","type":"not_found","code":"not_found","detail":"No match."}],"succeeded":1,"failed":1}`))
			}))
			defer server.Close()
			body := `{"items":[{"id":"b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53","description":""},{"id":"9c1f0b3d-6d2e-4c9a-8b41-2f7e5a0d6c88","description":""}]}`
			file := filepath.Join(t.TempDir(), "body.json")
			if err := os.WriteFile(file, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			args := []string{resource, "bulk-update", "--workspace", "acme", "--project", "ProjectID", "--base-url", server.URL, "--body-file", file}
			if resource == "state" {
				args = append(args, "--scope", "project")
			}
			code, stdout, stderr := invoke(t, catalog, args, stateLabelHandlers())
			if code != 6 || calls != 1 || !strings.Contains(stdout, `"status":"partial"`) || !strings.Contains(stdout, `"index":1`) || stderr == "" {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestStateLabelNativePagination(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, resource := range []string{"state", "label"} {
			t.Run(version+"/"+resource, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_VERSION", version)
				t.Setenv("PLANE_CLI_API_KEY", "state-label-canary")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					query := r.URL.Query()
					if query.Get("per_page") != "1" {
						t.Error("lost page size")
					}
					if version == "v1" {
						if query.Get("fields") != "id,name" || query.Get("expand") != "created_by" {
							t.Error("lost v1 response shaping")
						}
						if calls == 2 && query.Get("cursor") != "1:1:0" {
							t.Error("lost v1 cursor")
						}
					} else {
						if !query.Has("count") || query.Get("count") != "false" || query.Get("search") != "Review" {
							t.Error("lost v2 filters or explicit false")
						}
						if calls == 2 && query.Get("offset") != "7" {
							t.Error("computed offset instead of native next")
						}
					}
					w.Header().Set("Content-Type", "application/json")
					if version == "v1" {
						if calls == 1 {
							_, _ = w.Write([]byte(`{"results":[{"id":"first"}],"next_cursor":"1:1:0","next_page_results":true}`))
						} else {
							_, _ = w.Write([]byte(`{"results":[{"id":"second"}],"next_cursor":"1:1:0","next_page_results":false}`))
						}
					} else {
						if calls == 1 {
							_, _ = w.Write([]byte(`{"data":[{"id":"first"}],"next":7,"pagination":{"style":"offset"}}`))
						} else {
							_, _ = w.Write([]byte(`{"data":[{"id":"second"}],"next":null,"pagination":{"style":"offset"}}`))
						}
					}
				}))
				defer server.Close()
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				args := []string{resource, "list", "--workspace", "acme", "--project", "ProjectID", "--base-url", server.URL, "--per-page", "1", "--all"}
				if resource == "state" {
					args = append(args, "--scope", "project")
				}
				if version == "v1" {
					args = append(args, "--fields", "id,name", "--expand", "created_by")
				} else {
					args = append(args, "--count=false", "--search", "Review")
				}
				code, stdout, stderr := invoke(t, catalog, args, stateLabelHandlers())
				if code != 0 || stderr != "" || calls != 2 || !strings.Contains(stdout, `"id":"first"`) || !strings.Contains(stdout, `"id":"second"`) || !strings.Contains(stdout, `"complete":true`) {
					t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
				}
			})
		}
	}
}

func TestStateLabelSparseTableAndDryRun(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_API_KEY", "state-label-canary")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v2/workspaces/acme/states/" || r.URL.Query().Get("fields") != "name" {
			t.Errorf("request %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"sparse","name":"In Review"}],"next":null,"pagination":{"style":"offset"}}`))
	}))
	defer server.Close()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := invoke(t, catalog, []string{"state", "list", "--scope", "workspace", "--workspace", "acme", "--base-url", server.URL, "--fields", "name", "--output", "table"}, stateLabelHandlers())
	if code != 0 || stderr != "" || calls != 1 || !strings.Contains(stdout, "GROUP") || !strings.Contains(stdout, "In Review") {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
	t.Setenv("PLANE_CLI_API_KEY", "")
	code, stdout, stderr = invoke(t, catalog, []string{"project-label", "delete", "--workspace", "acme", "--id", "label", "--base-url", server.URL, "--dry-run"}, stateLabelHandlers())
	if code != 0 || stderr != "" || calls != 1 || !strings.Contains(stdout, `"requires_confirmation":true`) {
		t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
	}
}
