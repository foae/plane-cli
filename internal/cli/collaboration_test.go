package cli

import (
	"encoding/json"
	"fmt"
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

type collaborationFixture struct {
	ID       string          `json:"id"`
	Args     []string        `json:"args"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Body     json.RawMessage `json:"body"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"`
}

func collaborationFixtures(t *testing.T) []collaborationFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/p04-collaboration-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []collaborationFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func collaborationBodyFile(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestP04CollaborationRequests(t *testing.T) {
	for _, fixture := range collaborationFixtures(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			for _, rejected := range []bool{false, true} {
				t.Run(fmt.Sprintf("rejected=%v", rejected), func(t *testing.T) {
					transportEnv(t)
					t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						if r.Method != fixture.Method || r.URL.RequestURI() != fixture.Path {
							t.Errorf("request: %s %s", r.Method, r.URL.RequestURI())
						}
						raw, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						if string(fixture.Body) == "null" {
							if len(raw) != 0 {
								t.Errorf("unexpected body: %s", raw)
							}
						} else {
							var got, want any
							if json.Unmarshal(raw, &got) != nil || json.Unmarshal(fixture.Body, &want) != nil || !reflect.DeepEqual(got, want) {
								t.Errorf("body mismatch: %s want %s", raw, fixture.Body)
							}
						}
						w.Header().Set("Content-Type", "application/json")
						if rejected {
							w.WriteHeader(404)
							_, _ = io.WriteString(w, `{"type":"not_found","code":"not_found","detail":"Missing"}`)
							return
						}
						if strings.Contains(fixture.ID, ".upsert-") {
							w.Header().Set("X-Plane-Upsert", "updated")
						}
						w.WriteHeader(fixture.Status)
						if fixture.Status != 204 {
							_, _ = w.Write(fixture.Response)
						}
					}))
					defer server.Close()
					args := append(append([]string{}, fixture.Args...), "--base-url", server.URL)
					if string(fixture.Body) != "null" {
						args = append(args, "--body-file", collaborationBodyFile(t, fixture.Body))
					}
					catalog, err := spec.Load()
					if err != nil {
						t.Fatal(err)
					}
					code, out, stderr := invoke(t, catalog, args, nil)
					if rejected {
						if code != 5 || out != "" || calls != 1 {
							t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, out, stderr)
						}
						return
					}
					if code != 0 || stderr != "" || calls != 1 {
						t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, out, stderr)
					}
					var envelope struct {
						Data json.RawMessage `json:"data"`
					}
					if err := json.Unmarshal([]byte(out), &envelope); err != nil {
						t.Fatal(err)
					}
					var got, want any
					if json.Unmarshal(envelope.Data, &got) != nil || json.Unmarshal(fixture.Response, &want) != nil || !reflect.DeepEqual(got, want) {
						t.Errorf("native response changed: %s", out)
					}
				})
			}
		})
	}
}

