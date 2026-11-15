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

type p07ReleaseFixture struct {
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
	Command     []string          `json:"command"`
}

func p07ReleaseFixtures(t *testing.T) map[string]p07ReleaseFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/p07-releases-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []p07ReleaseFixture
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	fixtures := make(map[string]p07ReleaseFixture, len(rows))
	for _, row := range rows {
		fixtures[row.ID] = row
	}
	return fixtures
}

func TestP07ReleaseRequests(t *testing.T) {
	fixtures := p07ReleaseFixtures(t)
	t.Run("v2.releases.list-releases", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.list-releases"]) })
	t.Run("v2.releases.get-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.get-release"]) })
	t.Run("v2.releases.get-release-changelog", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.get-release-changelog"]) })
	t.Run("v2.releases.create-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.create-release"]) })
	t.Run("v2.releases.update-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.update-release"]) })
	t.Run("v2.releases.update-release-changelog", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.update-release-changelog"]) })
	t.Run("v2.releases.delete-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.delete-release"]) })
	t.Run("v2.releases.list-release-labels", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.list-release-labels"]) })
	t.Run("v2.releases.manage-release-labels", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.manage-release-labels"]) })
	t.Run("v2.releases.list-release-work-items", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.list-release-work-items"]) })
	t.Run("v2.releases.manage-release-work-items", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.releases.manage-release-work-items"]) })
	t.Run("v2.release-tags.list-release-tags", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-tags.list-release-tags"]) })
	t.Run("v2.release-tags.get-release-tag", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-tags.get-release-tag"]) })
	t.Run("v2.release-tags.create-release-tag", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-tags.create-release-tag"]) })
	t.Run("v2.release-tags.update-release-tag", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-tags.update-release-tag"]) })
	t.Run("v2.release-tags.delete-release-tag", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-tags.delete-release-tag"]) })
	t.Run("v2.release-labels.list-release-labels", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-labels.list-release-labels"]) })
	t.Run("v2.release-labels.get-release-label", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-labels.get-release-label"]) })
	t.Run("v2.release-labels.create-release-label", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-labels.create-release-label"]) })
	t.Run("v2.release-labels.update-release-label", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-labels.update-release-label"]) })
	t.Run("v2.release-labels.delete-release-label", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-labels.delete-release-label"]) })
	t.Run("v2.release-comments.list-release-comments", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-comments.list-release-comments"]) })
	t.Run("v2.release-comments.get-release-comment", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-comments.get-release-comment"]) })
	t.Run("v2.release-comments.create-release-comment", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-comments.create-release-comment"]) })
	t.Run("v2.release-comments.update-release-comment", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-comments.update-release-comment"]) })
	t.Run("v2.release-comments.delete-release-comment", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-comments.delete-release-comment"]) })
	t.Run("v2.release-links.list-release-links", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-links.list-release-links"]) })
	t.Run("v2.release-links.get-release-link", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-links.get-release-link"]) })
	t.Run("v2.release-links.create-release-link", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-links.create-release-link"]) })
	t.Run("v2.release-links.update-release-link", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-links.update-release-link"]) })
	t.Run("v2.release-links.delete-release-link", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.release-links.delete-release-link"]) })
	t.Run("v2.project-releases.list-project-releases", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.project-releases.list-project-releases"]) })
	t.Run("v2.project-releases.get-project-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.project-releases.get-project-release"]) })
	t.Run("v2.project-releases.create-project-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.project-releases.create-project-release"]) })
	t.Run("v2.project-releases.update-project-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.project-releases.update-project-release"]) })
	t.Run("v2.project-releases.delete-project-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.project-releases.delete-project-release"]) })
	t.Run("v2.project-releases.get-project-release-changelog", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-releases.get-project-release-changelog"])
	})
	t.Run("v2.project-releases.update-project-release-changelog", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-releases.update-project-release-changelog"])
	})
	t.Run("v2.project-releases.list-project-release-labels", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v2.project-releases.list-project-release-labels"]) })
	t.Run("v2.project-releases.manage-project-release-labels", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-releases.manage-project-release-labels"])
	})
	t.Run("v2.project-releases.list-project-release-work-items", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-releases.list-project-release-work-items"])
	})
	t.Run("v2.project-releases.manage-project-release-work-items", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-releases.manage-project-release-work-items"])
	})
	t.Run("v2.project-release-comments.list-project-release-comments", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-comments.list-project-release-comments"])
	})
	t.Run("v2.project-release-comments.get-project-release-comment", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-comments.get-project-release-comment"])
	})
	t.Run("v2.project-release-comments.create-project-release-comment", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-comments.create-project-release-comment"])
	})
	t.Run("v2.project-release-comments.update-project-release-comment", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-comments.update-project-release-comment"])
	})
	t.Run("v2.project-release-comments.delete-project-release-comment", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-comments.delete-project-release-comment"])
	})
	t.Run("v2.project-release-links.list-project-release-links", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-links.list-project-release-links"])
	})
	t.Run("v2.project-release-links.get-project-release-link", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-links.get-project-release-link"])
	})
	t.Run("v2.project-release-links.create-project-release-link", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-links.create-project-release-link"])
	})
	t.Run("v2.project-release-links.update-project-release-link", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-links.update-project-release-link"])
	})
	t.Run("v2.project-release-links.delete-project-release-link", func(t *testing.T) {
		p07ReleaseRequest(t, fixtures["v2.project-release-links.delete-project-release-link"])
	})
	t.Run("v1.release.add-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-release"]) })
	t.Run("v1.release.list-releases", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-releases"]) })
	t.Run("v1.release.get-release-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-release-detail"]) })
	t.Run("v1.release.update-release-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-release-detail"]) })
	t.Run("v1.release.delete-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.delete-release"]) })
	t.Run("v1.release.add-project-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-project-release"]) })
	t.Run("v1.release.list-project-releases", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-project-releases"]) })
	t.Run("v1.release.get-project-release-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-project-release-detail"]) })
	t.Run("v1.release.update-project-release-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-project-release-detail"]) })
	t.Run("v1.release.delete-project-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.delete-project-release"]) })
	t.Run("v1.release.add-release-work-items", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-release-work-items"]) })
	t.Run("v1.release.list-release-work-items", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-release-work-items"]) })
	t.Run("v1.release.remove-release-work-items", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.remove-release-work-items"]) })
	t.Run("v1.release.add-release-label", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-release-label"]) })
	t.Run("v1.release.list-release-labels", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-release-labels"]) })
	t.Run("v1.release.get-release-label-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-release-label-detail"]) })
	t.Run("v1.release.update-release-label-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-release-label-detail"]) })
	t.Run("v1.release.delete-release-label", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.delete-release-label"]) })
	t.Run("v1.release.add-labels-to-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-labels-to-release"]) })
	t.Run("v1.release.list-labels-for-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-labels-for-release"]) })
	t.Run("v1.release.remove-labels-from-release", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.remove-labels-from-release"]) })
	t.Run("v1.release.add-release-tag", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-release-tag"]) })
	t.Run("v1.release.list-release-tags", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-release-tags"]) })
	t.Run("v1.release.get-release-tag-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-release-tag-detail"]) })
	t.Run("v1.release.update-release-tag-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-release-tag-detail"]) })
	t.Run("v1.release.delete-release-tag", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.delete-release-tag"]) })
	t.Run("v1.release.add-release-comment", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-release-comment"]) })
	t.Run("v1.release.list-release-comments", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-release-comments"]) })
	t.Run("v1.release.get-release-comment-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-release-comment-detail"]) })
	t.Run("v1.release.update-release-comment-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-release-comment-detail"]) })
	t.Run("v1.release.delete-release-comment", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.delete-release-comment"]) })
	t.Run("v1.release.add-release-link", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.add-release-link"]) })
	t.Run("v1.release.list-release-links", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.list-release-links"]) })
	t.Run("v1.release.get-release-link-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-release-link-detail"]) })
	t.Run("v1.release.update-release-link-detail", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-release-link-detail"]) })
	t.Run("v1.release.delete-release-link", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.delete-release-link"]) })
	t.Run("v1.release.get-release-changelog", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.get-release-changelog"]) })
	t.Run("v1.release.update-release-changelog", func(t *testing.T) { p07ReleaseRequest(t, fixtures["v1.release.update-release-changelog"]) })
}

func p07ReleaseRequest(t *testing.T, fixture p07ReleaseFixture) {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, failureStatus := range []int{0, 402, 404} {
		t.Run(fmt.Sprint(failureStatus), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != fixture.Method || r.URL.RequestURI() != fixture.Path {
					t.Errorf("request = %s %s; want %s %s", r.Method, r.URL.RequestURI(), fixture.Method, fixture.Path)
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
						t.Errorf("unexpected body: %s", body)
					}
				} else {
					var got, want any
					if json.Unmarshal(body, &got) != nil || json.Unmarshal(fixture.Body, &want) != nil || !reflect.DeepEqual(got, want) {
						t.Errorf("body = %s; want %s", body, fixture.Body)
					}
				}
				if failureStatus != 0 {
					if fixture.Version == "v2" {
						w.Header().Set("Content-Type", "application/problem+json")
						w.WriteHeader(failureStatus)
						_, _ = fmt.Fprintf(w, `{"type":"about:blank","title":"Release unavailable","status":%d,"detail":"Native release entitlement or scope failure"}`, failureStatus)
					} else {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(failureStatus)
						_, _ = io.WriteString(w, `{"error":"Release unavailable"}`)
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(fixture.Status)
				if fixture.Status != 204 {
					_, _ = w.Write(fixture.Response)
				}
			}))
			defer server.Close()
			args := append(append([]string{}, fixture.Command...), "--api-version", fixture.Version, "--base-url", server.URL)
			for flag, value := range fixture.Flags {
				args = append(args, "--"+flag+"="+value)
			}
			if string(fixture.Body) != "null" {
				args = append(args, "--body-file", collaborationBodyFile(t, fixture.Body))
			}
			if fixture.Destructive {
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 2 || calls != 0 || out != "" || !strings.Contains(stderr, "confirmation_required") {
					t.Fatalf("confirmation code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
				code, out, stderr = invoke(t, catalog, append(append([]string{}, args...), "--dry-run"), nil)
				if code != 0 || calls != 0 || stderr != "" || !strings.Contains(out, `"requires_confirmation":true`) {
					t.Fatalf("dry-run code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
				args = append(args, "--yes")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if failureStatus != 0 {
				if code != 5 || calls != 1 || out != "" || !strings.Contains(stderr, fmt.Sprint(failureStatus)) {
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
			if fixture.Command[len(fixture.Command)-1] == "list" {
				code, out, stderr := invoke(t, catalog, append(append([]string{}, args...), "--all"), nil)
				if code != 0 || calls != 2 || stderr != "" || !strings.Contains(out, `"complete":true`) {
					t.Fatalf("all code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
			}
			if fixture.Handler == "release-list" || fixture.Handler == "release-get" || fixture.Handler == "release-project-list" || fixture.Handler == "release-project-get" {
				code, out, stderr := invoke(t, catalog, append(append([]string{}, args...), "--output", "table"), nil)
				if code != 0 || stderr != "" || !strings.Contains(out, "Ship 2026.11") || !strings.Contains(out, "2026-11-15") {
					t.Fatalf("table code=%d out=%s err=%s", code, out, stderr)
				}
			}
		})
	}
}

func TestP07ReleaseValidation(t *testing.T) {
	transportEnv(t)
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		body string
	}{
		{"workspace-project-separation", []string{"release", "get", "--api-version", "v2", "--workspace", "my-team", "--release", "id", "--project", "ENG"}, ""},
		{"v2-project-child-requires-project", []string{"release", "project", "comment", "get", "--api-version", "v2", "--workspace", "my-team", "--release", "id", "--comment", "id"}, ""},
		{"workspace-metadata-is-string", []string{"release", "link", "create", "--api-version", "v2", "--workspace", "my-team", "--release", "id"}, `{"title":"Build","url":"https://example.com","metadata":{}}`},
		{"project-metadata-is-object", []string{"release", "project", "link", "create", "--api-version", "v2", "--workspace", "my-team", "--project", "ENG", "--release", "id"}, `{"title":"Build","url":"https://example.com","metadata":"wrong"}`},
		{"v2-does-not-translate-lead", []string{"release", "create", "--api-version", "v2", "--workspace", "my-team"}, `{"name":"Ship","lead":"id"}`},
		{"v1-does-not-translate-lead", []string{"release", "create", "--api-version", "v1", "--workspace", "my-team"}, `{"name":"Ship","lead_id":"id"}`},
		{"v2-catalog-project-not-supported", []string{"release", "label-catalog", "create", "--api-version", "v2", "--workspace", "my-team"}, `{"name":"Label","project":"ENG"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--dry-run", "--base-url", "http://127.0.0.1:1")
			if tc.body != "" {
				args = append(args, "--body-file", collaborationBodyFile(t, json.RawMessage(tc.body)))
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 2 || out != "" {
				t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
			}
		})
	}
}

func TestP07ReleaseReadProjections(t *testing.T) {
	transportEnv(t)
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"v1", "v2"} {
		for _, project := range []bool{false, true} {
			for _, action := range []string{"list", "get"} {
				path := []string{"release"}
				if project {
					path = append(path, "project")
				}
				path = append(path, action)
				found := false
				for _, command := range catalog.Commands {
					if reflect.DeepEqual(command.Path, path) {
						columns := command.Versions[version].TableColumns
						if !reflect.DeepEqual(columns, []string{"id", "name", "status", "target_date", "release_date"}) {
							t.Fatalf("%s %s columns = %v", path, version, columns)
						}
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %v", path)
				}
			}
		}
	}
}

func TestP07ReleaseAddOnlyMembership(t *testing.T) {
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []bool{false, true} {
		for _, resource := range []string{"label", "work-item"} {
			t.Run(fmt.Sprintf("%t/%s", project, resource), func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				calls := 0
				path := "/api/v2/workspaces/my-team/"
				command := []string{"release"}
				if project {
					path += "projects/ENG/"
					command = append(command, "project")
				}
				path += "releases/b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53/"
				if resource == "label" {
					path += "labels/"
				} else {
					path += "work-items/"
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != "POST" || r.URL.Path != path {
						t.Errorf("request = %s %s", r.Method, r.URL.Path)
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					var got map[string]any
					if json.Unmarshal(body, &got) != nil || !reflect.DeepEqual(got, map[string]any{"add": []any{"16c61a3a-512a-48ac-b0be-b6b46fe6f430"}}) {
						t.Errorf("body = %s", body)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"added":[],"removed":[]}`)
				}))
				defer server.Close()
				args := append(command, resource, "manage", "--api-version", "v2", "--workspace", "my-team", "--release", "b7e42a19-3c5d-4f80-9a26-8d1c0f4e7b53", "--base-url", server.URL, "--body-file", collaborationBodyFile(t, json.RawMessage(`{"add":["16c61a3a-512a-48ac-b0be-b6b46fe6f430"]}`)))
				if project {
					args = append(args, "--project", "ENG")
				}
				code, out, stderr := invoke(t, catalog, args, nil)
				if code != 0 || calls != 1 || stderr != "" || !strings.Contains(out, `"added":[]`) {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
			})
		}
	}
}
