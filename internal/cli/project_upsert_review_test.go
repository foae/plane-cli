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

func TestProjectUpsertReviewedFields(t *testing.T) {
	const body = `{"identifier":"EX","name":"Example","external_source":"import","external_id":"1","archive_in":0,"close_in":0,"intake_view":false,"is_issue_type_enabled":false,"is_time_tracking_enabled":false,"issue_views_view":false,"logo_props":"","module_view":false,"network":"0","page_view":false,"priority":"none","project_lead_id":null,"start_date":null,"state_id":null,"target_date":null,"timezone":"UTC"}`
	for _, outcome := range []struct {
		name   string
		status int
	}{
		{"updated", http.StatusOK},
		{"created", http.StatusCreated},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.RequestURI() != "/api/v2/workspaces/example/projects/upsert/" {
					t.Errorf("wire: %s %s", r.Method, r.URL.RequestURI())
				}
				if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Api-Key") != "fixture-key" {
					t.Error("unexpected request headers")
				}
				got, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("reading request body: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var actual, expected map[string]json.RawMessage
				if err := json.Unmarshal(got, &actual); err != nil {
					t.Errorf("decoding request body: %v", err)
				}
				if err := json.Unmarshal([]byte(body), &expected); err != nil {
					t.Errorf("decoding expected body: %v", err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("request body %s != %s", got, body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Plane-Upsert", outcome.name)
				w.WriteHeader(outcome.status)
				_, _ = io.WriteString(w, `{"id":"project-id","name":"Example"}`)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "body.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, stdout, stderr := invoke(t, catalog, []string{"project", "upsert", "--workspace", "example", "--base-url", server.URL, "--body-file", path}, nil)
			if code != 0 || calls != 1 || stderr != "" || !strings.Contains(stdout, fmt.Sprintf(`"http_status":%d`, outcome.status)) {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}
