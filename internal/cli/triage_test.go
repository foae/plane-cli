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

func TestTriageWireOperations(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, group := range []string{"intake", "recurring-work-item", "work-item-relation"} {
			if version == "v1" && group == "recurring-work-item" || version == "v2" && group == "work-item-relation" {
				continue
			}
			actions := []string{"list", "get", "create", "update", "delete"}
			if group == "work-item-relation" {
				actions = []string{"list", "create", "remove"}
			}
			for _, action := range actions {
				t.Run(version+"/"+group+"/"+action, func(t *testing.T) {
					transportEnv(t)
					t.Setenv("PLANE_CLI_API_VERSION", version)
					t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
					method, path, body := "GET", "/api/"+version+"/workspaces/example/projects/project-id/", ""
					switch group {
					case "intake":
						path += "intake-issues/"
					case "recurring-work-item":
						path += "recurring-work-items/"
					case "work-item-relation":
						path += "work-items/item-id/relations/"
					}
					if action == "get" || action == "update" || action == "delete" {
						path += "target-id/"
					}
					switch action {
					case "create", "update":
						method = "POST"
						if action == "update" {
							method = "PATCH"
						}
						switch group {
						case "intake":
							body = `{"name":"literal <script>x</script>","status":0,"duplicate_to_id":null}`
							if version == "v1" {
								body = `{"issue":{"name":"literal <script>x</script>","priority":"none"}}`
							}
						case "recurring-work-item":
							body = `{"name":"Repeat","start_at":"2026-11-14","end_at":null,"interval_type":"weekly","interval_count":2,"assignee_ids":[],"description_html":"<p>literal</p>"}`
						case "work-item-relation":
							body = `{"relation_type":"blocking","issues":["related-id"]}`
						}
					case "delete":
						method = "DELETE"
					case "remove":
						method, path, body = "POST", path+"remove/", `{"related_issue":"related-id"}`
					}
					query := ""
					if version == "v2" && group == "intake" && action == "list" {
						query = "status__in=-2%2C0"
					}
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						if r.Method != method || r.URL.Path != path || r.URL.RawQuery != query || r.Header.Get("X-API-Key") != "fixture-key" {
							t.Errorf("unexpected request %s %s", r.Method, r.URL)
						}
						data, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						if body == "" {
							if len(data) != 0 {
								t.Errorf("unexpected body %s", data)
							}
						} else {
							var got, want any
							if err := json.Unmarshal(data, &got); err != nil {
								t.Error(err)
							}
							if err := json.Unmarshal([]byte(body), &want); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got, want) {
								t.Errorf("body %s != %s", data, body)
							}
						}
						w.Header().Set("Content-Type", "application/json")
						if calls == 2 {
							w.WriteHeader(http.StatusNotFound)
							_, _ = io.WriteString(w, `{"code":"not_found","detail":"Missing"}`)
							return
						}
						if action == "delete" || action == "remove" {
							w.WriteHeader(204)
							return
						}
						if action == "create" {
							w.WriteHeader(201)
						}
						_, _ = io.WriteString(w, `{"id":"target-id","unknown":9007199254740993}`)
					}))
					defer server.Close()
					args := []string{group, action, "--workspace", "example", "--project", "project-id", "--base-url", server.URL}
					if query != "" {
						args = append(args, "--status-in=-2,0")
					}
					if group == "work-item-relation" {
						args = append(args, "--work-item", "item-id")
					}
					if action == "get" || action == "update" || action == "delete" {
						args = append(args, "--id", "target-id")
					}
					if action == "delete" || action == "remove" {
						args = append(args, "--yes")
					}
					if body != "" {
						file := filepath.Join(t.TempDir(), "body.json")
						if err := os.WriteFile(file, []byte(body), 0600); err != nil {
							t.Fatal(err)
						}
						args = append(args, "--body-file", file)
					}
					catalog, err := spec.Load()
					if err != nil {
						t.Fatal(err)
					}
					code, out, stderr := invoke(t, catalog, args, nil)
					if code != 0 || stderr != "" || calls != 1 {
						t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
					}
					if action != "delete" && action != "remove" && !strings.Contains(out, "9007199254740993") {
						t.Fatalf("native data lost: %s", out)
					}
					code, out, stderr = invoke(t, catalog, args, nil)
					if code != 5 || out != "" || calls != 2 {
						t.Fatalf("rejection lost: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
					}
				})
			}
		}
	}
}

func TestTriageInvalidBodiesNeverSend(t *testing.T) {
	for _, tc := range []struct{ group, body string }{
		{"recurring-work-item", `{"name":"Repeat","start_at":"2026-11-14","interval_count":0}`},
		{"recurring-work-item", `{"name":"Repeat","start_at":"2026-11-14","enabled":false}`},
		{"recurring-work-item", `{"name":"Repeat"}`},
		{"intake", `{"status":3}`},
		{"intake", `{"issue":{"name":"v1 field"}}`},
	} {
		t.Run(tc.body, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			t.Setenv("PLANE_CLI_BASE_URL", "http://127.0.0.1:1")
			file := filepath.Join(t.TempDir(), "body.json")
			if err := os.WriteFile(file, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, []string{tc.group, "create", "--workspace", "example", "--project", "p", "--body-file", file, "--dry-run"}, nil)
			if code != 2 {
				t.Fatalf("invalid input accepted: code=%d out=%s stderr=%s", code, out, stderr)
			}
		})
	}
}
