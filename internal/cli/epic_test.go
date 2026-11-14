package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestEpicWireOperations(t *testing.T) {
	for _, tc := range []struct{ action, method, suffix, body, query string }{
		{"create", "POST", "", `{"name":"Epic","assignee_ids":[],"description_html":""}`, ""},
		{"list", "GET", "", "", "cursor=opaque&per_page=7"},
		{"get", "GET", "/epic-id", "", "fields=id%2Cname"},
		{"update", "PATCH", "/epic-id", `{"priority":"none","label_ids":[]}`, ""},
		{"delete", "DELETE", "/epic-id", "", ""},
		{"add-work-items", "POST", "/epic-id/issues", `{"work_item_ids":["item-id"]}`, ""},
		{"list-work-items", "GET", "/epic-id/issues", "", "cursor=opaque&per_page=7"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v1")
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != "/api/v1/workspaces/example/projects/project-id/epics"+tc.suffix+"/" || r.URL.RawQuery != tc.query || r.Header.Get("X-API-Key") != "fixture-key" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if tc.body != "" {
					var got, want any
					if json.Unmarshal(body, &got) != nil {
						t.Errorf("invalid JSON: %s", body)
					}
					_ = json.Unmarshal([]byte(tc.body), &want)
					a, _ := json.Marshal(got)
					b, _ := json.Marshal(want)
					if string(a) != string(b) {
						t.Errorf("body %s != %s", a, b)
					}
				} else if len(body) != 0 {
					t.Errorf("unexpected body %s", body)
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.action == "delete" {
					w.WriteHeader(204)
					return
				}
				_, _ = io.WriteString(w, `{"id":"epic-id","unknown":9007199254740993}`)
			}))
			defer server.Close()
			args := []string{"epic", tc.action, "--workspace", "example", "--project", "project-id", "--base-url", server.URL}
			if tc.suffix != "" {
				args = append(args, "--id", "epic-id")
			}
			if tc.action == "delete" {
				args = append(args, "--yes")
			}
			if tc.query == "fields=id%2Cname" {
				args = append(args, "--fields", "id,name")
			} else if tc.query != "" {
				args = append(args, "--cursor", "opaque", "--per-page", "7")
			}
			if tc.body != "" {
				path := filepath.Join(t.TempDir(), "body.json")
				if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--body-file", path)
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, out, stderr)
			}
			if tc.action != "delete" && !strings.Contains(out, "9007199254740993") {
				t.Fatalf("native precision lost: %s", out)
			}
		})
	}
}
