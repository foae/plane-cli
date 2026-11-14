package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/spec"
)

func authCLI(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	c, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := run(context.Background(), args, Streams{In: strings.NewReader(input), Out: &out, Err: &stderr}, Build{}, c, nil)
	return code, out.String(), stderr.String()
}
func authEnv(t *testing.T) {
	t.Helper()
	transportEnv(t)
	for _, key := range []string{"OAUTH_CLIENT_ID", "OAUTH_CLIENT_SECRET", "REDIRECT_URI", "SCOPES", "INSTALLATION_ID"} {
		t.Setenv("PLANE_CLI_"+key, "")
	}
}
func addAuthConnection(t *testing.T, base, mode, backend string) {
	t.Helper()
	code, out, stderr := authCLI(t, "", "connection", "add", "saved", "--base-url", base, "--api-version", "v2", "--auth-mode", mode, "--credential-backend", backend)
	if code != 0 {
		t.Fatalf("add %d %s %s", code, out, stderr)
	}
}

func TestTokenImportStatusAndDeletion(t *testing.T) {
	authEnv(t)
	addAuthConnection(t, "https://example.test", "api-key", "file")
	code, out, stderr := authCLI(t, "private-token\n", "auth", "login", "--connection", "saved", "--api-key-file", "-")
	if code != 0 || strings.Contains(out+stderr, "private-token") || !strings.Contains(stderr, "plaintext_credentials") {
		t.Fatalf("login %d %s %s", code, out, stderr)
	}
	code, out, stderr = authCLI(t, "", "auth", "status", "--connection", "saved")
	if code != 0 || !strings.Contains(out, `"stored"`) || strings.Contains(out+stderr, "private-token") {
		t.Fatalf("status %d %s %s", code, out, stderr)
	}
	t.Setenv("PLANE_CLI_BASE_URL", "invalid URL")
	t.Setenv("PLANE_CLI_TIMEOUT", "invalid")
	code, out, stderr = authCLI(t, "", "auth", "logout", "--connection", "saved", "--yes")
	if code != 0 {
		t.Fatalf("logout %d %s %s", code, out, stderr)
	}
	code, out, stderr = authCLI(t, "", "connection", "delete", "saved", "--yes")
	if code != 0 {
		t.Fatalf("delete %d %s %s", code, out, stderr)
	}
}
func TestHeadlessUserLoginAndProtectedInputs(t *testing.T) {
	authEnv(t)
	var calls atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("code") != "private-code" || r.Form.Get("client_secret") != "private-secret" {
			t.Error("incorrect exchange form")
		}
		_, _ = fmt.Fprint(w, `{"access_token":"private-token","refresh_token":"private-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer provider.Close()
	addAuthConnection(t, provider.URL, "oauth-user", "file")
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLANE_CLI_CA_FILE", caFile)
	code, out, stderr := authCLI(t, "private-secret", "auth", "login", "--connection", "saved", "--flow", "user", "--headless", "--client-id", "client", "--client-secret-file", "-", "--redirect-uri", "https://callback.example/callback")
	if code != 0 || strings.Contains(out+stderr, "private-secret") || !strings.Contains(stderr, "plaintext_credentials") {
		t.Fatalf("begin %d %s %s", code, out, stderr)
	}
	var pending struct {
		Data struct {
			ID      string `json:"login_id"`
			Consent string `json:"consent_url"`
			Status  string `json:"status"`
		}
	}
	if err := json.Unmarshal([]byte(out), &pending); err != nil || pending.Data.Status != "pending" {
		t.Fatal("no pending login output")
	}
	u, err := url.Parse(pending.Data.Consent)
	if err != nil {
		t.Fatal(err)
	}
	wrongCallback := "https://callback.example/callback?state=private-wrong-state&code=private-code"
	code, out, stderr = authCLI(t, wrongCallback, "auth", "complete", "--connection", "saved", "--login-id", pending.Data.ID, "--callback-file", "-")
	if code != 3 || !strings.Contains(stderr, "state") || calls.Load() != 0 {
		t.Fatalf("state validation %d %s %s", code, out, stderr)
	}
	if strings.Contains(out+stderr, "private-wrong-state") || strings.Contains(out+stderr, "private-code") {
		t.Fatal("state guidance exposed callback secrets")
	}
	callback := "https://callback.example/callback?" + url.Values{"state": {u.Query().Get("state")}, "code": {"private-code"}}.Encode()
	code, out, stderr = authCLI(t, callback, "auth", "complete", "--connection", "saved", "--login-id", pending.Data.ID, "--callback-file", "-")
	if code != 0 || calls.Load() != 1 {
		t.Fatalf("complete %d %s %s", code, out, stderr)
	}
	for _, secret := range []string{"private-secret", "private-token", "private-refresh", "private-code", u.Query().Get("state")} {
		if strings.Contains(out+stderr, secret) {
			t.Fatal("completed auth exposed secret")
		}
	}
	code, _, _ = authCLI(t, callback, "auth", "complete", "--connection", "saved", "--login-id", pending.Data.ID, "--callback-file", "-")
	if code == 0 || calls.Load() != 1 {
		t.Fatal("callback replayed")
	}
	for _, kind := range []string{"permissions", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := privateTempDir(t)
			path := filepath.Join(dir, "secret")
			switch kind {
			case "permissions":
				if err = os.WriteFile(path, []byte("secret"), 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(dir, "target")
				if err = os.WriteFile(target, []byte("secret"), 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err = os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = protectedInput(context.Background(), path, strings.NewReader("")); err == nil {
				t.Fatal("unsafe protected input accepted")
			}
		})
	}
}
func TestEnvBackedLoginFailsBeforeHTTP(t *testing.T) {
	authEnv(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	addAuthConnection(t, server.URL, "oauth-user", "env")
	code, out, stderr := authCLI(t, "secret", "auth", "login", "--connection", "saved", "--flow", "user", "--client-secret-file", "-")
	if code != 1 || out != "" || !strings.Contains(stderr, "persistent credential backend") || calls.Load() != 0 {
		t.Fatalf("env login %d %s %s", code, out, stderr)
	}
}

type consentWriter struct{ urls chan string }

func (w consentWriter) Write(p []byte) (int, error) {
	s := string(p)
	if strings.HasPrefix(s, "Authorize this registered app: ") {
		w.urls <- strings.TrimSpace(strings.TrimPrefix(s, "Authorize this registered app: "))
	}
	return len(p), nil
}
func TestLocalConsentBindingCancellationAndCompletion(t *testing.T) {
	for _, scenario := range []string{"bind-failure", "cancel", "success", "empty-path", "denied"} {
		t.Run(scenario, func(t *testing.T) {
			authEnv(t)
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = fmt.Fprint(w, `{"access_token":"private-token","token_type":"Bearer","expires_in":3600}`)
			}))
			defer provider.Close()
			addAuthConnection(t, provider.URL, "oauth-user", "file")
			held, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			redirect := "http://" + held.Addr().String() + "/callback"
			if scenario == "empty-path" {
				redirect = "http://" + held.Addr().String()
			}
			defer func() { _ = held.Close() }()
			if scenario != "bind-failure" {
				_ = held.Close()
			}
			c, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			urls := make(chan string, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			done := make(chan int, 1)
			var out bytes.Buffer
			go func() {
				done <- run(ctx, []string{"auth", "login", "--connection", "saved", "--flow", "user", "--client-id", "client", "--client-secret-file", "-", "--redirect-uri", redirect}, Streams{In: strings.NewReader("private-secret"), Out: &out, Err: consentWriter{urls}}, Build{}, c, nil)
			}()
			if scenario == "bind-failure" {
				select {
				case code := <-done:
					if code == 0 || len(urls) != 0 || calls.Load() != 0 {
						t.Fatal("consent exposed before bind")
					}
				case <-ctx.Done():
					t.Fatal("bind failure hung")
				}
				return
			}
			var consent string
			select {
			case consent = <-urls:
			case <-ctx.Done():
				t.Fatal("no consent instructions")
			}
			if scenario == "cancel" {
				cancel()
			} else {
				u, _ := url.Parse(consent)
				q := url.Values{"state": {u.Query().Get("state")}, "code": {"private-code"}}
				client := &http.Client{Timeout: 5 * time.Second}
				bad, err := client.Get(redirect + "?state=wrong&code=unrelated")
				if err != nil {
					t.Fatal(err)
				}
				_, _ = io.Copy(io.Discard, bad.Body)
				_ = bad.Body.Close()
				if bad.StatusCode != 400 || calls.Load() != 0 {
					t.Fatal("invalid callback consumed consent")
				}
				if scenario == "denied" {
					q.Del("code")
					q.Set("error", "denied")
				}
				response, err := client.Get(redirect + "?" + q.Encode())
				if err != nil {
					t.Fatal(err)
				}
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
			}
			select {
			case code := <-done:
				want := 0
				switch scenario {
				case "cancel":
					want = 130
				case "denied":
					want = 3
				}
				if code != want {
					t.Fatalf("completion code %d want %d", code, want)
				}
			case <-ctx.Done():
				if scenario != "cancel" {
					t.Fatal("callback did not terminate login")
				}
				select {
				case code := <-done:
					if code != 130 {
						t.Fatal("cancel status")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("cancel hung")
				}
			}
			success := scenario == "success" || scenario == "empty-path"
			if success && calls.Load() != 1 || !success && calls.Load() != 0 {
				t.Fatal("incorrect exchange count")
			}
			conn, err := net.DialTimeout("tcp", held.Addr().String(), time.Second)
			if err == nil {
				_ = conn.Close()
				t.Fatal("callback listener remained open")
			}
		})
	}
}

func TestProtectedInputCancellation(t *testing.T) {
	authEnv(t)
	addAuthConnection(t, "https://example.test", "api-key", "file")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"auth", "login", "--connection", "saved", "--api-key-file", "-"}, Streams{In: reader, Out: &out, Err: &stderr}, Build{}, catalog, nil)
	}()
	// Pipe writes synchronize with the blocked input read, without sleeps.
	if _, err := writer.Write([]byte("partial-secret")); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case code := <-done:
		if code != 130 || strings.Contains(out.String()+stderr.String(), "partial-secret") {
			t.Fatalf("cancel returned %d: %s %s", code, out.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("protected input ignored cancellation")
	}
}

func TestAuthInputEffectiveDeadline(t *testing.T) {
	authEnv(t)
	addAuthConnection(t, "https://example.test", "api-key", "file")
	t.Setenv("PLANE_CLI_TIMEOUT", "10ms")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{"auth", "login", "--connection", "saved", "--api-key-file", "-"}, Streams{In: reader, Out: &out, Err: &stderr}, Build{}, catalog, nil)
	if code != 4 || !strings.Contains(stderr.String(), `"deadline_exceeded"`) {
		t.Fatalf("deadline returned %d: %s %s", code, out.String(), stderr.String())
	}
}
