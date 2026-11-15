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

type workspaceTypeFixture struct {
	Group       string          `json:"group"`
	Action      string          `json:"action"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Body        json.RawMessage `json:"body"`
	Status      int             `json:"status"`
	Response    json.RawMessage `json:"response"`
	Destructive bool            `json:"destructive"`
}

func TestWorkspaceTypesWireOperations(t *testing.T) {
	files, err := filepath.Glob("testdata/p05-workspace-*.json")
	if err != nil || len(files) != 25 {
		t.Fatalf("fixtures: %d %v", len(files), err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var fixture workspaceTypeFixture
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatal(err)
		}
		t.Run(fixture.Group+"/"+fixture.Action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			query := ""
			if fixture.Group != "workspace-work-item-property-option" && fixture.Action != "attach" && fixture.Action != "detach" {
				query = "fields=id"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != fixture.Method || r.URL.Path != fixture.Path || r.URL.RawQuery != query {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("X-API-Key") != "fixture-key" {
					t.Error("missing credential")
				}
				got, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(fixture.Body) == "null" {
					if len(got) != 0 {
						t.Errorf("unexpected body: %s", got)
					}
				} else {
					var a, b any
					if err := json.Unmarshal(got, &a); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal(fixture.Body, &b); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(a, b) {
						t.Errorf("body changed: %s != %s", got, fixture.Body)
					}
				}
				if calls == 2 {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(409)
					_, _ = io.WriteString(w, `{"type":"conflict","code":"work_item_types_managed_at_project","detail":"Work item types are managed at the project level for this workspace."}`)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(fixture.Status)
				if fixture.Status != 204 {
					_, _ = w.Write(fixture.Response)
				}
			}))
			defer server.Close()
			args := []string{fixture.Group, fixture.Action, "--workspace", "example", "--base-url", server.URL}
			if query != "" {
				args = append(args, "--fields", "id")
			}
			if strings.Contains(fixture.Path, "/property-id/") {
				args = append(args, "--property", "property-id")
			}
			if strings.Contains(fixture.Path, "/type-id/") {
				args = append(args, "--type", "type-id")
			}
			if strings.Contains(fixture.Path, "/target-id/") {
				args = append(args, "--id", "target-id")
			}
			if string(fixture.Body) != "null" {
				bodyFile := filepath.Join(t.TempDir(), "body.json")
				if err := os.WriteFile(bodyFile, fixture.Body, 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--body-file", bodyFile)
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			if fixture.Destructive {
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 2 || calls != 0 || out != "" {
					t.Fatalf("confirmation: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
				}
				args = append(args, "--yes")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || calls != 1 || stderr != "" {
				t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(envelope.Data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(fixture.Response, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native response changed: %s", out)
			}
			code, out, stderr = invoke(t, catalog, args, nil)
			if code != 5 || calls != 2 || out != "" || !strings.Contains(stderr, `"http_status":409`) {
				t.Fatalf("409 must not retry: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestWorkspaceTypesInvalidBodiesNeverSend(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer server.Close()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ group, action, body string }{
		{"workspace-work-item-type", "create", `{"name":"Epic","is_epic":true}`},
		{"workspace-work-item-type", "update", `{"is_default":true}`},
		{"workspace-work-item-type", "update", `{"logo_props":{}}`},
		{"workspace-work-item-property", "create", `{"display_name":"Field","property_type":"INVALID"}`},
		{"workspace-work-item-property", "create", `{"display_name":"Field","property_type":"TEXT","name":"readonly"}`},
		{"workspace-work-item-property", "create", `{"display_name":"Field","property_type":"TEXT","default_value":"scalar"}`},
		{"workspace-work-item-property-option", "update", `{"sort_order":100}`},
		{"workspace-work-item-property-option", "create", `{"name":"Choice","is_active":false}`},
		{"work-item-property-context", "create", `{}`},
		{"work-item-property-context", "create", `{"applies_to_all_projects":true,"project_ids":[],"applies_to_all_work_item_types":true}`},
		{"work-item-property-context", "create", `{"project_ids":["a","a"],"issue_type_ids":["b"]}`},
		{"work-item-property-context", "create", `{"project_ids":["a"],"issue_type_ids":[]}`},
		{"work-item-property-context", "create", `{"project_ids":["a"],"issue_type_ids":["b"],"options":[{}]}`},
		{"work-item-property-context", "create", `{"project_ids":["a"],"issue_type_ids":["b"],"options":[{"name":"a","sort_order":1}]}`},
		{"work-item-property-context", "create", `{"project_ids":["a"],"issue_type_ids":["b"],"options":[{"name":"Major"},{"name":"MAJOR"}]}`},
		{"work-item-property-context", "create", `{"project_ids":["a"],"issue_type_ids":["b"],"options":[{"id":"a"},{"id":"a"}]}`},
		{"work-item-property-context", "create", `{"project_ids":["a"],"issue_type_ids":["b"],"external_id":"unpaired"}`},
		{"work-item-property-context", "update", `{"is_default":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.group+"/"+tc.action+"/"+tc.body, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "body.json")
			if err := os.WriteFile(file, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{tc.group, tc.action, "--workspace", "example", "--base-url", server.URL, "--body-file", file}
			if tc.group == "workspace-work-item-property-option" || tc.group == "work-item-property-context" {
				args = append(args, "--property", "property-id")
			}
			if tc.action == "update" {
				args = append(args, "--id", "target-id")
			}
			if tc.group == "work-item-property-context" && tc.action == "update" {
				args = append(args, "--yes")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 2 || calls != 0 || out != "" {
				t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestWorkspaceContextPatchPreservesOmission(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PATCH" || r.URL.Path != "/api/v2/workspaces/example/work-item-properties/property-id/contexts/target-id/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if string(data) != `{"is_required":false}` {
			t.Errorf("omitted fields inserted: %s", data)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"target-id","is_required":false,"is_multi":true,"default_value":["fallback"],"options":[{"id":"choice","is_default":true}],"extension":9007199254740993}`)
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(`{"is_required":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := invoke(t, catalog, []string{"work-item-property-context", "update", "--workspace", "example", "--property", "property-id", "--id", "target-id", "--body-file", file, "--yes", "--base-url", server.URL}, nil)
	if code != 0 || calls != 1 || stderr != "" || !strings.Contains(out, "9007199254740993") {
		t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
	}
}
