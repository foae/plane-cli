package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestTypeGovernanceReads(t *testing.T) {
	const typeID = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		group, path, response string
		status                int
	}{
		{"workspace-feature", "features/", `{"is_work_item_types_enabled":true,"future_flag":9007199254740993}`, 200},
		{"work-item-type-governance", "work-item-types/" + typeID + "/governance/", `{"mode":"constrained","allowlist":[{"locked":true,"in_use_by_project_ids":["project"]}]}`, 200},
		{"work-item-type-governance", "work-item-types/" + typeID + "/governance/", `{"type":"conflict","code":"states_managed_at_project","detail":"Project-managed workflows"}`, 409},
	} {
		t.Run(tc.group+"/"+http.StatusText(tc.status), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v2/workspaces/example/"+tc.path || r.URL.RawQuery != "" || r.Header.Get("X-Api-Key") != "fixture-key" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || len(body) != 0 {
					t.Errorf("unexpected request body: %s %v", body, err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			args := []string{tc.group, "get", "--api-version", "v2", "--workspace", "example", "--base-url", server.URL}
			if tc.group == "work-item-type-governance" {
				args = append(args, "--type", typeID)
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if calls != 1 {
				t.Fatalf("calls=%d out=%s stderr=%s", calls, out, stderr)
			}
			if tc.status == 200 {
				if code != 0 || stderr != "" || !strings.Contains(out, tc.response) {
					t.Fatalf("code=%d out=%s stderr=%s", code, out, stderr)
				}
			} else if code != 5 || out != "" || !strings.Contains(stderr, `"http_status":409`) {
				t.Fatalf("code=%d out=%s stderr=%s", code, out, stderr)
			}
		})
	}
}
