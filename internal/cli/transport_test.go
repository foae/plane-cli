package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/foae/plane-cli/internal/config"
	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

func TestCurrentUser(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_VERSION", version)
			t.Setenv("PLANE_CLI_API_KEY", "identity-canary")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.RequestURI() != "/prefix/api/"+version+"/users/me/" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
				}
				if r.Header.Get("X-Api-Key") != "identity-canary" || r.Header.Get("Authorization") != "" {
					t.Error("wrong identity authentication")
				}
				w.Header().Set("Content-Type", "application/json")
				if calls > 1 {
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"detail":"identity-private-canary"}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"principal","unknown":9007199254740993,"scopes":[]}`))
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, stdout, stderr := invoke(t, catalog, []string{"user", "me", "--base-url", server.URL + "/prefix"}, nil)
			if code != 0 || stderr != "" || calls != 1 || !strings.Contains(stdout, `"unknown":9007199254740993`) || !strings.Contains(stdout, `"id":"principal"`) {
				t.Fatalf("identity: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
			code, stdout, stderr = invoke(t, catalog, []string{"user", "me", "--base-url", server.URL + "/prefix"}, nil)
			if code != 5 || calls != 2 || stdout != "" || !strings.Contains(stderr, "api_error") || strings.Contains(stderr, "identity-private-canary") {
				t.Fatalf("unsafe upstream rejection: code=%d calls=%d out=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func transportCatalog(t *testing.T) (spec.Catalog, map[string]handler) {
	t.Helper()
	c, e := spec.Load()
	if e != nil {
		t.Fatal(e)
	}
	props := map[string]any{}
	flags := []spec.Flag{}
	for _, name := range []string{"api-version", "base-url", "auth-mode", "credential-backend", "connection", "timeout", "request-timeout", "ca-file", "yes", "dry-run"} {
		typ := "string"
		if name == "yes" || name == "dry-run" {
			typ = "boolean"
		}
		props[name] = map[string]any{"type": typ}
		flags = append(flags, spec.Flag{Name: name, Type: typ, Required: name == "api-version", Binding: "/flags/" + name, Description: name})
	}
	schema, e := json.Marshal(map[string]any{"type": "object", "properties": map[string]any{"arguments": map[string]any{"type": "array", "maxItems": 0}, "flags": map[string]any{"type": "object", "properties": props, "required": []string{"api-version"}, "additionalProperties": false}}, "required": []string{"arguments", "flags"}})
	if e != nil {
		t.Fatal(e)
	}
	c.Commands = append(c.Commands, spec.Command{Path: []string{"transport-probe"}, Handler: "transport-probe", Summary: "Test-only transport endpoint", SideEffect: "delete", Versions: map[string]spec.Variant{"v2": {Schema: schema, Flags: flags}}})
	return c, map[string]handler{"transport-probe": func(cmd *cobra.Command, _ []string, _ map[string]any) error {
		return apiRequest(cmd, http.MethodDelete, []string{"api", "v2", "test", "space id"}, nil, nil, true)
	}}
}
func transportEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PLANE_CLI_CONFIG_DIR", filepath.Join(privateTempDir(t), "config"))
	for _, name := range []string{"CONNECTION", "BASE_URL", "API_VERSION", "API_KEY", "ACCESS_TOKEN", "AUTH_MODE", "CREDENTIAL_BACKEND", "TIMEOUT", "REQUEST_TIMEOUT", "CA_FILE"} {
		t.Setenv("PLANE_CLI_"+name, "")
	}
}
func TestTransportCLI(t *testing.T) {
	transportEnv(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.EscapedPath() != "/prefix/api/v2/test/space%20id/" {
			t.Errorf("path %s", r.URL.EscapedPath())
		}
		if r.Header.Get("X-Api-Key") != "canary" || r.Header.Get("Authorization") != "" {
			t.Error("wrong auth")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":9007199254740993,"unknown":false}`))
	}))
	defer server.Close()
	c, h := transportCatalog(t)
	base := []string{"transport-probe", "--api-version", "v2", "--base-url", server.URL + "/prefix"}
	t.Setenv("PLANE_CLI_API_KEY", "canary")
	t.Setenv("PLANE_CLI_ACCESS_TOKEN", "ambiguous")
	for _, tail := range [][]string{{}, {"--dry-run"}, {"--yes", "--unknown"}} {
		code, out, stderr := invoke(t, c, append(append([]string{}, base...), tail...), h)
		if len(tail) > 0 && tail[0] == "--dry-run" {
			if code != 0 || !strings.Contains(out, `"requires_confirmation":true`) {
				t.Fatalf("dry-run %d %s %s", code, out, stderr)
			}
		} else if code != 2 {
			t.Fatalf("guard %d %s", code, stderr)
		}
		if calls != 0 {
			t.Fatal("guard sent request")
		}
	}
	t.Setenv("PLANE_CLI_ACCESS_TOKEN", "")
	code, out, stderr := invoke(t, c, append(base, "--yes"), h)
	if code != 0 || !strings.Contains(out, "9007199254740993") || stderr != "" || calls != 1 {
		t.Fatalf("result %d %s %s calls=%d", code, out, stderr, calls)
	}
	var diagnostic bytes.Buffer
	code = run(context.Background(), append(base, "--yes"), Streams{In: strings.NewReader(""), Out: brokenWriter{}, Err: &diagnostic}, Build{}, c, h)
	if code != 6 || !strings.Contains(diagnostic.String(), `"remote_changed":true`) {
		t.Fatalf("mutation output failure %d %s", code, &diagnostic)
	}
}
func TestExternalProxyProcess(t *testing.T) {
	if os.Getenv("PLANE_TEST_PROXY_CHILD") == "1" {
		c, h := transportCatalog(t)
		code, out, stderr := invoke(t, c, []string{"transport-probe", "--api-version", "v2", "--base-url", "http://plane.logical.test/prefix", "--auth-mode", "external", "--yes"}, h)
		if code != 0 || !strings.Contains(out, `"injected":true`) || stderr != "" {
			t.Fatalf("proxy result %d %s %s", code, out, stderr)
		}
		return
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer proxy-owned" || r.Header.Get("X-Api-Key") != "" {
			t.Error("proxy did not supply isolated upstream authentication")
			http.Error(w, "denied", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"injected":true}`))
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "plane.logical.test" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Error("unexpected destination or leaked local auth")
		}
		r.Header.Set("Authorization", "Bearer proxy-owned")
		forward.ServeHTTP(w, r)
	}))
	defer proxy.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestExternalProxyProcess$")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "PLANE_CLI_CONFIG_DIR=" + filepath.Join(privateTempDir(t), "config"), "PLANE_TEST_PROXY_CHILD=1", "HTTP_PROXY=" + proxy.URL, "NO_PROXY=", "PLANE_CLI_API_KEY=ignored-canary", "PLANE_CLI_ACCESS_TOKEN=ignored-canary"}
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("proxy subprocess: %v %s", e, out)
	}
}
func TestConnectionCLI(t *testing.T) {
	transportEnv(t)
	c, e := spec.Load()
	if e != nil {
		t.Fatal(e)
	}
	code, _, stderr := invoke(t, c, []string{"connection", "add", "a", "--base-url", "http://example.test/prefix", "--api-version", "v2"}, nil)
	if code != 0 || !strings.Contains(stderr, `"http_connection"`) {
		t.Fatalf("add %d %s", code, stderr)
	}
	code, _, stderr = invoke(t, c, []string{"connection", "update", "a", "--workspace", "workspace"}, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("update %d %s", code, stderr)
	}
	t.Setenv("PLANE_CLI_BASE_URL", ":malformed")
	t.Setenv("PLANE_CLI_TIMEOUT", "bad")
	for _, args := range [][]string{{"connection", "list"}, {"connection", "use", "a"}, {"connection", "delete", "a", "--yes"}} {
		code, _, stderr = invoke(t, c, args, nil)
		if code != 0 {
			t.Fatalf("repair %v: %d %s", args, code, stderr)
		}
	}
}

func privateTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEffectiveVersionAndStatelessTransport(t *testing.T) {
	transportEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Error("external request forwarded credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()
	c, h := transportCatalog(t)
	code, _, stderr := invoke(t, c, []string{"connection", "add", "saved", "--base-url", server.URL, "--api-version", "v2", "--auth-mode", "external"}, h)
	if code != 0 {
		t.Fatal(stderr)
	}
	code, out, stderr := invoke(t, c, []string{"transport-probe", "--connection", "saved", "--yes"}, h)
	if code != 0 || !strings.Contains(out, `"accepted":true`) {
		t.Fatalf("saved version: %d %s %s", code, out, stderr)
	}
	t.Setenv("PLANE_CLI_BASE_URL", server.URL)
	t.Setenv("PLANE_CLI_API_VERSION", "v2")
	t.Setenv("PLANE_CLI_AUTH_MODE", "external")
	t.Setenv("PLANE_CLI_CONFIG_DIR", "")
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	code, out, stderr = invoke(t, c, []string{"transport-probe", "--yes"}, h)
	if code != 0 || !strings.Contains(out, `"accepted":true`) {
		t.Fatalf("stateless version: %d %s %s", code, out, stderr)
	}
}

type lockWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *lockWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

func TestConnectionLockInterrupt(t *testing.T) {
	for _, connectionLock := range []bool{false, true} {
		t.Run(fmt.Sprint(connectionLock), func(t *testing.T) {
			transportEnv(t)
			c, h := transportCatalog(t)
			store, err := config.Open(os.Getenv("PLANE_CLI_CONFIG_DIR"), true)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.Close() }()
			var unlock func() error
			if connectionLock {
				unlock, err = store.ConnectionLock(context.Background(), "waiting")
			} else {
				unlock, err = store.Lock(context.Background(), "config.lock")
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = unlock() }()
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &lockWaitContext{Context: parent, entered: make(chan struct{})}
			var out, diagnostics bytes.Buffer
			result := make(chan int, 1)
			go func() {
				result <- run(ctx, []string{"connection", "add", "waiting", "--base-url", "https://example.test", "--api-version", "v2"}, Streams{In: strings.NewReader(""), Out: &out, Err: &diagnostics}, Build{}, c, h)
			}()
			<-ctx.entered
			cancel()
			if code := <-result; code != 130 || out.Len() != 0 || !strings.Contains(diagnostics.String(), `"interrupted"`) {
				t.Fatalf("lock interrupt: %d %s %s", code, &out, &diagnostics)
			}
		})
	}
}

func TestConnectionValidationAndWarningFailure(t *testing.T) {
	transportEnv(t)
	c, h := transportCatalog(t)
	var out bytes.Buffer
	code := run(context.Background(), []string{"connection", "add", "saved", "--base-url", "http://example.test", "--api-version", "v2"}, Streams{In: strings.NewReader(""), Out: &out, Err: brokenWriter{}}, Build{}, c, h)
	if code != 0 || !strings.Contains(out.String(), `"completed"`) {
		t.Fatalf("informational warning changed status: %d %s", code, &out)
	}
	for _, dry := range []bool{false, true} {
		for _, pair := range [][]string{{"--timeout", "bad"}, {"--auth-mode", "unsupported"}} {
			args := append([]string{"connection", "update", "saved"}, pair...)
			if dry {
				args = append(args, "--dry-run")
			}
			code, _, stderr := invoke(t, c, args, h)
			if code != 2 || !strings.Contains(stderr, `"usage_error"`) {
				t.Fatalf("validation %v: %d %s", args, code, stderr)
			}
		}
	}
}

func TestTransportDryRunPath(t *testing.T) {
	transportEnv(t)
	c, h := transportCatalog(t)
	args := []string{"transport-probe", "--api-version", "v2", "--base-url", "https://example.test/prefix", "--dry-run"}
	code, out, stderr := invoke(t, c, args, h)
	if code != 0 || !strings.Contains(out, "https://example.test/prefix/api/v2/test/space%20id/") {
		t.Fatalf("missing dry-run target: %d %s %s", code, out, stderr)
	}
	h["transport-probe"] = func(cmd *cobra.Command, _ []string, _ map[string]any) error {
		return apiRequest(cmd, http.MethodDelete, []string{".."}, nil, nil, true)
	}
	code, out, stderr = invoke(t, c, args, h)
	if code != 2 || out != "" {
		t.Fatalf("invalid dry-run path accepted: %d %s %s", code, out, stderr)
	}
}
