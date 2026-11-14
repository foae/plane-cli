package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestProjectPaginationAtomic(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, broken := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/broken=%v", version, broken), func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_VERSION", version)
				t.Setenv("PLANE_CLI_API_KEY", "project-canary")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != "GET" || r.URL.Path != "/api/"+version+"/workspaces/example/projects/" || r.Header.Get("X-Api-Key") != "project-canary" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL)
					}
					if calls == 2 && broken {
						w.WriteHeader(503)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if version == "v1" {
						if calls == 1 {
							_, _ = fmt.Fprint(w, `{"results":[{"id":"first","name":"one"}],"next_page_results":true,"next_cursor":"opaque"}`)
						} else {
							if r.URL.Query().Get("cursor") != "opaque" {
								t.Error("cursor lost")
							}
							_, _ = fmt.Fprint(w, `{"results":[{"id":"second","name":"two","unknown":9007199254740993}],"next_page_results":false}`)
						}
					} else {
						if calls == 1 {
							_, _ = fmt.Fprint(w, `{"data":[{"id":"first","name":"one"}],"pagination":{"style":"offset"},"next":1}`)
						} else {
							if r.URL.Query().Get("offset") != "1" {
								t.Error("offset lost")
							}
							_, _ = fmt.Fprint(w, `{"data":[{"id":"second","name":"two","unknown":9007199254740993}],"pagination":{"style":"offset"},"next":null}`)
						}
					}
				}))
				defer server.Close()
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				code, out, stderr := invoke(t, catalog, []string{"project", "list", "--workspace", "example", "--base-url", server.URL, "--all"}, nil)
				if broken {
					if code == 0 || out != "" {
						t.Fatalf("partial leak: %d %s %s", code, out, stderr)
					}
					return
				}
				if code != 0 || stderr != "" || calls != 2 || !strings.Contains(out, `9007199254740993`) || !strings.Contains(out, `"pages":2`) {
					t.Fatalf("pagination: %d %s %s calls=%d", code, out, stderr, calls)
				}
			})
		}
	}
}

func TestProjectWireOperations(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		cases := []struct {
			action, method, suffix, body string
			id, yes                      bool
		}{
			{"create", "POST", "", `{"name":"Example","identifier":"EX"}`, false, false},
			{"get", "GET", "/project-id", "", true, false},
			{"update", "PATCH", "/project-id", `{"description":"","cycle_view":false}`, true, false},
			{"list", "GET", "", "", false, false},
			{"archive", "POST", "/project-id/archive", "", true, true},
			{"delete", "DELETE", "/project-id", "", true, true},
		}
		if version == "v1" {
			cases = append(cases, struct {
				action, method, suffix, body string
				id, yes                      bool
			}{"unarchive", "DELETE", "/project-id/archive", "", true, false},
				struct {
					action, method, suffix, body string
					id, yes                      bool
				}{"create-with-template", "POST", "/templates/use", `{"template_id":"template-id"}`, false, false})
		} else {
			cases = append(cases,
				struct {
					action, method, suffix, body string
					id, yes                      bool
				}{"unarchive", "POST", "/project-id/unarchive", "", true, false},
				struct {
					action, method, suffix, body string
					id, yes                      bool
				}{"summary", "GET", "/project-id/summary", "", true, false},
				struct {
					action, method, suffix, body string
					id, yes                      bool
				}{"upsert", "POST", "/upsert", `{"name":"Example","identifier":"EX","external_source":"import","external_id":"1"}`, false, false},
				struct {
					action, method, suffix, body string
					id, yes                      bool
				}{"bulk-create", "POST", "/bulk-create", `{"items":[{"name":"Example","identifier":"EX"}],"all_or_none":false}`, false, false},
				struct {
					action, method, suffix, body string
					id, yes                      bool
				}{"bulk-update", "POST", "/bulk-update", `{"items":[{"id":"project-id","name":"Renamed"}]}`, false, false})
		}
		for _, tc := range cases {
			t.Run(version+"/"+tc.action, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_VERSION", version)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != tc.method || r.URL.RequestURI() != "/api/"+version+"/workspaces/example/projects"+tc.suffix+"/" {
						t.Errorf("wire: %s %s", r.Method, r.URL)
					}
					got, _ := io.ReadAll(r.Body)
					if tc.body != "" {
						var a, b any
						if json.Unmarshal(got, &a) != nil || json.Unmarshal([]byte(tc.body), &b) != nil {
							t.Error("invalid body")
							w.WriteHeader(400)
							return
						}
						ga, _ := json.Marshal(a)
						gb, _ := json.Marshal(b)
						if string(ga) != string(gb) {
							t.Errorf("body %s != %s", ga, gb)
						}
					} else if len(got) != 0 {
						t.Errorf("unexpected body %s", got)
					}
					w.Header().Set("Content-Type", "application/json")
					if strings.HasPrefix(tc.action, "bulk-") {
						_, _ = fmt.Fprint(w, `{"results":[{"index":0,"result":{"id":"project-id"}}],"succeeded":1,"failed":0}`)
					} else if tc.action == "delete" {
						w.WriteHeader(204)
					} else {
						_, _ = fmt.Fprint(w, `{"id":"project-id","name":"Example"}`)
					}
				}))
				defer server.Close()
				args := []string{"project", tc.action, "--workspace", "example", "--base-url", server.URL}
				if tc.id {
					args = append(args, "--id", "project-id")
				}
				if tc.yes {
					args = append(args, "--yes")
				}
				if tc.body != "" {
					p := filepath.Join(t.TempDir(), "body.json")
					if err := os.WriteFile(p, []byte(tc.body), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--body-file", p)
				}
				c, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				code, out, stderr := invoke(t, c, args, nil)
				if code != 0 || stderr != "" || calls != 1 {
					t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, out, stderr)
				}
			})
		}
	}
}
