package cli

import (
	"bytes"
	"context"
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

func TestP09WorkflowGovernanceRequests(t *testing.T) {
	raw, err := os.ReadFile("testdata/p09-governance-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		ID       string          `json:"id"`
		Command  []string        `json:"command"`
		Method   string          `json:"method"`
		Route    string          `json:"route"`
		Status   int             `json:"status"`
		Body     json.RawMessage `json:"body"`
		Response json.RawMessage `json:"response"`
		Mutation bool            `json:"mutation"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 9 {
		t.Fatalf("operation count %d", len(fixtures))
	}
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.ID, func(t *testing.T) {
			for _, status := range []int{f.Status, 403, 409} {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					wantPath := strings.NewReplacer("{slug}", "fixture-workspace", "{project_id}", "ENG", "{type_id}", workflowFixtureID, "{pk}", workflowFixtureID).Replace(f.Route)
					if r.Method != f.Method || r.URL.Path != wantPath {
						t.Errorf("request %s %s", r.Method, r.URL)
					}
					body, _ := io.ReadAll(r.Body)
					if string(f.Body) != "null" {
						var got, want any
						if json.Unmarshal(body, &got) != nil || json.Unmarshal(f.Body, &want) != nil || !reflect.DeepEqual(got, want) {
							t.Errorf("body %s", body)
						}
					} else if len(body) != 0 {
						t.Errorf("unexpected body %s", body)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					if status == 403 {
						_, _ = io.WriteString(w, `{"code":"forbidden","detail":"Scope denied"}`)
					} else if status == 409 {
						_, _ = io.WriteString(w, `{"code":"workflow_state_orphans","detail":"State mapping required"}`)
					} else if status != 204 {
						_, _ = w.Write(f.Response)
					}
				}))
				args := append(append([]string{}, f.Command...), "--api-version", "v2", "--workspace", "fixture-workspace", "--base-url", server.URL)
				for p, flag := range map[string]string{"project_id": "project", "type_id": "type", "pk": "id"} {
					if strings.Contains(f.Route, "{"+p+"}") {
						value := workflowFixtureID
						if p == "project_id" {
							value = "ENG"
						}
						args = append(args, "--"+flag, value)
					}
				}
				if string(f.Body) != "null" {
					args = append(args, "--body-file", collaborationBodyFile(t, f.Body))
				}
				if f.Mutation {
					code, _, stderr := invoke(t, catalog, args, nil)
					if code != 2 || calls != 0 || !strings.Contains(stderr, "confirmation_required") {
						t.Fatalf("confirmation code=%d calls=%d err=%s", code, calls, stderr)
					}
					args = append(args, "--yes")
				}
				code, out, stderr := invoke(t, catalog, args, nil)
				server.Close()
				if calls != 1 {
					t.Fatalf("calls=%d code=%d err=%s", calls, code, stderr)
				}
				if status == 403 {
					if code != 3 || out != "" {
						t.Fatalf("permission code=%d out=%s err=%s", code, out, stderr)
					}
				} else if status == 409 {
					if code != 5 || out != "" {
						t.Fatalf("conflict code=%d out=%s err=%s", code, out, stderr)
					}
				} else if code != 0 {
					t.Fatalf("code=%d err=%s", code, stderr)
				}
			}
		})
	}
}

func TestP09WorkflowGovernanceServerPreview(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PATCH" || r.URL.Query().Get("dry_run") != "true" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"total":3,"per_state":[{"state_id":"11111111-1111-4111-8111-111111111111","count":3}]}`)
	}))
	defer server.Close()
	args := []string{"work-item-type-workflow", "update", "--api-version", "v2", "--workspace", "fixture-workspace", "--project", "ENG", "--type", workflowFixtureID, "--base-url", server.URL, "--server-preview=true", "--body-file", collaborationBodyFile(t, []byte(`{"workflow_id":"11111111-1111-4111-8111-111111111111"}`))}
	code, out, stderr := invoke(t, catalog, append(append([]string{}, args...), "--dry-run"), nil)
	if code != 0 || calls != 0 {
		t.Fatalf("local preview code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
	code, out, stderr = invoke(t, catalog, args, nil)
	if code != 0 || calls != 1 || !strings.Contains(out, `"total":3`) {
		t.Fatalf("server preview code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
}

func TestP09WorkflowGovernancePreviewFailures(t *testing.T) {
	for _, malformed := range []bool{true, false} {
		t.Run(fmt.Sprint(malformed), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if malformed {
					_, _ = io.WriteString(w, `{"preview":`)
				} else {
					_, _ = io.WriteString(w, `{"preview":{"total":3}}`)
				}
			}))
			defer server.Close()
			args := []string{"work-item-type-workflow", "update", "--api-version", "v2", "--workspace", "fixture", "--project", "ENG", "--type", workflowFixtureID, "--base-url", server.URL, "--server-preview=true", "--body-file", collaborationBodyFile(t, []byte(`{"workflow_id":"11111111-1111-4111-8111-111111111111"}`))}
			var diagnostic bytes.Buffer
			code := Run(context.Background(), args, Streams{Out: failedAssetWriter{}, Err: &diagnostic}, Build{})
			want := 1
			if malformed {
				want = 5
			}
			if code != want || calls != 1 || strings.Contains(diagnostic.String(), `"outcome_unknown":true`) || strings.Contains(diagnostic.String(), `"remote_changed":true`) {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, diagnostic.String())
			}
		})
	}
}
