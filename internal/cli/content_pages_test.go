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

type contentPageWireCase struct {
	Version  string            `json:"version"`
	Group    string            `json:"group"`
	Action   string            `json:"action"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Query    string            `json:"query"`
	Parents  map[string]string `json:"parents"`
	Body     json.RawMessage   `json:"body"`
	Status   int               `json:"status"`
	Response json.RawMessage   `json:"response"`
}

func contentPageCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func contentPageBody(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContentPagesWireOperations(t *testing.T) {
	catalog := contentPageCatalog(t)
	data, err := os.ReadFile("testdata/p06-page-wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []contentPageWireCase
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
			args := []string{tc.Group, tc.Action, "--workspace", "example", "--base-url", server.URL}
			for _, parent := range []string{"project", "work-item", "id", "anchor"} {
				if value, ok := tc.Parents[parent]; ok {
					args = append(args, "--"+parent, value)
				}
			}
			if tc.Action == "delete" || tc.Action == "archive" || tc.Action == "restore" {
				args = append(args, "--yes")
			}
			if string(tc.Body) != "null" {
				args = append(args, "--body-file", contentPageBody(t, tc.Body))
			}
			code, out, stderr := invoke(t, catalog, args, contentPagesHandlers())
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

func TestContentPagesSafeguardsAndValidation(t *testing.T) {
	catalog := contentPageCatalog(t)
	for _, tc := range []struct {
		version, group, action, body string
		extra                        []string
		code                         int
	}{
		{"v1", "workspace-page", "update", `{}`, nil, 2},
		{"v1", "project-page", "update", `{"description_html":null}`, []string{"--project", "ENG"}, 2},
		{"v1", "workspace-page", "update", `{"description":"not-a-write-field"}`, nil, 2},
		{"v1", "workspace-page", "update", `{"description_html":"<p>Native HTML</p>"}`, []string{"--dry-run"}, 0},
		{"v2", "work-item-page", "create", `{"page_id":"not-a-uuid"}`, []string{"--project", "550e8400-e29b-41d4-a716-446655440000", "--work-item", "550e8400-e29b-41d4-a716-446655440001"}, 2},
		{"v2", "work-item-page", "delete", "", []string{"--project", "ENG", "--work-item", "ENG-123"}, 2},
		{"v1", "workspace-page", "archive", "", nil, 2},
		{"v2", "workspace-page", "archive", "", nil, 2},
		{"v2", "workspace-page", "restore", "", nil, 2},
		{"v1", "workspace-page", "delete", "", nil, 2},
		{"v2", "sticky", "delete", "", nil, 2},
		{"v2", "workspace-page", "list", "", nil, 7},
		{"v2", "sticky", "list", "", nil, 7},
		{"v1", "work-item-page", "get", "", []string{"--project", "ENG", "--work-item", "ENG-123"}, 7},
		{"v2", "workspace-page", "update", `{"name":"Not implemented"}`, nil, 7},
	} {
		t.Run(tc.version+"/"+tc.group+"/"+tc.action+"/"+tc.body, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", tc.version)
			t.Setenv("PLANE_CLI_BASE_URL", "http://127.0.0.1:1")
			args := []string{tc.group, tc.action, "--workspace", "example"}
			if tc.action != "create" && tc.action != "list" {
				args = append(args, "--id", "550e8400-e29b-41d4-a716-446655440000")
			}
			if tc.body != "" {
				args = append(args, "--body-file", contentPageBody(t, []byte(tc.body)))
			}
			args = append(args, tc.extra...)
			code, out, stderr := invoke(t, catalog, args, contentPagesHandlers())
			if code != tc.code {
				t.Fatalf("code=%d want=%d out=%s stderr=%s", code, tc.code, out, stderr)
			}
		})
	}
}

func TestContentPagesRestoreConfirmation(t *testing.T) {
	catalog := contentPageCatalog(t)
	for _, version := range []string{"v1", "v2"} {
		for _, group := range []string{"workspace-page", "project-page"} {
			t.Run(version+"/"+group, func(t *testing.T) {
				transportEnv(t)
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusNoContent)
				}))
				defer server.Close()
				args := []string{group, "restore", "--api-version", version, "--workspace", "example", "--id", "550e8400-e29b-41d4-a716-446655440000", "--base-url", server.URL}
				if group == "project-page" {
					args = append(args, "--project", "550e8400-e29b-41d4-a716-446655440001")
				}
				code, out, stderr := invoke(t, catalog, args, contentPagesHandlers())
				if code != 2 || calls != 0 || out != "" {
					t.Fatalf("confirmation code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
				}
				code, out, stderr = invoke(t, catalog, append(args, "--dry-run"), contentPagesHandlers())
				if code != 0 || calls != 0 || stderr != "" || !strings.Contains(out, `"requires_confirmation":true`) {
					t.Fatalf("dry-run code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
				}
			})
		}
	}
}

func TestContentPagesExplicitDependenciesNoRetry(t *testing.T) {
	for _, action := range []string{"delete", "restore", "archive"} {
		t.Run(action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"type":"invalid_request","code":"invalid_request","detail":"The parent page should be restored first."}`)
			}))
			defer server.Close()
			code, out, stderr := invoke(t, contentPageCatalog(t), []string{"workspace-page", action, "--workspace", "example", "--id", "550e8400-e29b-41d4-a716-446655440000", "--base-url", server.URL, "--yes"}, contentPagesHandlers())
			if code != 5 || calls != 1 || !strings.Contains(stderr, `"http_status":400`) {
				t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestContentPagesV1Aggregate(t *testing.T) {
	for _, group := range []string{"workspace-page", "project-page", "work-item-page"} {
		t.Run(group, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v1")
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("per_page") != "1" {
					t.Errorf("lost page size: %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					_, _ = io.WriteString(w, `{"results":[{"id":"one","description_html":"<p>Original</p>"}],"next_cursor":"opaque-next","next_page_results":true}`)
				} else {
					if r.URL.Query().Get("cursor") != "opaque-next" {
						t.Errorf("lost native continuation: %s", r.URL)
					}
					_, _ = io.WriteString(w, `{"results":[{"id":"two","logo_props":{"opaque":[null,false,2]}}],"next_cursor":"","next_page_results":false}`)
				}
			}))
			defer server.Close()
			args := []string{group, "list", "--workspace", "example", "--base-url", server.URL, "--per-page", "1", "--all"}
			if group != "workspace-page" {
				args = append(args, "--project", "project-id")
			}
			if group == "work-item-page" {
				args = append(args, "--work-item", "work-item-id")
			}
			code, out, stderr := invoke(t, contentPageCatalog(t), args, contentPagesHandlers())
			if code != 0 || calls != 2 || !strings.Contains(out, "Original") || !strings.Contains(out, "opaque") {
				t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestContentPagesNativeQueryWire(t *testing.T) {
	for _, tc := range []struct {
		version string
		args    []string
		query   string
	}{
		{"v2", []string{"workspace-page", "get", "--id", "550e8400-e29b-41d4-a716-446655440000", "--fields", "id,description_html", "--expand", "parent"}, "expand=parent&fields=id%2Cdescription_html"},
		{"v2", []string{"work-item-page", "get", "--project", "ENG", "--work-item", "ENG-123", "--id", "550e8400-e29b-41d4-a716-446655440000", "--fields", "id,page_id", "--expand", "page"}, "expand=page&fields=id%2Cpage_id"},
		{"v1", []string{"workspace-page", "list", "--type", "archived", "--search", "Release notes", "--cursor", "opaque:1", "--per-page", "2"}, "cursor=opaque%3A1&per_page=2&search=Release+notes&type=archived"},
		{"v1", []string{"work-item-page", "list", "--project", "project-id", "--work-item", "work-item-id", "--order-by", "created_at", "--per-page", "2"}, "order_by=created_at&per_page=2"},
	} {
		t.Run(tc.version+"/"+tc.args[0], func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", tc.version)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.RawQuery != tc.query {
					t.Errorf("query %s != %s", r.URL.RawQuery, tc.query)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"native","description_html":"<p>Exact</p>","parent":{"opaque":[false,null]}}`)
			}))
			defer server.Close()
			args := append(append([]string{}, tc.args...), "--workspace", "example", "--base-url", server.URL)
			code, out, stderr := invoke(t, contentPageCatalog(t), args, contentPagesHandlers())
			if code != 0 || calls != 1 || !strings.Contains(out, "opaque") {
				t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}
