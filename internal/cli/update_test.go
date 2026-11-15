package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/spec"
)

type updateRoundTripper func(*http.Request) (*http.Response, error)

func (f updateRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The client sees only the immutable production URL. Its test transport sends
// those requests to a verified local TLS server, without an endpoint flag.
func updateTestClient(t *testing.T, h http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewTLSServer(h)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	return &http.Client{Transport: updateRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "api.github.com" || r.URL.Path != "/repos/foae/plane-cli/releases" {
			t.Errorf("unexpected update destination: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials sent to GitHub")
		}
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Scheme, u.Host = target.Scheme, target.Host
		clone.URL = &u
		return transport.RoundTrip(clone)
	})}
}

func updateRelease(tag string) map[string]any {
	return map[string]any{"tag_name": tag, "draft": false, "prerelease": strings.Contains(strings.SplitN(tag, "+", 2)[0], "-"), "published_at": "2026-01-02T03:04:05Z", "html_url": "https://github.com/foae/plane-cli/releases/tag/" + url.PathEscape(tag)}
}

func updateInvoke(t *testing.T, ctx context.Context, client *http.Client, current string, args ...string) (int, string, string) {
	t.Helper()
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	handlers := map[string]handler{"update-check": updateCheckHandler(Build{Version: current}, &out, client)} //nolint:contextcheck // Factory does no IO; its callback derives the deadline from cmd.Context().
	code := run(ctx, append([]string{"update", "check"}, args...), Streams{In: strings.NewReader(""), Out: &out, Err: &stderr}, Build{Version: current}, catalog, handlers)
	return code, out.String(), stderr.String()
}

