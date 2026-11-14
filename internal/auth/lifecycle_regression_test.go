package auth

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/config"
)

// A failed write may have reached the backend. Cleanup must retain its journal
// until deletion is confirmed, without destroying the active credential.
type uncertainKeyring struct {
	*memoryKeyring
	setThenFail   bool
	failDeleteRef string
}

func (k *uncertainKeyring) Set(ctx context.Context, key, value string) error {
	if err := k.memoryKeyring.Set(ctx, key, value); err != nil {
		return err
	}
	if k.setThenFail {
		return errors.New("PRIVATE uncertain backend write")
	}
	return nil
}

func (k *uncertainKeyring) Delete(ctx context.Context, key string) error {
	if key == k.failDeleteRef {
		return errors.New("PRIVATE backend deletion")
	}
	return k.memoryKeyring.Delete(ctx, key)
}

func TestCommittedLoginReportsCleanupPhase(t *testing.T) {
	for _, scenario := range []struct {
		mode                   string
		direct, pendingFailure bool
	}{
		{mode: "oauth-bot", direct: true},
		{mode: "oauth-user"},
		{mode: "oauth-user", pendingFailure: true},
		{mode: "oauth-bot", pendingFailure: true},
	} {
		name := fmt.Sprintf("%s/direct=%t/pending=%t", scenario.mode, scenario.direct, scenario.pendingFailure)
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, oauthGoodResponse)
			}))
			defer server.Close()
			m, _ := managerFixture(t, server.URL, scenario.mode)
			keyring := &uncertainKeyring{memoryKeyring: m.Keyring.(*memoryKeyring)}
			m.Keyring = keyring
			if err := m.Store.Update(ctx, func(f *config.File) error {
				c := f.Connections["saved"]
				c.Backend = "keyring"
				f.Connections["saved"] = c
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			options := LoginOptions{ClientID: "client", ClientSecret: "secret", RedirectURI: "http://127.0.0.1:45678/callback", Scopes: "read write"}
			if scenario.direct {
				options.InstallationID = "installation"
				if _, _, err := m.Begin(ctx, "saved", options); err != nil {
					t.Fatal(err)
				}
			} else if scenario.mode == "oauth-user" {
				loginFixture(t, m, scenario.mode)
			}
			original := currentFixture(t, m)
			var err error
			if scenario.direct {
				keyring.failDeleteRef = original.CredentialRef
				_, _, err = m.Begin(ctx, "saved", options)
			} else {
				id, consent, beginErr := m.Begin(ctx, "saved", options)
				if beginErr != nil {
					t.Fatal(beginErr)
				}
				keyring.failDeleteRef = original.CredentialRef
				if scenario.pendingFailure {
					p, pendingErr := m.pending(currentFixture(t, m))
					if pendingErr != nil {
						t.Fatal(pendingErr)
					}
					keyring.failDeleteRef = p.SecretRef
				}
				err = m.Complete(ctx, "saved", id, callbackFixture(t, consent, scenario.mode))
			}
			var cleanup *CleanupError
			var partial *PartialError
			if !errors.As(err, &cleanup) || errors.As(err, &partial) || errors.Is(err, ErrLoginEnded) {
				t.Fatalf("committed login must report cleanup, not failed exchange: %v", err)
			}
			want := "New credentials saved; previous backend cleanup failed; run auth logout --yes to repair"
			if scenario.pendingFailure {
				want = "New credentials saved; pending login cleanup failed; run auth logout --yes to repair"
			}
			if cleanup.Phase != want || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("wrong safe phase: %v", err)
			}
			committed := currentFixture(t, m)
			if committed.CredentialRef == original.CredentialRef || committed.StagedRef != "" {
				t.Fatal("new credential reference was not committed")
			}
			if !scenario.pendingFailure && committed.CleanupRef != original.CredentialRef {
				t.Fatal("previous credential cleanup journal was lost")
			}
			if _, _, resolveErr := m.Resolve(ctx, resolvedFixture(t, m), false); resolveErr != nil {
				t.Fatalf("committed credentials inaccessible: %v", resolveErr)
			}
			if scenario.pendingFailure {
				p, pendingErr := m.pending(committed)
				if pendingErr != nil || !p.Consumed {
					t.Fatal("pending cleanup failure lost consumed journal")
				}
			} else if _, pendingErr := m.pending(committed); !errors.Is(pendingErr, os.ErrNotExist) {
				t.Fatalf("committed login left pending state: %v", pendingErr)
			}
			keyring.failDeleteRef = ""
			if logoutErr := m.Logout(ctx, "saved"); logoutErr != nil {
				t.Fatal(logoutErr)
			}
		})
	}
}

