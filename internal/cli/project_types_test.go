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

type projectTypesWireCase struct {
	Version   string            `json:"version"`
	Group     string            `json:"group"`
	Action    string            `json:"action"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     string            `json:"query"`
	Parents   map[string]string `json:"parents"`
	ExtraArgs []string          `json:"extra_args"`
	Body      json.RawMessage   `json:"body"`
	Status    int               `json:"status"`
	Response  json.RawMessage   `json:"response"`
}

func projectTypesBodyFile(t *testing.T, body []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestProjectTypesWireOperations(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"v1", "v2"} {
		data, err := os.ReadFile("testdata/p05-project-" + version + "-wire.json")
		if err != nil {
			t.Fatal(err)
		}
		var cases []projectTypesWireCase
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			t.Run(tc.Version+"/"+tc.Group+"/"+tc.Action, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_VERSION", tc.Version)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != tc.Method || r.URL.Path != tc.Path || r.URL.RawQuery != tc.Query || r.Header.Get("X-API-Key") != "fixture-key" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL)
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					if string(tc.Body) == "null" {
						if len(body) != 0 {
							t.Errorf("unexpected body %s", body)
						}
					} else {
						got, err := spec.Decode(body)
						if err != nil {
							t.Error(err)
						}
						want, err := spec.Decode(tc.Body)
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Errorf("body %s != %s", body, tc.Body)
						}
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.Status)
					if tc.Status != 204 {
						_, _ = w.Write(tc.Response)
					}
				}))
				defer server.Close()
				args := []string{tc.Group, tc.Action, "--workspace", "example", "--project", "project-id", "--base-url", server.URL}
				for _, parent := range []string{"type", "property", "work-item", "id"} {
					if value, ok := tc.Parents[parent]; ok {
						args = append(args, "--"+parent, value)
					}
				}
				args = append(args, tc.ExtraArgs...)
				if tc.Action == "delete" || tc.Action == "detach" {
					args = append(args, "--yes")
				}
				if string(tc.Body) != "null" {
					args = append(args, "--body-file", projectTypesBodyFile(t, tc.Body))
				}
				code, out, stderr := invoke(t, catalog, args, projectTypesHandlers())
				if code != 0 || calls != 1 || stderr != "" {
					t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
				}
				decoded, err := spec.Decode([]byte(out))
				if err != nil {
					t.Fatal(err)
				}
				want, err := spec.Decode(tc.Response)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decoded.(map[string]any)["data"], want) {
					t.Fatalf("native response changed: %s", out)
				}
			})
		}
	}
}

func TestProjectTypesValidation(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		version, group, action, body string
		parents                      []string
		valid                        bool
	}{
		{"v2", "work-item-type", "create", `{"name":"Bug","is_epic":true}`, nil, false},
		{"v2", "work-item-type", "update", `{"is_default":true}`, []string{"--id", "target-id"}, false},
		{"v2", "work-item-type", "create", `{"name":"Bug","logo_props":{}}`, nil, false},
		{"v2", "work-item-type", "create", `{"name":"Bug","level":1}`, nil, false},
		{"v2", "work-item-type", "create", `{"name":"Bug","governance_mode":"any"}`, nil, false},
		{"v2", "work-item-type", "create", `{"name":"Bug","rollout":"seed"}`, nil, false},
		{"v2", "work-item-type", "create", `{"name":"Bug","project_ids":[]}`, nil, false},
		{"v2", "work-item-type", "create", `{"name":"Bug","created_at":"now"}`, nil, false},
		{"v2", "work-item-property", "create", `{"name":"readonly","display_name":"X","property_type":"TEXT"}`, nil, false},
		{"v2", "work-item-property", "create", `{"display_name":"X","property_type":"INVALID"}`, nil, false},
		{"v1", "work-item-property", "create", `{"display_name":"X","property_type":"RELATION","relation_type":"RELEASE"}`, []string{"--type", "type-id"}, false},
		{"v2", "work-item-property", "create", `{"display_name":"X","property_type":"DECIMAL","default_value":3}`, nil, false},
		{"v2", "work-item-property", "create", `{"display_name":"X","property_type":"DECIMAL","default_value":[3]}`, nil, false},
		{"v2", "work-item-property", "create", `{"display_name":"X","property_type":"DECIMAL","default_value":["3"],"settings":{"unknown":[null,0,false]},"validation_rules":{"arbitrary":true}}`, nil, true},
		{"v2", "work-item-property", "create", `{"display_name":"X","property_type":"OPTION","options":[{}]}`, nil, false},
		{"v2", "work-item-property", "create", `{"display_name":"X","property_type":"OPTION","options":[{"name":"X","id":"readonly"}]}`, nil, false},
		{"v2", "work-item-property-option", "create", `{"name":"X","sort_order":1}`, []string{"--property", "property-id"}, false},
		{"v2", "work-item-property-option", "create", `{"name":"X","is_active":false}`, []string{"--property", "property-id"}, false},
		{"v2", "work-item-property-option", "create", `{"name":"X","external_id":null,"external_source":null,"is_default":false}`, []string{"--property", "property-id"}, true},
		{"v2", "work-item-type-property", "attach", `{"properties":[]}`, []string{"--type", "type-id"}, false},
		{"v2", "work-item-type-property", "attach", `{"properties":["property-id"],"is_required":true}`, []string{"--type", "type-id"}, false},
		{"v1", "work-item-property-value", "upsert", `{"value":null}`, []string{"--work-item", "item-id", "--property", "property-id"}, false},
		{"v1", "work-item-property-value", "upsert", `{"value":{}}`, []string{"--work-item", "item-id", "--property", "property-id"}, false},
		{"v1", "work-item-property-value", "upsert", `{"value":[1]}`, []string{"--work-item", "item-id", "--property", "property-id"}, false},
		{"v1", "work-item-property-value", "upsert", `{}`, []string{"--work-item", "item-id", "--property", "property-id"}, false},
	} {
		t.Run(tc.version+"/"+tc.group+"/"+tc.body, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", tc.version)
			t.Setenv("PLANE_CLI_BASE_URL", "http://127.0.0.1:1")
			args := []string{tc.group, tc.action, "--workspace", "example", "--project", "project-id", "--body-file", projectTypesBodyFile(t, []byte(tc.body)), "--dry-run"}
			args = append(args, tc.parents...)
			code, out, stderr := invoke(t, catalog, args, projectTypesHandlers())
			wantCode := 2
			if tc.valid {
				wantCode = 0
			}
			if code != wantCode {
				t.Fatalf("code=%d want=%d out=%s stderr=%s", code, wantCode, out, stderr)
			}
		})
	}
}

func TestProjectTypesModeConflictNoRetry(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v2/workspaces/example/projects/project-id/work-item-types/" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		_, _ = io.WriteString(w, `{"type":"conflict","code":"work_item_types_managed_at_workspace","detail":"Work item types are managed at the workspace level for this workspace."}`)
	}))
	defer server.Close()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"work-item-type", "create", "--workspace", "example", "--project", "project-id", "--base-url", server.URL, "--body-file", projectTypesBodyFile(t, []byte(`{"name":"Bug"}`))}
	code, out, stderr := invoke(t, catalog, args, projectTypesHandlers())
	if code != 5 || calls != 1 || out != "" || !strings.Contains(stderr, `"http_status":409`) {
		t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
	}
}

func TestProjectTypesConfirmationAndAvailability(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		version string
		args    []string
		code    int
	}{
		{"v2", []string{"work-item-type", "delete", "--id", "target-id"}, 2},
		{"v2", []string{"work-item-property", "delete", "--id", "target-id"}, 2},
		{"v2", []string{"work-item-property-option", "delete", "--id", "target-id", "--property", "property-id"}, 2},
		{"v2", []string{"work-item-type-property", "detach", "--id", "target-id", "--type", "type-id"}, 2},
		{"v1", []string{"work-item-property-value", "delete", "--work-item", "item-id", "--property", "property-id"}, 2},
		{"v2", []string{"work-item-property-value", "get", "--work-item", "item-id", "--property", "property-id"}, 7},
		{"v1", []string{"work-item-type", "enable"}, 7},
		{"v1", []string{"work-item-type-property", "list", "--type", "type-id"}, 7},
		{"v1", []string{"work-item-property-option", "list", "--property", "property-id", "--all"}, 2},
	} {
		t.Run(tc.version+"/"+strings.Join(tc.args, " "), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", tc.version)
			t.Setenv("PLANE_CLI_BASE_URL", "http://127.0.0.1:1")
			args := append(append([]string{}, tc.args...), "--workspace", "example", "--project", "project-id")
			code, out, stderr := invoke(t, catalog, args, projectTypesHandlers())
			if code != tc.code || out != "" {
				t.Fatalf("code=%d want=%d out=%s stderr=%s", code, tc.code, out, stderr)
			}
		})
	}
}

func TestProjectTypeSchemaInformsMutation(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	const project = "11111111-1111-4111-8111-111111111111"
	const schema = `{"custom_fields":{"server_choices":{"type":"option","required":true,"is_multi":true,"options":[{"id":"option-a","name":"A"},{"id":"option-b","name":"B"}]},"server_note":{"type":"string","required":false,"is_multi":false}}}`
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Api-Key") != "fixture-key" {
			t.Error("missing API credential")
		}
		switch calls {
		case 1:
			if r.Method != "GET" || r.URL.Path != "/api/v2/workspaces/example/projects/"+project+"/work-item-types/type-id/schema/" {
				t.Errorf("unexpected schema request: %s %s", r.Method, r.URL)
			}
			_, _ = io.WriteString(w, schema)
		case 2:
			if r.Method != "PATCH" || r.URL.Path != "/api/v2/workspaces/example/projects/"+project+"/work-items/ENG-12/" {
				t.Errorf("unexpected mutation: %s %s", r.Method, r.URL)
			}
			body, _ := io.ReadAll(r.Body)
			got, err := spec.Decode(body)
			want, _ := spec.Decode([]byte(`{"custom_fields":{"server_choices":["option-a","option-b"],"server_note":null}}`))
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("mutation changed values or added omitted fields: %s (%v)", body, err)
			}
			_, _ = io.WriteString(w, `{"id":"ENG-12"}`)
		default:
			t.Errorf("hidden request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	t.Setenv("PLANE_CLI_BASE_URL", server.URL)
	code, out, stderr := invoke(t, catalog, []string{"work-item-type", "get-schema", "--workspace", "example", "--project", project, "--id", "type-id"}, projectTypesHandlers())
	if code != 0 {
		t.Fatalf("schema: code=%d out=%s stderr=%s", code, out, stderr)
	}
	envelope, err := spec.Decode([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	fields := envelope.(map[string]any)["data"].(map[string]any)["custom_fields"].(map[string]any)
	values := map[string]any{}
	for name, raw := range fields {
		field := raw.(map[string]any)
		if field["required"] == true && field["is_multi"] == true {
			ids := []any{}
			for _, option := range field["options"].([]any) {
				ids = append(ids, option.(map[string]any)["id"])
			}
			values[name] = ids
		} else {
			values[name] = nil
		}
	}
	body, err := json.Marshal(map[string]any{"custom_fields": values})
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr = invoke(t, catalog, []string{"work-item", "update", "--workspace", "example", "--project", project, "--id", "ENG-12", "--body-file", projectTypesBodyFile(t, body)}, workItemHandlers())
	if code != 0 || calls != 2 {
		t.Fatalf("mutation: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
	}
	for _, group := range []string{"work-item-type", "work-item-property", "work-item-property-option", "work-item-type-property", "workspace-work-item-type", "workspace-work-item-property", "workspace-work-item-property-option", "workspace-work-item-type-property", "work-item-property-context"} {
		code, out, stderr = invoke(t, catalog, []string{group, "list", "--all", "--max-items", "1"}, projectTypesHandlers())
		if code != 2 || calls != 2 {
			t.Fatalf("%s ignored unsupported limit: code=%d calls=%d out=%s stderr=%s", group, code, calls, out, stderr)
		}
	}
}