func TestUpdateSelection(t *testing.T) {
	for _, tc := range []struct {
		name, current, latest, status string
		include                       bool
		available                     any
	}{
		{"stable-numeric", "1.9.0", "1.10.0", "update_available", false, true},
		{"equal", "v1.10.0", "1.10.0", "up_to_date", false, false},
		{"newer-current", "2.0.0", "1.10.0", "up_to_date", false, false},
		{"preview-numeric", "1.10.0", "2.0.0-rc.10", "update_available", true, true},
		{"development", "dev", "1.10.0", "unknown_current", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := updateTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Query().Get("per_page") != "100" || r.URL.Query().Get("page") != "1" {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				_ = json.NewEncoder(w).Encode([]any{updateRelease("v1.9.0"), updateRelease("v2.0.0-rc.2"), updateRelease("v1.10.0"), updateRelease("v2.0.0-rc.10"), map[string]any{"draft": true}, updateRelease("nightly")})
			})
			args := []string{}
			if tc.include {
				args = append(args, "--include-prerelease")
			}
			code, stdout, stderr := updateInvoke(t, context.Background(), client, tc.current, args...)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			var envelope struct {
				SchemaVersion int            `json:"schema_version"`
				Data          map[string]any `json:"data"`
				Meta          map[string]any `json:"meta"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.SchemaVersion != 1 || len(envelope.Data) != 6 || envelope.Data["current_version"] != tc.current || envelope.Data["latest_version"] != tc.latest || envelope.Data["status"] != tc.status || envelope.Data["update_available"] != tc.available || envelope.Meta["command"] != "update check" || envelope.Meta["status"] != "completed" || len(envelope.Meta) != 2 {
				t.Fatalf("unexpected envelope: %s", stdout)
			}
			channel := "stable"
			if tc.include {
				channel = "prerelease"
			}
			if envelope.Data["channel"] != channel || envelope.Data["release_url"] != "https://github.com/foae/plane-cli/releases/tag/v"+tc.latest {
				t.Fatalf("channel/url: %s", stdout)
			}
		})
	}
	for _, current := range []string{"1.0.0", "dev"} {
		client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `[]`) })
		code, out, stderr := updateInvoke(t, context.Background(), client, current)
		status := "no_release"
		if current == "dev" {
			status = "unknown_current"
		}
		if code != 0 || stderr != "" || !strings.Contains(out, `"status":"`+status+`"`) || !strings.Contains(out, `"latest_version":null`) || !strings.Contains(out, `"release_url":null`) {
			t.Fatalf("empty: %d %s %s", code, out, stderr)
		}
	}
}

func TestUpdateMetadataErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		status, exit int
		headers      map[string]string
	}{
		{"invalid-json", `[`, 200, 5, nil},
		{"null", `null`, 200, 5, nil},
		{"object", `{}`, 200, 5, nil},
		{"missing-draft", `[{"tag_name":"v1.0.0"}]`, 200, 5, nil},
		{"duplicate", `[{"draft":true,"draft":false}]`, 200, 5, nil},
		{"trailing", `[] []`, 200, 5, nil},
		{"oversized", strings.Repeat(" ", updateMaxBytes) + `[]`, 200, 5, nil},
		{"rate-limit", `PRIVATE reflected response`, 429, 5, nil},
		{"github-rate-limit", `PRIVATE`, 403, 5, map[string]string{"X-RateLimit-Remaining": "0"}},
		{"secondary-rate-limit", `PRIVATE`, 403, 5, map[string]string{"Retry-After": "60"}},
		{"forbidden", `PRIVATE`, 403, 3, nil},
		{"unauthorized", `PRIVATE`, 401, 3, nil},
		{"not-found", `PRIVATE`, 404, 5, nil},
		{"server-failure", `PRIVATE`, 503, 5, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
			if code != tc.exit || out != "" || strings.Contains(stderr, "PRIVATE") || strings.Contains(stderr, `"api_version"`) {
				t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
			}
			if tc.exit == 3 && !strings.Contains(stderr, `"code":"auth_error"`) {
				t.Fatalf("authentication error code: %s", stderr)
			}
		})
	}
	for _, field := range []string{"html_url", "published_at", "prerelease", "tag_name"} {
		t.Run(field, func(t *testing.T) {
			client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				r := updateRelease("v2.0.0")
				r[field] = "https://evil.invalid/PRIVATE"
				_ = json.NewEncoder(w).Encode([]any{r})
			})
			code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
			// Non-SemVer string tags are deliberately ignored, not protocol failures.
			if field == "tag_name" {
				if code != 0 || !strings.Contains(out, `"status":"no_release"`) {
					t.Fatalf("ignored tag: %d %s %s", code, out, stderr)
				}
				return
			}
			if code != 5 || out != "" || strings.Contains(stderr, "PRIVATE") {
				t.Fatalf("metadata: %d %s %s", code, out, stderr)
			}
		})
	}
	for _, badURL := range []string{"http://github.com/foae/plane-cli/releases/tag/v2.0.0", "https://github.com.evil.invalid/foae/plane-cli/releases/tag/v2.0.0", "https://github.com/other/plane-cli/releases/tag/v2.0.0", "https://github.com/foae/plane-cli/releases/tag/v1.0.0", "https://github.com/foae/plane-cli/releases/tag/v2.0.0?token=PRIVATE", "https://github.com/foae/plane-cli/releases/tag/v2.0.0#PRIVATE"} {
		client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			r := updateRelease("v2.0.0")
			r["html_url"] = badURL
			_ = json.NewEncoder(w).Encode([]any{r})
		})
		code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
		if code != 5 || out != "" || strings.Contains(stderr, "PRIVATE") {
			t.Fatalf("URL: %d %s %s", code, out, stderr)
		}
	}
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, location := range []string{target.URL, updateAPI} {
		client := updateTestClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, location, http.StatusFound) })
		code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
		if code != 5 || out != "" || redirected.Load() != 0 {
			t.Fatalf("redirect: %d %s %s calls=%d", code, out, stderr, redirected.Load())
		}
	}
}

func TestUpdatePagination(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bound bool
	}{{"numeric-across-pages", false}, {"scan-bound", true}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := updateTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page != calls {
					t.Errorf("nonsequential pages: %d/%d", page, calls)
				}
				if tc.bound || page == 1 {
					w.Header().Set("Link", fmt.Sprintf(`<%s?per_page=100&page=%d>; rel="next"`, updateAPI, page+1))
				}
				tag := "v1.9.0"
				if page == 2 {
					tag = "v1.10.0"
				}
				_ = json.NewEncoder(w).Encode([]any{updateRelease(tag)})
			})
			code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0", "--include-prerelease")
			if tc.bound {
				if code != 5 || out != "" || calls != updateMaxPages || !strings.Contains(stderr, "10-page") {
					t.Fatalf("bound: %d %d %s %s", code, calls, out, stderr)
				}
				return
			}
			if code != 0 || calls != 2 || !strings.Contains(out, `"latest_version":"1.10.0"`) {
				t.Fatalf("pages: %d %d %s %s", code, calls, out, stderr)
			}
		})
	}
	for _, link := range []string{`<https://evil.invalid/releases?page=2&per_page=100>; rel="next"`, `<` + updateAPI + `?page=1&per_page=100>; rel="next"`, `<` + updateAPI + `?page=2&per_page=100&token=PRIVATE>; rel="next"`, `invalid`} {
		calls := 0
		client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.Header().Set("Link", link)
			_, _ = io.WriteString(w, `[]`)
		})
		code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
		if code != 5 || out != "" || calls != 1 || strings.Contains(stderr, "PRIVATE") {
			t.Fatalf("link: %d %d %s %s", code, calls, out, stderr)
		}
	}
}