func TestBotTransientRenewalRemainsRepeatable(t *testing.T) {
	for _, failure := range []string{"provider", "transport"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 2 {
					if failure == "transport" {
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = connection.Close()
						return
					}
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = fmt.Fprint(w, oauthGoodResponse)
			}))
			defer server.Close()
			m, _ := managerFixture(t, server.URL, "oauth-bot")
			loginFixture(t, m, "oauth-bot")
			expireFixture(t, m)
			if _, _, err := m.Resolve(context.Background(), resolvedFixture(t, m), true); err == nil {
				t.Fatal("transient failure hidden")
			}
			c := currentFixture(t, m)
			b, err := m.load(context.Background(), c.Backend, c.CredentialRef)
			if err != nil || b.Invalid {
				t.Fatalf("repeatable bot grant invalidated: %v", err)
			}
			credential, _, err := m.Resolve(context.Background(), resolvedFixture(t, m), true)
			if err != nil || credential.Token == "" || calls.Load() != 3 {
				t.Fatalf("bot grant did not recover: %v, calls=%d", err, calls.Load())
			}
		})
	}
}

func TestFailedStagedWriteRecoveryPreservesActiveCredential(t *testing.T) {
	for _, retry := range []string{"login", "migration"} {
		t.Run(retry, func(t *testing.T) {
			ctx := context.Background()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, oauthGoodResponse)
			}))
			defer server.Close()
			m, _ := managerFixture(t, server.URL, "oauth-user")
			loginFixture(t, m, "oauth-user")
			original := currentFixture(t, m)
			keyring := &uncertainKeyring{memoryKeyring: m.Keyring.(*memoryKeyring), setThenFail: true}
			m.Keyring = keyring
			if err := m.Migrate(ctx, "saved", "keyring"); err == nil {
				t.Fatal("uncertain write was reported successful")
			}
			staged := currentFixture(t, m)
			if staged.CredentialRef != original.CredentialRef || staged.Backend != original.Backend || staged.StagedRef == "" || keyring.values[staged.StagedRef] == "" {
				t.Fatal("uncertain destination or known-good active credential was lost")
			}
			keyring.setThenFail = false
			keyring.failDeleteRef = staged.StagedRef
			attempt := func() error {
				if retry == "migration" {
					return m.Migrate(ctx, "saved", "keyring")
				}
				_, _, err := m.Begin(ctx, "saved", LoginOptions{})
				return err
			}
			var cleanup *CleanupError
			if err := attempt(); !errors.As(err, &cleanup) || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("missing safe staged cleanup guidance: %v", err)
			}
			retained := currentFixture(t, m)
			if retained.StagedRef != staged.StagedRef || retained.CredentialRef != original.CredentialRef {
				t.Fatal("failed cleanup discarded repair journal or active credential")
			}
			if credential, _, err := m.Resolve(ctx, resolvedFixture(t, m), false); err != nil || credential.Token == "" {
				t.Fatalf("failed cleanup destroyed known-good credential: %v", err)
			}
			keyring.failDeleteRef = ""
			if err := attempt(); err != nil {
				t.Fatal(err)
			}
			recovered := currentFixture(t, m)
			if recovered.StagedRef != "" || keyring.values[staged.StagedRef] != "" {
				t.Fatal("confirmed cleanup left stale destination or journal")
			}
			if retry == "login" {
				if recovered.CredentialRef != original.CredentialRef || recovered.Backend != original.Backend {
					t.Fatal("starting replacement consent deleted active credentials")
				}
				p, err := m.pending(recovered)
				if err != nil {
					t.Fatal(err)
				}
				if err = m.Cancel(ctx, "saved", p.ID); err != nil {
					t.Fatal(err)
				}
			}
			if credential, _, err := m.Resolve(ctx, resolvedFixture(t, m), false); err != nil || credential.Token == "" {
				t.Fatalf("recovery lost usable credential: %v", err)
			}
		})
	}
}

