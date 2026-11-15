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

type collectionFixture struct {
	ID          string            `json:"id"`
	Handler     string            `json:"handler"`
	Version     string            `json:"version"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Flags       map[string]string `json:"flags"`
	Body        json.RawMessage   `json:"body"`
	Status      int               `json:"status"`
	Response    json.RawMessage   `json:"response"`
	Destructive bool              `json:"destructive"`
}

func TestP06CollectionRequests(t *testing.T) {
	raw, err := os.ReadFile("testdata/p06-collection-requests.json")
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
					if fixture.Destructive || fixture.Handler == "collection-member-add" || fixture.Handler == "collection-member-update" {
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

func TestP06CollectionV2Schemas(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, handler := range []string{"collection-delete", "collection-manage-pages", "collection-member-list", "collection-page-search"} {
		t.Run(handler, func(t *testing.T) {
			for _, command := range catalog.Commands {
				if command.Handler != handler {
					continue
				}
				version, ok := command.Versions["v2"]
				if !ok {
					t.Fatal("missing v2 descriptor")
				}
				raw, err := json.Marshal(version)
				if err != nil {
					t.Fatal(err)
				}
				var descriptor map[string]any
				if err := json.Unmarshal(raw, &descriptor); err != nil {
					t.Fatal(err)
				}
				schema := descriptor["schema"].(map[string]any)
				properties := schema["properties"].(map[string]any)
				flags := properties["flags"].(map[string]any)["properties"].(map[string]any)
				if flags["api-version"].(map[string]any)["const"] != "v2" {
					t.Fatal("schema is not version-exact")
				}
				switch handler {
				case "collection-delete":
					query := properties["query"].(map[string]any)["properties"].(map[string]any)
					if len(query) != 2 || query["fields"] == nil || query["expand"] == nil {
						t.Fatalf("delete query = %#v", query)
					}
					if properties["body"] != nil {
						t.Fatal("delete must not import a request body")
					}
				case "collection-manage-pages":
					if _, ok := command.Versions["v1"]; ok {
						t.Fatal("manage-pages must remain separate from v1 add/remove")
					}
					body := properties["body"].(map[string]any)
					fields := body["properties"].(map[string]any)
					if len(fields) != 2 || body["required"] != nil {
						t.Fatalf("body = %#v", body)
					}
					for _, name := range []string{"add", "remove"} {
						field := fields[name].(map[string]any)
						items := field["items"].(map[string]any)
						if field["type"] != "array" || items["type"] != "string" || items["format"] != "uuid" || field["minItems"] != nil || field["uniqueItems"] != nil {
							t.Fatalf("%s = %#v", name, field)
						}
					}
				default:
					query := properties["query"].(map[string]any)["properties"].(map[string]any)
					want := 2
					if handler == "collection-page-search" {
						want = 1
					}
					if len(query) != want || query["fields"] == nil || (want == 2 && query["expand"] == nil) {
						t.Fatalf("native query = %#v", query)
					}
					if properties["body"] != nil || descriptor["response_kind"] != "object" || descriptor["pagination"] != "none" {
						t.Fatal("native object reads must not import v1 body or pagination semantics")
					}
				}
				return
			}
			t.Fatal("missing command")
		})
	}
}
