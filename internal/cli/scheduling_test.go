package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

type schedulingFixture struct {
	Version   string            `json:"version"`
	Resource  string            `json:"resource"`
	Action    string            `json:"action"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     map[string]string `json:"query"`
	Body      json.RawMessage   `json:"body"`
	Status    int               `json:"status"`
	Response  json.RawMessage   `json:"response"`
	Paginated bool              `json:"paginated"`
}

func schedulingFixtures(t *testing.T) []schedulingFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/p04-scheduling-wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []schedulingFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func schedulingArgs(t *testing.T, fixture schedulingFixture, base string) []string {
	t.Helper()
	args := append([]string{}, fixture.Command...)
	args = append(args, "--workspace", "team", "--project", "project", "--api-version", fixture.Version, "--base-url", base)
	args = append(args, fixture.Args...)
	if string(fixture.Body) != "null" && len(fixture.Body) > 0 {
		path := filepath.Join(t.TempDir(), "body.json")
		if err := os.WriteFile(path, fixture.Body, 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--body-file", path)
	}
	return args
}

func TestSchedulingWireOperations(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range schedulingFixtures(t) {
		t.Run(fixture.Version+"/"+fixture.Resource+"/"+fixture.Action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "scheduling-fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != fixture.Method || r.URL.Path != fixture.Path {
					t.Errorf("request: %s %s; want %s %s", r.Method, r.URL.Path, fixture.Method, fixture.Path)
				}
				wantQuery := url.Values{}
				for key, value := range fixture.Query {
					wantQuery.Set(key, value)
				}
				if !reflect.DeepEqual(r.URL.Query(), wantQuery) {
					t.Errorf("query: %v; want %v", r.URL.Query(), wantQuery)
				}
				body, _ := io.ReadAll(r.Body)
				if string(fixture.Body) == "null" {
					if len(body) != 0 {
						t.Errorf("unexpected body: %s", body)
					}
				} else {
					got, e1 := spec.Decode(body)
					want, e2 := spec.Decode(fixture.Body)
					if e1 != nil || e2 != nil || !reflect.DeepEqual(got, want) {
						t.Errorf("body: %s; want %s", body, fixture.Body)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(fixture.Status)
				if fixture.Status != 204 {
					_, _ = w.Write(fixture.Response)
				}
			}))
			defer server.Close()
			args := schedulingArgs(t, fixture, server.URL)
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
			value, err := spec.Decode([]byte(out))
			if err != nil {
				t.Fatal(err)
			}
			want, err := spec.Decode(fixture.Response)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value.(map[string]any)["data"], want) {
				t.Fatalf("native response changed: %s", out)
			}
			if !strings.Contains(out, fmt.Sprintf(`"http_status":%d`, fixture.Status)) {
				t.Fatalf("status lost: %s", out)
			}
			// Every operation's error is safe and remains an upstream error, not availability.
			errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(409)
				_, _ = io.WriteString(w, `{"type":"conflict","code":"conflict","detail":"scheduling-private-canary"}`)
			}))
			defer errorServer.Close()
			code, out, stderr = invoke(t, catalog, schedulingArgs(t, fixture, errorServer.URL), nil)
			if code != 5 || out != "" || strings.Contains(stderr, "scheduling-private-canary") {
				t.Fatalf("unsafe error: %d %s %s", code, out, stderr)
			}
		})
	}
}

func TestSchedulingOfflineSchemas(t *testing.T) {
	transportEnv(t)
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range schedulingFixtures(t) {
		t.Run(fixture.Version+"/"+fixture.Resource+"/"+fixture.Action, func(t *testing.T) {
			args := append([]string{"schema"}, fixture.Command...)
			args = append(args, "--api-version", fixture.Version)
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || stderr != "" || !strings.Contains(out, fixture.Path[:7]) {
				t.Fatalf("schema: %d %s %s", code, out, stderr)
			}
		})
	}
}

func TestSchedulingConfirmation(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range schedulingFixtures(t) {
		if fixture.Action != "delete" && fixture.Action != "archive" && fixture.Action != "bulk-delete" && fixture.Action != "transfer-work-items" && fixture.Action != "remove-work-item" && fixture.Action != "manage-work-items" {
			continue
		}
		t.Run(fixture.Version+"/"+fixture.Resource+"/"+fixture.Action, func(t *testing.T) {
			transportEnv(t)
			args := schedulingArgs(t, fixture, "https://invalid.example")
			withoutYes := make([]string, 0, len(args))
			for _, arg := range args {
				if arg != "--yes" {
					withoutYes = append(withoutYes, arg)
				}
			}
			code, out, stderr := invoke(t, catalog, withoutYes, nil)
			if code != 2 || out != "" || !strings.Contains(stderr, "confirmation") {
				t.Fatalf("confirmation: %d %s %s", code, out, stderr)
			}
			code, out, stderr = invoke(t, catalog, append(withoutYes, "--dry-run"), nil)
			if code != 0 || stderr != "" || !strings.Contains(out, `"requires_confirmation":true`) {
				t.Fatalf("dry run: %d %s %s", code, out, stderr)
			}
		})
	}
}

func TestSchedulingManageAddOnly(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"cycle", "module", "milestone"} {
		for _, body := range []string{`{"add":["22222222-2222-4222-8222-222222222222"]}`, `{"add":[],"remove":[]}`, `{}`} {
			t.Run(resource+"/"+body, func(t *testing.T) {
				transportEnv(t)
				fixture := schedulingFixture{Version: "v2", Command: []string{resource, "manage-work-items"}, Args: []string{"--id", "source"}, Body: json.RawMessage(body)}
				args := append(schedulingArgs(t, fixture, "https://invalid.example"), "--dry-run")
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 0 || stderr != "" || !strings.Contains(out, `"requires_confirmation":false`) {
					t.Fatalf("add-only: %d %s %s", code, out, stderr)
				}
			})
		}
	}
}

func TestSchedulingBodyConstraints(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ version, resource, action, body string }{
		{"v2", "cycle", "create", `{}`},
		{"v2", "cycle", "create", `{"name":"x","owned_by_id":"user"}`},
		{"v2", "cycle", "create", `{"name":"x","start_date":"2026-02-30T00:00:00Z"}`},
		{"v2", "cycle", "create", `{"name":"x","end_date":"2026-01-01"}`},
		{"v2", "cycle", "create", `{"name":"x","sort_order":null}`},
		{"v2", "cycle", "create", `{"name":"x","timezone":"Not/A_Zone"}`},
		{"v2", "cycle", "create", `{"name":"x","timezone":""}`},
		{"v2", "cycle", "upsert", `{"name":"x","external_id":"x"}`},
		{"v2", "module", "create", `{"name":"x","status":"unknown"}`},
		{"v2", "module", "create", `{"name":"x","member_ids":[]}`},
		{"v2", "module", "create", `{"name":"x","start_date":"2026-02-30"}`},
		{"v2", "module", "create", `{"name":"x","start_date":"2026-01-19","target_date":"2026-01-05"}`},
		{"v1", "milestone", "create", `{"title":"x","target_date":"2026-02-30"}`},
		{"v2", "milestone", "create", `{"title":"x","target_date":"not-a-date"}`},
		{"v2", "estimate", "create", `{"name":"x","type":"fibonacci"}`},
		{"v2", "estimate", "create", `{"name":"x","last_used":false}`},
		{"v2", "estimate", "bulk-create", `{"items":[]}`},
		{"v2", "estimate", "bulk-update", `{"items":[{"name":"x"}]}`},
		{"v2", "estimate", "bulk-delete", `{"ids":["not-a-uuid"]}`},
		{"v2", "module", "manage-work-items", `{"remove":null}`},
		{"v2", "cycle", "transfer-work-items", `{"new_cycle_id":null}`},
		{"v1", "estimate-point", "create", `{"estimate_points":[{"value":"123456789012345678901"}]}`},
		{"v1", "estimate-point", "create", `{"estimate_points":[{"key":0}]}`},
		{"v2", "estimate-point", "create", `{"value":0}`},
		{"v2", "estimate-point", "create", `{"value":"x","key":0.5}`},
		{"v2", "cycle", "create", `{"name":"x","name":"duplicate"}`},
		{"v2", "cycle", "create", `{"name":"x"} {}`},
		{"v2", "cycle", "create", `[]`},
		{"v2", "cycle", "create", `null`},
	}
	oversized := `{"items":[` + strings.TrimSuffix(strings.Repeat(`{"name":"x"},`, 51), ",") + `]}`
	cases = append(cases, struct{ version, resource, action, body string }{"v2", "cycle", "bulk-create", oversized})
	cases = append(cases, struct{ version, resource, action, body string }{"v2", "cycle", "create", `{"name":"` + strings.Repeat("x", 256) + `"}`})
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			transportEnv(t)
			command := []string{tc.resource, tc.action}
			if tc.resource == "estimate-point" {
				command = []string{"estimate", "point", tc.action}
			}
			fixture := schedulingFixture{Version: tc.version, Command: command, Body: json.RawMessage(tc.body), Args: []string{"--yes"}}
			// --yes is only available for destructive or body-driven actions.
			if tc.action != "bulk-delete" && tc.action != "manage-work-items" && tc.action != "transfer-work-items" {
				fixture.Args = nil
			}
			if tc.resource == "estimate-point" {
				fixture.Args = append(fixture.Args, "--estimate", "11111111-1111-4111-8111-111111111111")
			}
			if tc.action == "manage-work-items" || tc.action == "transfer-work-items" {
				fixture.Args = append(fixture.Args, "--id", "source")
			}
			code, out, stderr := invoke(t, catalog, schedulingArgs(t, fixture, "https://invalid.example"), nil)
			if code != 2 || out != "" {
				t.Fatalf("invalid body accepted: %d %s %s", code, out, stderr)
			}
		})
	}
}

func TestSchedulingBulkPartial(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range schedulingFixtures(t) {
		if !strings.HasPrefix(fixture.Action, "bulk-") {
			continue
		}
		t.Run(fixture.Resource+"/"+fixture.Action, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"results":[{"index":0,"result":"failed","type":"not_found","code":"not_found","detail":"private-canary"}],"succeeded":0,"failed":1}`)
			}))
			defer server.Close()
			code, out, stderr := invoke(t, catalog, schedulingArgs(t, fixture, server.URL), nil)
			if code != 6 || !strings.Contains(out, `"status":"partial"`) || strings.Contains(stderr, "private-canary") {
				t.Fatalf("partial: %d %s %s", code, out, stderr)
			}
		})
	}
}

