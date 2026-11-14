package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/config"
)

type memoryKeyring struct {
	values              map[string]string
	failSet, failDelete atomic.Bool
}

func (k *memoryKeyring) Get(_ context.Context, key string) (string, error) {
	v, ok := k.values[key]
	if !ok {
		return "", ErrCredentialNotFound
	}
	return v, nil
}
func (k *memoryKeyring) Set(_ context.Context, key, value string) error {
	if k.failSet.Load() {
		return errors.New("PRIVATE backend error")
	}
	k.values[key] = value
	return nil
}
func (k *memoryKeyring) Delete(_ context.Context, key string) error {
	if k.failDelete.Load() {
		return errors.New("PRIVATE backend error")
	}
	delete(k.values, key)
	return nil
}
func managerFixture(t *testing.T, base, mode string) (*Manager, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := config.Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{Store: store, Keyring: &memoryKeyring{values: map[string]string{}}}
	t.Cleanup(func() { _ = m.Close() })
	id, err := config.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	err = store.Update(context.Background(), func(f *config.File) error {
		f.Connections["saved"] = config.Connection{ID: id, Generation: 1, BaseURL: base, APIVersion: "v2", AuthMode: mode, Backend: "file"}
		f.Default = "saved"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, dir
}
func currentFixture(t *testing.T, m *Manager) config.Connection {
	t.Helper()
	c, err := m.current("saved")
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func resolvedFixture(t *testing.T, m *Manager) config.Resolved {
	t.Helper()
	return config.Resolved{Name: "saved", Connection: currentFixture(t, m)}
}
func expireFixture(t *testing.T, m *Manager) {
	t.Helper()
	c := currentFixture(t, m)
	b, err := m.load(context.Background(), c.Backend, c.CredentialRef)
	if err != nil {
		t.Fatal(err)
	}
	b.Expiry = time.Unix(1, 0)
	if err = m.save(context.Background(), c.Backend, c.CredentialRef, b); err != nil {
		t.Fatal(err)
	}
}
func callbackFixture(t *testing.T, consent, mode string) string {
	t.Helper()
	u, err := url.Parse(consent)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	result := url.Values{"state": {q.Get("state")}}
	if mode == "oauth-user" {
		result.Set("code", "private-code")
	} else {
		result.Set("app_installation_id", "installation")
	}
	return q.Get("redirect_uri") + "?" + result.Encode()
}
func loginFixture(t *testing.T, m *Manager, mode string) {
	t.Helper()
	id, consent, err := m.Begin(context.Background(), "saved", LoginOptions{ClientID: "client", ClientSecret: "secret", RedirectURI: "http://127.0.0.1:45678/callback", Scopes: "read write"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Complete(context.Background(), "saved", id, callbackFixture(t, consent, mode)); err != nil {
		t.Fatal(err)
	}
}

func TestPersistedUserAndBotRenewal(t *testing.T) {
	for _, mode := range []string{"oauth-user", "oauth-bot"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if r.URL.Path != "/prefix/auth/o/token/" || r.Method != "POST" {
					t.Error("incorrect OAuth route")
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if mode == "oauth-user" {
					want := "authorization_code"
					if n > 1 {
						want = "refresh_token"
						expected := "refresh-one"
						if n > 2 {
							expected = "refresh-two"
						}
						if r.Form.Get("refresh_token") != expected {
							t.Errorf("wrong refresh token on request %d", n)
						}
					}
					if r.Form.Get("grant_type") != want || r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") != "secret" || r.Header.Get("Authorization") != "" {
						t.Error("incorrect user grant")
					}
				} else {
					id, secret, ok := r.BasicAuth()
					if !ok || id != "client" || secret != "secret" || r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("app_installation_id") != "installation" || r.Form.Get("scope") != "read write" {
						t.Error("incorrect bot grant")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				refresh := "refresh-one"
				if n == 2 {
					refresh = "refresh-two"
				}
				extra := ""
				if n < 3 {
					extra = fmt.Sprintf(",\"refresh_token\":%q", refresh)
				}
				_, _ = fmt.Fprintf(w, `{"access_token":"access-%d","token_type":"Bearer","expires_in":3600%s}`, n, extra)
			}))
			defer server.Close()
			m, _ := managerFixture(t, server.URL+"/prefix", mode)
			loginFixture(t, m, mode)
			for range 3 {
				expireFixture(t, m)
				credential, status, err := m.Resolve(context.Background(), resolvedFixture(t, m), true)
				if err != nil || credential.Header != "Authorization" || credential.Token == "" || status.Expiry == nil || status.Binding == "" {
					t.Fatalf("renewal: %v %+v", err, status)
				}
			}
			if calls.Load() != 4 {
				t.Fatalf("exchanges=%d", calls.Load())
			}
			c := currentFixture(t, m)
			b, err := m.load(context.Background(), c.Backend, c.CredentialRef)
			if err != nil || b.RefreshToken != "refresh-two" || b.ClientSecret != "secret" {
				t.Fatal("rotation or omitted refresh token lost bundle")
			}
			if err = m.Logout(context.Background(), "saved"); err != nil {
				t.Fatal(err)
			}
			_, status, err := m.Resolve(context.Background(), resolvedFixture(t, m), true)
			if err != nil || status.Status != "missing" {
				t.Fatal("logout retained usable credentials")
			}
		})
	}
}

func TestPendingCallbackValidationAndCleanup(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = fmt.Fprint(w, oauthGoodResponse) }))
	defer server.Close()
	m, _ := managerFixture(t, server.URL, "oauth-user")
	begin := func() (string, string) {
		id, consent, err := m.Begin(context.Background(), "saved", LoginOptions{ClientID: "client", ClientSecret: "secret", RedirectURI: "http://127.0.0.1:45678/callback"})
		if err != nil {
			t.Fatal(err)
		}
		return id, callbackFixture(t, consent, "oauth-user")
	}
	id, callback := begin()
	for _, bad := range []string{strings.Replace(callback, "state=", "state=bad", 1), strings.Replace(callback, "/callback", "/other", 1), callback + "&code=duplicate", strings.Replace(callback, "http:", "https:", 1), callback + "#fragment"} {
		if err := m.Complete(context.Background(), "saved", id, bad); err == nil {
			t.Fatal("invalid callback accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid callback exchanged")
	}
	if err := m.Complete(context.Background(), "saved", id, callback); err != nil {
		t.Fatal(err)
	}
	if err := m.Complete(context.Background(), "saved", id, callback); err == nil || calls.Load() != 1 {
		t.Fatal("callback replayed")
	}
	for _, kind := range []string{"denied", "expired", "cancelled", "superseded", "logout", "recreated"} {
		t.Run(kind, func(t *testing.T) {
			id, callback := begin()
			c := currentFixture(t, m)
			p, err := m.pending(c)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "denied":
				u, _ := url.Parse(callback)
				q := u.Query()
				q.Del("code")
				q.Set("error", "PRIVATE provider detail")
				u.RawQuery = q.Encode()
				callback = u.String()
			case "expired":
				p.Expires = time.Unix(1, 0)
				if err = m.savePending(c, p); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				if err = m.Cancel(context.Background(), "saved", id); err != nil {
					t.Fatal(err)
				}
			case "superseded":
				begin()
			case "logout":
				if err = m.Logout(context.Background(), "saved"); err != nil {
					t.Fatal(err)
				}
			case "recreated":
				if err = m.Logout(context.Background(), "saved"); err != nil {
					t.Fatal(err)
				}
				err = m.Store.Update(context.Background(), func(f *config.File) error {
					n := f.Connections["saved"]
					n.ID = "recreated"
					f.Connections["saved"] = n
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = m.Complete(context.Background(), "saved", id, callback); err == nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("stale callback accepted or provider data leaked")
			}
			if _, err = m.load(context.Background(), p.Backend, p.SecretRef); !errors.Is(err, ErrCredentialNotFound) {
				t.Fatal("staged secret survived invalidation")
			}
			if calls.Load() != 1 {
				t.Fatal("invalidated callback exchanged")
			}
		})
	}
}

func TestMigrationAndPartialCleanup(t *testing.T) {
	ctx := context.Background()
	m, _ := managerFixture(t, "https://example.test", "bearer")
	if err := m.Import(ctx, "saved", "token"); err != nil {
		t.Fatal(err)
	}
	original := currentFixture(t, m)
	if err := m.Migrate(ctx, "saved", "keyring"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.load(ctx, "file", original.CredentialRef); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatal("file credential not removed")
	}
	keyring := m.Keyring.(*memoryKeyring)
	keyring.failDelete.Store(true)
	err := m.Migrate(ctx, "saved", "file")
	var cleanup *CleanupError
	if !errors.As(err, &cleanup) {
		t.Fatal("missing partial cleanup error")
	}
	c := currentFixture(t, m)
	if c.Backend != "file" || c.CleanupRef == "" {
		t.Fatal("lost committed or cleanup reference")
	}
	credential, _, err := m.Resolve(ctx, resolvedFixture(t, m), true)
	if err != nil || credential.Token != "Bearer token" {
		t.Fatal("committed bundle inaccessible")
	}
	if err = m.Logout(ctx, "saved"); err == nil {
		t.Fatal("keyring cleanup failure hidden")
	}
	keyring.failDelete.Store(false)
	if err = m.Logout(ctx, "saved"); err != nil {
		t.Fatal(err)
	}
	if len(keyring.values) != 0 {
		t.Fatal("logout left keyring secret")
	}
	if err = m.Import(ctx, "saved", "token"); err != nil {
		t.Fatal(err)
	}
	keyring.failSet.Store(true)
	if err = m.Migrate(ctx, "saved", "keyring"); err == nil {
		t.Fatal("backend failure silently fell back")
	}
	c = currentFixture(t, m)
	if c.Backend != "file" || c.StagedRef == "" {
		t.Fatal("failed migration lost source or repair intent")
	}
	if err = m.Logout(ctx, "saved"); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidGrantAndPersistenceFailureNeverReplay(t *testing.T) {
	for _, failure := range []string{"invalid_grant", "persistence"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			var keyring *memoryKeyring
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if n == 1 {
					_, _ = fmt.Fprint(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`)
					return
				}
				if failure == "invalid_grant" {
					w.WriteHeader(400)
					_, _ = fmt.Fprint(w, `{"error":"invalid_grant","error_description":"PRIVATE"}`)
				} else {
					keyring.failSet.Store(true)
					_, _ = fmt.Fprint(w, oauthGoodResponse)
				}
			}))
			defer server.Close()
			m, _ := managerFixture(t, server.URL, "oauth-user")
			keyring = m.Keyring.(*memoryKeyring)
			loginFixture(t, m, "oauth-user")
			if err := m.Migrate(context.Background(), "saved", "keyring"); err != nil {
				t.Fatal(err)
			}
			expireFixture(t, m)
			_, _, err := m.Resolve(context.Background(), resolvedFixture(t, m), true)
			var partial *PartialError
			if failure == "invalid_grant" && !errors.Is(err, ErrInvalidGrant) || failure == "persistence" && !errors.As(err, &partial) {
				t.Fatalf("wrong error %v", err)
			}
			keyring.failSet.Store(false)
			if _, _, err = m.Resolve(context.Background(), resolvedFixture(t, m), true); err == nil || calls.Load() != 2 {
				t.Fatal("consumed refresh replayed")
			}
		})
	}
}

func TestOverridesBypassUnusableStoreAndBinding(t *testing.T) {
	t.Setenv("PLANE_CLI_CONFIG_DIR", filepath.Join(t.TempDir(), "absent"))
	r := config.Resolved{Name: "saved", Connection: config.Connection{AuthMode: "oauth-user", Backend: "keyring"}}
	c, s, err := ResolveStored(context.Background(), r, func(key string) string {
		if key == "PLANE_CLI_ACCESS_TOKEN" {
			return "override"
		}
		return ""
	}, true)
	if err != nil || c.Token != "Bearer override" || !s.ExternallyManaged {
		t.Fatal("override read persistent credentials")
	}
	if _, err = os.Stat(os.Getenv("PLANE_CLI_CONFIG_DIR")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("override touched store")
	}
	m, _ := managerFixture(t, "https://example.test", "api-key")
	if err = m.Import(context.Background(), "saved", "token"); err != nil {
		t.Fatal(err)
	}
	changed := resolvedFixture(t, m)
	changed.BaseURL = "https://other.test"
	if _, _, err = m.Resolve(context.Background(), changed, true); err == nil {
		t.Fatal("stored credential crossed base binding")
	}
}

func TestAuthProcessHelper(t *testing.T) {
	action := os.Getenv("PLANE_AUTH_TEST_ACTION")
	if action == "" {
		return
	}
	m, err := OpenManager()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	switch action {
	case "resolve":
		f, e := m.Store.Load()
		if e != nil {
			t.Fatal(e)
		}
		c, s, e := m.Resolve(context.Background(), config.Resolved{Name: "saved", Connection: f.Connections["saved"]}, true)
		if e != nil || c.Token != "Bearer renewed" || s.Status != "stored" {
			t.Fatalf("process resolve %v %s", e, s.Status)
		}
	case "logout":
		if err = m.Logout(context.Background(), "saved"); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCrossProcessRefreshAndLogout(t *testing.T) {
	for _, last := range []string{"resolve", "logout"} {
		t.Run(last, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if n == 1 {
					_, _ = fmt.Fprint(w, `{"access_token":"initial","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`)
					return
				}
				if n == 2 {
					close(started)
				}
				<-release
				_, _ = fmt.Fprint(w, `{"access_token":"renewed","refresh_token":"rotated","expires_in":3600,"token_type":"Bearer"}`)
			}))
			defer func() { once.Do(func() { close(release) }); server.Close() }()
			m, dir := managerFixture(t, server.URL, "oauth-user")
			loginFixture(t, m, "oauth-user")
			expireFixture(t, m)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			process := func(action string) *exec.Cmd {
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAuthProcessHelper$")
				cmd.Env = append(os.Environ(), "PLANE_AUTH_TEST_ACTION="+action, "PLANE_CLI_CONFIG_DIR="+dir, "PLANE_CLI_OAUTH_CLIENT_SECRET=")
				return cmd
			}
			first := process("resolve")
			var firstOutput strings.Builder
			first.Stdout = &firstOutput
			first.Stderr = &firstOutput
			if err := first.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("refresh did not start")
			}
			second := process(last)
			var secondOutput strings.Builder
			second.Stdout = &secondOutput
			second.Stderr = &secondOutput
			if err := second.Start(); err != nil {
				t.Fatal(err)
			}
			once.Do(func() { close(release) })
			if err := first.Wait(); err != nil {
				t.Fatalf("first: %v %s", err, &firstOutput)
			}
			if err := second.Wait(); err != nil {
				t.Fatalf("second: %v %s", err, &secondOutput)
			}
			if calls.Load() != 2 {
				t.Fatal("refresh token used twice")
			}
			if last == "logout" {
				c := currentFixture(t, m)
				if c.CredentialRef != "" {
					t.Fatal("refresh survived logout")
				}
			}
		})
	}
}

func TestPendingFilesContainNoAppSecret(t *testing.T) {
	m, dir := managerFixture(t, "https://example.test", "oauth-user")
	_, _, err := m.Begin(context.Background(), "saved", LoginOptions{ClientID: "client", ClientSecret: "app-secret-canary", RedirectURI: "https://callback.example/callback"})
	if err != nil {
		t.Fatal(err)
	}
	c := currentFixture(t, m)
	data, err := os.ReadFile(filepath.Join(dir, pendingName(c)))
	if err != nil || strings.Contains(string(data), "app-secret-canary") {
		t.Fatal("pending record contains app secret")
	}
	var pending Pending
	if err = json.Unmarshal(data, &pending); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, pendingName(c)))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe pending permissions")
	}
	if err = m.Cancel(context.Background(), "saved", pending.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.load(context.Background(), pending.Backend, pending.SecretRef); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatal("cancel retained staged secret")
	}
}