func TestUpdateCancellationAndTimeout(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		started := make(chan struct{})
		client := updateTestClient(t, func(_ http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() })
		ctx, cancel := context.WithCancel(context.Background())
		args := []string{}
		if deadline {
			args = append(args, "--timeout", "20ms")
		} else {
			go func() { <-started; cancel() }()
		}
		code, out, stderr := updateInvoke(t, ctx, client, "1.0.0", args...)
		cancel()
		want := 130
		if deadline {
			want = 4
		}
		if code != want || out != "" {
			t.Fatalf("cancellation: %d %s %s", code, out, stderr)
		}
	}
	calls := 0
	client := &http.Client{Transport: updateRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("PRIVATE transport error")
	})}
	for _, raw := range []string{"0", "-1s", "bad"} {
		code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0", "--timeout", raw)
		if code != 2 || out != "" || calls != 0 {
			t.Fatalf("timeout: %d %s %s calls=%d", code, out, stderr, calls)
		}
	}
	code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
	if code != 4 || out != "" || strings.Contains(stderr, "PRIVATE") {
		t.Fatalf("transport: %d %s %s", code, out, stderr)
	}
}

func TestUpdateIsolation(t *testing.T) {
	home := t.TempDir()
	poison := filepath.Join(home, "not-a-config-directory")
	if err := os.WriteFile(poison, []byte("PRIVATE invalid config"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PLANE_CLI_CONFIG_DIR", poison)
	for _, key := range []string{"API_KEY", "ACCESS_TOKEN", "AUTH_MODE", "API_VERSION", "CA_FILE", "TIMEOUT", "REQUEST_TIMEOUT", "CREDENTIAL_BACKEND", "CONNECTION"} {
		t.Setenv("PLANE_CLI_"+key, "PRIVATE invalid setting")
	}
	var planeCalls atomic.Int32
	plane := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { planeCalls.Add(1) }))
	defer plane.Close()
	t.Setenv("PLANE_CLI_BASE_URL", plane.URL)
	client := updateTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if len(r.Cookies()) != 0 {
			t.Error("cookies forwarded")
		}
		_, _ = io.WriteString(w, `[]`)
	})
	code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || stderr != "" || planeCalls.Load() != 0 || len(entries) != 1 || entries[0].Name() != "not-a-config-directory" {
		t.Fatalf("isolation: %d %s %s calls=%d entries=%v", code, out, stderr, planeCalls.Load(), entries)
	}
	for _, args := range [][]string{{"--base-url", plane.URL}, {"--connection", "private"}, {"--api-version", "v2"}, {"--ca-file", "private"}, {"--dry-run"}} {
		code, out, stderr = updateInvoke(t, context.Background(), client, "1.0.0", args...)
		if code != 2 || out != "" {
			t.Fatalf("irrelevant flag: %d %s %s", code, out, stderr)
		}
	}
}

func TestUpdateOfflineProcess(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "plane-cli")
	build := exec.Command("go", "build", "-o", binary, "./cmd/plane-cli")
	build.Dir = "../.."
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusBadGateway) }))
	defer proxy.Close()
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "config"), "PLANE_CLI_CONFIG_DIR="+filepath.Join(home, "plane-config"), "HTTP_PROXY="+proxy.URL, "HTTPS_PROXY="+proxy.URL, "ALL_PROXY="+proxy.URL, "NO_PROXY=", "no_proxy=")
	for _, args := range [][]string{nil, {"--help"}, {"version"}, {"--version"}, {"help", "update", "check"}, {"update", "check", "--help"}, {"__complete", "update", "ch"}, {"commands", "update", "--api-version", "v2"}, {"schema", "update", "check", "--api-version", "v1"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = env
		data, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("offline %v: %v\n%s", args, err, data)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 || len(entries) != 0 {
		t.Fatalf("offline commands: requests=%d writes=%v", requests.Load(), entries)
	}
}

func TestUpdateNoEligibleRelease(t *testing.T) {
	client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{map[string]any{"draft": true}, updateRelease("nightly"), updateRelease("v3.0.0-beta.1")})
	})
	code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
	if code != 0 || stderr != "" || !strings.Contains(out, `"status":"no_release"`) || !strings.Contains(out, `"update_available":false`) {
		t.Fatalf("eligible release: %d %s %s", code, out, stderr)
	}
	code, out, stderr = updateInvoke(t, context.Background(), client, "1.0.0", "--include-prerelease")
	if code != 0 || stderr != "" || !strings.Contains(out, `"latest_version":"3.0.0-beta.1"`) {
		t.Fatalf("preview release: %d %s %s", code, out, stderr)
	}
}

func TestUpdatePaginationPageSize(t *testing.T) {
	for _, count := range []int{updatePageSize, updatePageSize + 1} {
		calls := 0
		client := updateTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			calls++
			releases := []any{}
			if calls == 1 {
				for range count {
					releases = append(releases, updateRelease("v1.0.0"))
				}
			}
			_ = json.NewEncoder(w).Encode(releases)
		})
		code, out, stderr := updateInvoke(t, context.Background(), client, "1.0.0")
		if count > updatePageSize {
			if code != 5 || out != "" || calls != 1 {
				t.Fatalf("oversized page: %d %d %s %s", code, calls, out, stderr)
			}
		} else if code != 0 || calls != 2 || stderr != "" {
			t.Fatalf("full page without continuation: %d %d %s %s", code, calls, out, stderr)
		}
	}
}
