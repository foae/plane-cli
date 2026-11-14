package auth

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const oauthGoodResponse = `{"access_token":"access-value","token_type":"Bearer","expires_in":3600,"scope":"read write"}`

func TestOAuthExchangeRequestContracts(t *testing.T) {
	for _, grant := range []string{"authorization_code", "refresh_token", "client_credentials"} {
		t.Run(grant, func(t *testing.T) {
			form := url.Values{"grant_type": {grant}, "scope": {"read write"}}
			id, secret := "", ""
			switch grant {
			case "authorization_code":
				form.Set("code", "private-code")
				form.Set("redirect_uri", "http://localhost:1234/callback")
				form.Set("client_id", "client")
				form.Set("client_secret", "private-secret")
			case "refresh_token":
				form.Set("refresh_token", "private-refresh")
				form.Set("client_id", "client")
				form.Set("client_secret", "private-secret")
			default:
				id, secret = "client", "private-secret"
				form.Set("app_installation_id", "installation")
			}
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.RequestURI() != "/prefix/auth/o/token/" || r.ProtoMajor != 1 || !r.Close {
					t.Errorf("unexpected request contract: %s %s HTTP/%d close=%v", r.Method, r.URL.Path, r.ProtoMajor, r.Close)
				}
				if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Error("incorrect content type")
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(r.PostForm, form) {
					t.Error("incorrect form")
				}
				gotID, gotSecret, basic := r.BasicAuth()
				if (id != "") != basic || gotID != id || gotSecret != secret {
					t.Error("incorrect credential placement")
				}
				_, _ = fmt.Fprint(w, oauthGoodResponse)
			}))
			defer srv.Close()
			client, err := NewOAuthClient(srv.URL+"/prefix/", time.Second, "")
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			token, err := client.Exchange(context.Background(), form, id, secret)
			if err != nil {
				t.Fatal(err)
			}
			if token.AccessToken != "access-value" || token.RefreshToken != "" || token.ExpiresIn != 3600 || token.Scope != "read write" || calls.Load() != 1 {
				t.Fatal("unexpected token or retry")
			}
		})
	}
}

func TestOAuthRedirectIsolation(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", target.URL+"/secret")
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, oauthGoodResponse)
			}))
			defer srv.Close()
			c, err := NewOAuthClient(srv.URL, time.Second, "")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if _, err := c.Exchange(context.Background(), url.Values{"code": {"private-code"}}, "client", "private-secret"); err == nil {
				t.Fatal("accepted redirect")
			}
			if calls.Load() != 1 || targetCalls.Load() != 0 {
				t.Fatal("redirect followed or retried")
			}
		})
	}
}

func TestOAuthInvalidResponsesAndSafeErrors(t *testing.T) {
	cases := []struct {
		name, body   string
		status       int
		invalidGrant bool
	}{
		{"malformed", `private-secret`, 200, false},
		{"multiple", oauthGoodResponse + ` {}`, 200, false},
		{"null", `null`, 200, false},
		{"oversized", strings.Repeat("x", (1<<20)+1), 200, false},
		{"wrong_type", `{"access_token":"private-token","token_type":"MAC","expires_in":1}`, 200, false},
		{"empty_token", `{"access_token":"","token_type":"Bearer","expires_in":1}`, 200, false},
		{"header_injection", `{"access_token":"private-token\r\nInjected: yes","token_type":"Bearer","expires_in":1}`, 200, false},
		{"zero_expiry", `{"access_token":"private-token","token_type":"Bearer","expires_in":0}`, 200, false},
		{"negative_expiry", `{"access_token":"private-token","token_type":"Bearer","expires_in":-1}`, 200, false},
		{"duration_overflow", `{"access_token":"private-token","token_type":"Bearer","expires_in":9223372037}`, 200, false},
		{"fraction_expiry", `{"access_token":"private-token","token_type":"Bearer","expires_in":1.5}`, 200, false},
		{"invalid_grant", `{"error":"invalid_grant","error_description":"private-code private-secret private-token"}`, 400, true},
		{"provider_failure", `{"error":"private-secret"}`, 500, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			c, err := NewOAuthClient(srv.URL, time.Second, "")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			token, err := c.Exchange(context.Background(), url.Values{"code": {"private-code"}}, "client", "private-secret")
			if err == nil || token != (OAuthToken{}) {
				t.Fatal("accepted invalid response")
			}
			if errors.Is(err, ErrInvalidGrant) != tc.invalidGrant {
				t.Fatal("incorrect invalid-grant classification")
			}
			for _, secret := range []string{"private-code", "private-secret", "private-token", srv.URL} {
				if strings.Contains(err.Error(), secret) {
					t.Fatal("unsafe error")
				}
			}
			if calls.Load() != 1 {
				t.Fatal("exchange retried")
			}
		})
	}
}

func TestOAuthTLSCustomCAAndTransportFailure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = fmt.Fprint(w, oauthGoodResponse) }))
	defer srv.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, trust := range []bool{false, true} {
		path := ""
		if trust {
			path = ca
		}
		c, err := NewOAuthClient(srv.URL, time.Second, path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Exchange(context.Background(), nil, "client", "private-secret")
		c.Close()
		if trust && err != nil {
			t.Fatal(err)
		}
		if !trust && (err == nil || strings.Contains(err.Error(), srv.URL) || strings.Contains(err.Error(), "private-secret")) {
			t.Fatal("unverified TLS accepted or unsafe error")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("unexpected HTTP attempts")
	}
}

func TestOAuthUncertainExchangeSingleAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()
	c, err := NewOAuthClient(srv.URL, time.Second, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, err = c.Exchange(context.Background(), url.Values{"code": {"private-code"}}, "client", "private-secret")
	if err == nil || !strings.Contains(err.Error(), "uncertain") || strings.Contains(err.Error(), srv.URL) || calls.Load() != 1 {
		t.Fatal("unsafe failure or retry")
	}
}

func TestOAuthRotatedRefreshTokenAndExpiryBoundary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"access_token":"access","refresh_token":"rotated","token_type":"bearer","expires_in":9223372036}`)
	}))
	defer srv.Close()
	c, err := NewOAuthClient(srv.URL, time.Second, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	token, err := c.Exchange(context.Background(), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken != "rotated" || token.ExpiresIn != 9223372036 {
		t.Fatal("rotation or expiry lost")
	}
}

func TestOAuthConstructorSafeErrors(t *testing.T) {
	for _, base := range []string{"file:///private-secret", "https://user:private-secret@example.com", "https://example.com/?code=private-code", "https://example.com/#private-token"} {
		if _, err := NewOAuthClient(base, time.Second, ""); err == nil || strings.Contains(err.Error(), "private-") {
			t.Fatal("invalid base accepted or exposed")
		}
	}
	if _, err := NewOAuthClient("https://example.com", 0, ""); err == nil {
		t.Fatal("unbounded timeout accepted")
	}
	if _, err := NewOAuthClient("https://example.com", time.Second, filepath.Join(t.TempDir(), "private-secret")); err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("CA path exposed")
	}
}

func TestOAuthEscapedProxyPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/proxy%20prefix/auth/o/token/" {
			t.Errorf("escaped prefix changed: %s", r.URL.RequestURI())
		}
		_, _ = fmt.Fprint(w, oauthGoodResponse)
	}))
	defer srv.Close()
	c, err := NewOAuthClient(srv.URL+"/proxy%20prefix/", time.Second, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Exchange(context.Background(), nil, "", ""); err != nil {
		t.Fatal(err)
	}
}
