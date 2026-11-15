package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

type contentAssetWireCase struct {
	Version  string            `json:"version"`
	Group    string            `json:"group"`
	Action   string            `json:"action"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Parents  map[string]string `json:"parents"`
	Body     json.RawMessage   `json:"body"`
	Status   int               `json:"status"`
	Response json.RawMessage   `json:"response"`
}

// TestContentAssetsNativeWire covers the ordinary API phases only. Storage and
// protected secret delivery acceptance belong to their shared transport owner.
func TestContentAssetsNativeWire(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/p06-asset-native-wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []contentAssetWireCase
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
				if r.Method != tc.Method || r.URL.Path != tc.Path || r.URL.RawQuery != "" || r.Header.Get("X-API-Key") != "fixture-key" {
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
				if tc.Status != http.StatusNoContent {
					_, _ = w.Write(tc.Response)
				}
			}))
			defer server.Close()
			args := []string{tc.Group, tc.Action, "--base-url", server.URL}
			for _, name := range []string{"workspace", "project", "work-item", "page", "id"} {
				if value, ok := tc.Parents[name]; ok {
					args = append(args, "--"+name, value)
				}
			}
			if tc.Action == "delete" {
				args = append(args, "--yes")
			}
			if string(tc.Body) != "null" {
				file := filepath.Join(t.TempDir(), "body.json")
				if err := os.WriteFile(file, tc.Body, 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--body-file", file)
			}
			code, out, stderr := invoke(t, catalog, args, contentAssetsHandlers())
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
				t.Fatalf("native phase response changed: %s", out)
			}
		})
	}
}