func TestOAuthUsesInvocationTransportOverrides(t *testing.T) {
	for _, mode := range []string{"oauth-user", "oauth-bot"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, `{"access_token":"access-value","refresh_token":"refresh-value","token_type":"Bearer","expires_in":3600}`)
			}))
			defer server.Close()
			m, dir := managerFixture(t, server.URL, mode)
			caFile := filepath.Join(dir, "invocation-ca.pem")
			if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			if err := m.Store.Update(context.Background(), func(f *config.File) error {
				c := f.Connections["saved"]
				c.CAFile, c.RequestTimeout = filepath.Join(dir, "missing-saved-ca.pem"), "1ns"
				f.Connections["saved"] = c
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			m.Transport = &TransportOptions{RequestTimeout: 2 * time.Second, CAFile: caFile}
			client, err := m.oauthClient(currentFixture(t, m))
			if err != nil {
				t.Fatal(err)
			}
			if client.client.Timeout != 2*time.Second {
				t.Fatal("OAuth ignored invocation request timeout")
			}
			client.Close()
			if mode == "oauth-bot" {
				if _, _, err := m.Begin(context.Background(), "saved", LoginOptions{ClientID: "client", ClientSecret: "secret", InstallationID: "installation"}); err != nil {
					t.Fatalf("direct bot exchange ignored transport override: %v", err)
				}
			} else {
				loginFixture(t, m, mode)
			}
			// Ordinary resolution supplies its effective settings without mutating
			// saved connection settings or needing a Manager transport override.
			m.Transport = nil
			expireFixture(t, m)
			r := resolvedFixture(t, m)
			r.CAFile, r.RequestTimeout = caFile, "2s"
			if _, _, err := m.Resolve(context.Background(), r, true); err != nil {
				t.Fatalf("renewal ignored invocation transport override: %v", err)
			}
			if c := currentFixture(t, m); c.CAFile == caFile || c.RequestTimeout != "1ns" {
				t.Fatal("invocation transport was persisted")
			}
			// Explicit empty CA overrides must not fall back to the saved file.
			m.Transport = &TransportOptions{RequestTimeout: time.Second}
			client, err = m.oauthClient(currentFixture(t, m))
			if err != nil {
				t.Fatalf("empty invocation CA did not clear saved CA: %v", err)
			}
			client.Close()
		})
	}
}

func TestSafeInputAndLoginGuidance(t *testing.T) {
	ctx := context.Background()
	m, _ := managerFixture(t, "https://example.test", "oauth-user")
	for _, err := range []error{
		m.Import(ctx, "saved", "private-token"),
		m.Migrate(ctx, "saved", "unsupported"),
		func() error {
			_, _, err := m.Begin(ctx, "saved", LoginOptions{})
			return err
		}(),
	} {
		var input *InputError
		if !errors.As(err, &input) || strings.Contains(err.Error(), "private-token") {
			t.Fatalf("expected safe invocation guidance: %v", err)
		}
	}
	id, consent, err := m.Begin(ctx, "saved", LoginOptions{ClientID: "client", ClientSecret: "private-secret", RedirectURI: "https://operator.example/callback"})
	if err != nil {
		t.Fatal(err)
	}
	callback := callbackFixture(t, consent, "oauth-user")
	for _, scenario := range []struct {
		id, callback, want string
	}{
		{id: "wrong-private-id", callback: callback, want: "login ID does not match pending attempt"},
		{id: id, callback: "https://operator.example/callback?state=wrong-private-state&code=private-code", want: "callback state does not match"},
	} {
		err = m.Complete(ctx, "saved", scenario.id, scenario.callback)
		var login *LoginError
		if !errors.As(err, &login) || login.Message != scenario.want || strings.Contains(err.Error(), "private-") {
			t.Fatalf("expected safe lifecycle guidance: %v", err)
		}
		p, pendingErr := m.pending(currentFixture(t, m))
		if pendingErr != nil || p.ID != id || p.Consumed {
			t.Fatal("invalid callback consumed pending login")
		}
	}
	if err := m.Cancel(ctx, "saved", id); err != nil {
		t.Fatal(err)
	}
}
