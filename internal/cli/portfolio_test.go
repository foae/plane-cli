package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestP07PortfolioRequests(t *testing.T) {
	raw, err := os.ReadFile("testdata/p07-portfolio-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []collectionFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			for _, rejected := range []bool{false, true} {
				t.Run(fmt.Sprint(rejected), func(t *testing.T) {
					transportEnv(t)
					t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						if r.Method != fixture.Method || r.URL.RequestURI() != fixture.Path {
							t.Errorf("request = %s %s", r.Method, r.URL.RequestURI())
						}
						if r.Header.Get("X-API-Key") != "fixture-key" {
							t.Error("missing native authentication")
						}
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						if string(fixture.Body) == "null" {
							if len(body) != 0 {
								t.Errorf("unexpected body %s", body)
							}
						} else {
							var got, want any
							if json.Unmarshal(body, &got) != nil || json.Unmarshal(fixture.Body, &want) != nil || !reflect.DeepEqual(got, want) {
								t.Errorf("body = %s; want %s", body, fixture.Body)
							}
						}
						w.Header().Set("Content-Type", "application/json")
						if rejected {
							w.WriteHeader(404)
							_, _ = io.WriteString(w, `{"error":"Not found"}`)
							return
						}
						w.WriteHeader(fixture.Status)
						if fixture.Status != 204 {
							_, _ = w.Write(fixture.Response)
						}
					}))
					defer server.Close()
					var path []string
					for _, command := range catalog.Commands {
						if command.Handler == fixture.Handler {
							path = command.Path
							break
						}
					}
					if path == nil {
						t.Fatalf("missing offline command %s", fixture.Handler)
					}
					args := append(append([]string{}, path...), "--api-version", fixture.Version, "--base-url", server.URL)
					for flag, value := range fixture.Flags {
						args = append(args, "--"+flag, value)
					}
					if string(fixture.Body) != "null" {
						args = append(args, "--body-file", collaborationBodyFile(t, fixture.Body))
					}
					replacement := false
					if fixture.Version == "v2" {
						var body map[string]json.RawMessage
						if err := json.Unmarshal(fixture.Body, &body); err != nil {
							t.Fatal(err)
						}
						_, projects := body["project_ids"]
						_, members := body["member_ids"]
						replacement = (fixture.Handler == "initiative-update" && projects) ||
							(fixture.Handler == "teamspace-update" && (projects || members))
					}
					if fixture.Destructive || replacement {
						code, out, stderr := invoke(t, catalog, args, nil)
						if code != 2 || calls != 0 || out != "" {
							t.Fatalf("confirmation code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
						code, out, stderr = invoke(t, catalog, append(append([]string{}, args...), "--dry-run"), nil)
						if code != 0 || calls != 0 || stderr != "" || !strings.Contains(out, `"requires_confirmation":true`) {
							t.Fatalf("dry-run code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
						args = append(args, "--yes")
					}
					code, out, stderr := invoke(t, catalog, args, nil)
					if rejected {
						if code != 5 || calls != 1 || out != "" {
							t.Fatalf("rejection code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
						return
					}
					if code != 0 || calls != 1 || stderr != "" {
						t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
					}
					var envelope struct {
						Data json.RawMessage `json:"data"`
					}
					if err := json.Unmarshal([]byte(out), &envelope); err != nil {
						t.Fatal(err)
					}
					if fixture.Status != 204 {
						var got, want any
						if json.Unmarshal(envelope.Data, &got) != nil || json.Unmarshal(fixture.Response, &want) != nil || !reflect.DeepEqual(got, want) {
							t.Fatalf("native response changed: %s", out)
						}
					}
				})
			}
		})
	}
}

func TestP07PortfolioReplacementConfirmation(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		family string
		field  string
	}{
		{"initiative", "project_ids"},
		{"teamspace", "member_ids"},
		{"teamspace", "project_ids"},
	} {
		for _, value := range []string{`[]`, `["11111111-1111-4111-8111-111111111111"]`, ""} {
			t.Run(tc.family+"/"+tc.field+"/"+value, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				body := `{"name":"Metadata"}`
				if value != "" {
					body = fmt.Sprintf(`{"%s":%s}`, tc.field, value)
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != http.MethodPatch || r.URL.Path != "/api/v2/workspaces/acme/"+tc.family+"s/11111111-1111-4111-8111-111111111111/" {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					}
					got, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					var actual, expected any
					if json.Unmarshal(got, &actual) != nil || json.Unmarshal([]byte(body), &expected) != nil || !reflect.DeepEqual(actual, expected) {
						t.Errorf("body=%s want=%s", got, body)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"id":"native-id","name":"Metadata"}`)
				}))
				defer server.Close()
				args := []string{tc.family, "update", "--api-version", "v2", "--workspace", "acme", "--id", "11111111-1111-4111-8111-111111111111", "--base-url", server.URL, "--body-file", collaborationBodyFile(t, json.RawMessage(body))}
				required := value != ""
				code, out, stderr := invoke(t, catalog, append(append([]string{}, args...), "--dry-run"), nil)
				if code != 0 || calls != 0 || stderr != "" || !strings.Contains(out, fmt.Sprintf(`"requires_confirmation":%t`, required)) {
					t.Fatalf("dry-run code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
				if required {
					code, out, stderr = invoke(t, catalog, args, nil)
					if code != 2 || calls != 0 || out != "" || !strings.Contains(stderr, "confirmation_required") {
						t.Fatalf("rejection code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
					}
					args = append(args, "--yes")
				}
				code, out, stderr = invoke(t, catalog, args, nil)
				if code != 0 || calls != 1 || stderr != "" {
					t.Fatalf("update code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
			})
		}
	}
}

func TestP07PortfolioTablesAndEntitlement(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"v1", "v2"} {
		for _, family := range []string{"initiative", "initiative-label", "teamspace"} {
			for _, action := range []string{"list", "get"} {
				for _, status := range []int{200, 402} {
					t.Run(fmt.Sprintf("%s/%s/%s/%d", version, family, action, status), func(t *testing.T) {
						transportEnv(t)
						t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
						calls := 0
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							calls++
							if r.Method != http.MethodGet {
								t.Errorf("method=%s", r.Method)
							}
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(status)
							if status == 402 {
								_, _ = io.WriteString(w, `{"error":"Portfolio entitlement required"}`)
							} else if action == "list" {
								_, _ = io.WriteString(w, `{"results":[{"id":"native-id","name":"Native portfolio","private_field":"must-not-render"}],"next_cursor":null,"next_page_results":false}`)
							} else {
								_, _ = io.WriteString(w, `{"id":"native-id","name":"Native portfolio","private_field":"must-not-render"}`)
							}
						}))
						defer server.Close()
						args := []string{family, action, "--api-version", version, "--workspace", "acme", "--base-url", server.URL, "--output", "table"}
						if action == "get" {
							args = append(args, "--id", "11111111-1111-4111-8111-111111111111")
						}
						code, out, stderr := invoke(t, catalog, args, nil)
						if status == 402 {
							if code != 5 || calls != 1 || out != "" || !strings.Contains(stderr, `"http_status":402`) || !strings.Contains(stderr, `"code":"api_error"`) {
								t.Fatalf("entitlement code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
							}
							return
						}
						if code != 0 || calls != 1 || stderr != "" || !strings.Contains(out, "native-id") || !strings.Contains(out, "Native portfolio") || strings.Contains(out, "must-not-render") {
							t.Fatalf("projection code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
						}
					})
				}
			}
		}
	}
}