func TestSchedulingPagination(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range schedulingFixtures(t) {
		if !fixture.Paginated {
			continue
		}
		styles := []string{"cursor"}
		if fixture.Version == "v2" {
			styles = append(styles, "offset")
		}
		for _, style := range styles {
			t.Run(fixture.Version+"/"+fixture.Resource+"/"+fixture.Action+"/"+style, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.URL.Path != fixture.Path {
						t.Errorf("wrong route %s", r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					if calls == 2 {
						key, want := "cursor", "next:+/="
						if style == "offset" {
							key, want = "offset", "7"
						}
						if r.URL.Query().Get(key) != want {
							t.Errorf("continuation lost: %s", r.URL)
						}
						if r.URL.Query().Get("per_page") != "2" {
							t.Error("page size lost")
						}
					}
					if fixture.Version == "v1" {
						_, _ = fmt.Fprintf(w, `{"results":[],"next_page_results":%t,"next_cursor":"next:+/="}`, calls == 1)
					} else if style == "cursor" {
						_, _ = fmt.Fprintf(w, `{"data":[],"pagination":{"style":"cursor"},"has_more":%t,"next_cursor":"next:+/="}`, calls == 1)
					} else if calls == 1 {
						_, _ = io.WriteString(w, `{"data":[],"pagination":{"style":"offset"},"next":7}`)
					} else {
						_, _ = io.WriteString(w, `{"data":[],"pagination":{"style":"offset"},"next":null}`)
					}
				}))
				defer server.Close()
				pageFixture := fixture
				pageFixture.Args = []string{"--per-page=2", "--all"}
				if fixture.Resource == "estimate-point" {
					pageFixture.Args = append(pageFixture.Args, "--estimate", "11111111-1111-4111-8111-111111111111")
				}
				if fixture.Action == "list-work-items" {
					pageFixture.Args = append(pageFixture.Args, "--id", "11111111-1111-4111-8111-111111111111")
				}
				if fixture.Version == "v2" && style == "cursor" {
					pageFixture.Args = append(pageFixture.Args, "--paginate=cursor")
				}
				code, out, stderr := invoke(t, catalog, schedulingArgs(t, pageFixture, server.URL), nil)
				if code != 0 || stderr != "" || calls != 2 || !strings.Contains(out, `"pages":2`) {
					t.Fatalf("pagination: %d %s %s calls=%d", code, out, stderr, calls)
				}
			})
		}
	}
}