func TestP04CollaborationBodyEdges(t *testing.T) {
	cases := []struct {
		name, group, action, version, body string
		yes                                bool
		want                               int
	}{
		{"duration-zero", "work-item-worklog", "create", "v2", `{"duration":0}`, false, 0},
		{"duration-absent", "work-item-worklog", "create", "v2", `{}`, false, 2},
		{"duration-legacy-absent", "work-item-worklog", "create", "v1", `{}`, false, 0},
		{"duration-update-absent", "work-item-worklog", "update", "v2", `{}`, false, 0},
		{"duration-wrong-type", "work-item-worklog", "create", "v2", `{"duration":"0"}`, false, 2},
		{"html-literal", "work-item-comment", "create", "v2", `{"comment_html":"<script>literal</script>"}`, false, 0},
		{"comment-empty-update", "work-item-comment", "update", "v2", `{}`, false, 0},
		{"comment-create-required", "work-item-comment", "create", "v2", `{}`, false, 2},
		{"metadata-v1-not-exposed", "work-item-link", "create", "v1", `{"url":"https://example.invalid","metadata":"x"}`, false, 2},
		{"metadata-v2-string", "work-item-link", "create", "v2", `{"url":"https://example.invalid","metadata":"{literal}"}`, false, 0},
		{"metadata-v2-object-invalid", "work-item-link", "create", "v2", `{"url":"https://example.invalid","metadata":{}}`, false, 2},
		{"nullable-title", "work-item-link", "update", "v2", `{"title":null}`, false, 0},
		{"subscriber-noop", "work-item-subscriber", "update", "v2", `{}`, false, 0},
		{"subscriber-remove-confirmation", "work-item-subscriber", "update", "v2", `{"remove":["33333333-3333-4333-8333-333333333333"]}`, false, 2},
		{"subscriber-remove-confirmed", "work-item-subscriber", "update", "v2", `{"remove":["33333333-3333-4333-8333-333333333333"]}`, true, 0},
		{"subscriber-overlap", "work-item-subscriber", "update", "v2", `{"add":["33333333-3333-4333-8333-333333333333"],"remove":["33333333-3333-4333-8333-333333333333"]}`, true, 2},
		{"bulk-empty", "work-item-comment", "bulk-create", "v2", `{"items":[]}`, false, 2},
		{"bulk-row-required", "work-item-comment", "bulk-create", "v2", `{"items":[{}]}`, false, 2},
		{"bulk-update-id-required", "work-item-comment", "bulk-update", "v2", `{"items":[{"comment_html":"x"}]}`, false, 2},
		{"bulk-delete-confirmation", "work-item-comment", "bulk-delete", "v2", `{"ids":["33333333-3333-4333-8333-333333333333"]}`, false, 2},
		{"vote-zero-invalid", "work-item-vote", "cast", "v2", `{"vote":0}`, false, 2},
		{"vote-required", "work-item-vote", "cast", "v2", `{}`, false, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				got, _ := io.ReadAll(r.Body)
				var a, b any
				if json.Unmarshal(got, &a) != nil || json.Unmarshal([]byte(tc.body), &b) != nil || !reflect.DeepEqual(a, b) {
					t.Errorf("body changed: %s", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"ok"}`)
			}))
			defer server.Close()
			args := []string{tc.group, tc.action, "--api-version", tc.version, "--workspace", "example", "--project", "11111111-1111-4111-8111-111111111111", "--work-item", "22222222-2222-4222-8222-222222222222", "--base-url", server.URL, "--body-file", collaborationBodyFile(t, []byte(tc.body))}
			if tc.action == "update" && tc.group != "work-item-subscriber" {
				args = append(args, "--id", "33333333-3333-4333-8333-333333333333")
			}
			if tc.yes {
				args = append(args, "--yes")
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			wantCalls := 0
			if tc.want == 0 {
				wantCalls = 1
			}
			if code != tc.want || calls != wantCalls {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP04CollaborationBulkPartial(t *testing.T) {
	for _, action := range []string{"bulk-create", "bulk-update", "bulk-delete"} {
		t.Run(action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"results":[{"index":0,"result":"failed","code":"invalid_request"}],"succeeded":0,"failed":1}`)
			}))
			defer server.Close()
			for _, fixture := range collaborationFixtures(t) {
				if fixture.Args[0] != "work-item-comment" || fixture.Args[1] != action {
					continue
				}
				args := append(append([]string{}, fixture.Args...), "--base-url", server.URL, "--body-file", collaborationBodyFile(t, fixture.Body))
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 6 || !strings.Contains(out, `"status":"partial"`) || !strings.Contains(stderr, "partial_failure") {
					t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
				}
			}
		})
	}
}

func TestP04CollaborationPagination(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, group := range []string{"work-item-activity", "work-item-comment", "work-item-link", "work-item-worklog", "work-item-vote"} {
			if version == "v1" && (group == "work-item-worklog" || group == "work-item-vote") {
				continue
			}
			t.Run(version+"/"+group, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.URL.Query().Get("fields") != "id" {
						t.Error("fields lost")
					}
					w.Header().Set("Content-Type", "application/json")
					if version == "v1" {
						if calls == 1 {
							_, _ = io.WriteString(w, `{"results":[],"next_cursor":"20:1:0","next_page_results":true}`)
						} else {
							if r.URL.Query().Get("cursor") != "20:1:0" {
								t.Error("cursor lost")
							}
							_, _ = io.WriteString(w, `{"results":[{"id":"last"}],"next_cursor":"20:2:0","next_page_results":false}`)
						}
					} else if calls == 1 {
						_, _ = io.WriteString(w, `{"data":[],"next":7,"pagination":{"style":"offset"}}`)
					} else {
						if r.URL.Query().Get("offset") != "7" {
							t.Error("offset not native next")
						}
						_, _ = io.WriteString(w, `{"data":[{"id":"last"}],"next":null,"pagination":{"style":"offset"}}`)
					}
				}))
				defer server.Close()
				args := []string{group, "list", "--api-version", version, "--workspace", "example", "--project", "11111111-1111-4111-8111-111111111111", "--work-item", "22222222-2222-4222-8222-222222222222", "--base-url", server.URL, "--all", "--fields", "id"}
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 0 || calls != 2 || !strings.Contains(out, `"complete":true`) {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
			})
		}
	}
}

func TestP04CollaborationCursorContinuation(t *testing.T) {
	for _, group := range []string{"work-item-activity", "work-item-comment", "work-item-link", "work-item-worklog"} {
		t.Run(group, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			const cursor = "opaque+/=:cursor"
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("cursor") != cursor || r.URL.Query().Get("paginate") != "cursor" {
					t.Errorf("continuation changed: %s", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"id":"continued"}],"pagination":{"style":"cursor"},"has_more":false,"next_cursor":null}`)
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			args := []string{group, "list", "--api-version", "v2", "--workspace", "example", "--project", "11111111-1111-4111-8111-111111111111", "--work-item", "22222222-2222-4222-8222-222222222222", "--base-url", server.URL, "--paginate", "cursor", "--cursor", cursor}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || calls != 1 || !strings.Contains(out, `"id":"continued"`) {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}
