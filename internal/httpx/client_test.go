package httpx

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/config"
)

func clientFor(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New(base, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.http.CloseIdleConnections)
	return c
}

func transportError(t *testing.T, err error, exit int) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Exit != exit {
		t.Fatalf("error = %#v, want exit %d", err, exit)
	}
	return e
}

func TestPrefixEscapingAndNativeJSON(t *testing.T) {
	const raw = ` {"id":9007199254740993,"unknown":false} `
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/plane/api/v2/a%20b%3F%25/?filter=a%26b" {
			t.Errorf("URI = %q", r.RequestURI)
		}
		if r.Header.Get("X-Api-Key") != "secret" || r.Header.Get("Authorization") != "" {
			t.Error("wrong authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(raw))
	}))
	defer s.Close()
	c := clientFor(t, s.URL+"/plane")
	r, err := c.Do(context.Background(), "GET", []string{"api", "v2", "a b?%"}, url.Values{"filter": {"a&b"}}, nil, "X-Api-Key", "secret")
	if err != nil || string(r.Data) != raw || r.Status != 200 {
		t.Fatalf("response = %#v, %v", r, err)
	}
}

func TestRequestURL(t *testing.T) {
	for _, tc := range []struct {
		name, base string
		segments   []string
		query      url.Values
		want       string
	}{
		{"root", "https://example.com", nil, nil, "https://example.com/"},
		{"prefix", "https://example.com/plane///", []string{"api", "v2"}, nil, "https://example.com/plane/api/v2/"},
		{"escaped-prefix", "https://example.com/a%2Fb/", []string{"a b?%"}, url.Values{"filter": {"a&b"}}, "https://example.com/a%2Fb/a%20b%3F%25/?filter=a%26b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RequestURL(tc.base, tc.segments, tc.query)
			if err != nil || got != tc.want {
				t.Fatalf("URL = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, base := range []string{"", "ftp://example.com", "https:///missing", "https://user:secret@example.com", "https://example.com/?secret=value", "https://example.com/#secret", "https://%zz"} {
		_, err := RequestURL(base, nil, nil)
		e := transportError(t, err, 1)
		if e.Code != "invalid_transport_config" || strings.Contains(e.Error(), "secret") {
			t.Fatalf("unsafe base validation: %#v", e)
		}
	}
	for _, segment := range []string{"", ".", "..", "a/b", `a\b`} {
		_, err := RequestURL("https://example.com", []string{segment}, nil)
		e := transportError(t, err, 2)
		if e.Code != "invalid_path" {
			t.Fatalf("invalid segment error = %#v", e)
		}
	}
}

func TestRejectAmbiguousSegmentsBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer s.Close()
	c := clientFor(t, s.URL)
	for _, segment := range []string{"", ".", "..", "a/b", `a\b`} {
		_, err := c.Do(context.Background(), "GET", []string{segment}, nil, nil, "", "")
		_ = transportError(t, err, 2)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid path sent a request")
	}
}

func TestCredentialHeaderBytesBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()
	c := clientFor(t, s.URL)
	for _, mode := range []string{"api-key", "bearer"} {
		var resolved config.Resolved
		resolved.AuthMode = mode
		header, variable := "X-Api-Key", "PLANE_CLI_API_KEY"
		if mode == "bearer" {
			header, variable = "Authorization", "PLANE_CLI_ACCESS_TOKEN"
		}
		for b := range 256 {
			if (b >= 0x20 || b == '\t') && b != 0x7f {
				continue
			}
			token := "canary-secret" + string(byte(b))
			credential, _, err := auth.Resolve(resolved, func(name string) string {
				if name == variable {
					return token
				}
				return ""
			})
			if err == nil || credential.Token != "" || strings.Contains(err.Error(), "canary") {
				t.Fatalf("%s byte %02x: unsafe credential resolution: %#v, %v", mode, b, credential, err)
			}
			_, err = c.Do(context.Background(), http.MethodPost, []string{"api"}, nil, json.RawMessage(`{}`), header, token)
			e := transportError(t, err, 2)
			if e.Code != "invalid_auth" || e.OutcomeUnknown || strings.Contains(e.Error(), "canary") {
				t.Fatalf("%s byte %02x: unsafe local error: %#v", mode, b, e)
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid credentials sent %d requests", calls.Load())
	}
	for _, token := range []string{"a\tb", "a\x80\xffb"} {
		var resolved config.Resolved
		resolved.AuthMode = "api-key"
		credential, _, err := auth.Resolve(resolved, func(name string) string {
			if name == "PLANE_CLI_API_KEY" {
				return token
			}
			return ""
		})
		if err != nil || credential.Token != token {
			t.Fatalf("valid header bytes rejected by resolver: %v", err)
		}
		if _, err := c.Do(context.Background(), http.MethodPost, []string{"api"}, nil, nil, credential.Header, credential.Token); err != nil {
			t.Fatalf("valid header bytes rejected by transport: %v", err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("valid credentials sent %d requests, want 2", calls.Load())
	}
}

func TestRedirectDoesNotForwardCredentials(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { destinationCalls.Add(1) }))
	defer destination.Close()
	var sourceCalls atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		if r.Header.Get("Authorization") != "Bearer canary" || r.Header.Get("X-Api-Key") != "" {
			t.Error("wrong source auth")
		}
		http.Redirect(w, r, destination.URL+"/secret", http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	_, err := clientFor(t, source.URL).Do(context.Background(), "POST", []string{"api"}, nil, json.RawMessage(`{}`), "Authorization", "Bearer canary")
	e := transportError(t, err, 5)
	if e.Status != 307 || sourceCalls.Load() != 1 || destinationCalls.Load() != 0 {
		t.Fatalf("redirect followed or retried: %#v %d %d", e, sourceCalls.Load(), destinationCalls.Load())
	}
}

func TestCustomCA(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`null`))
	}))
	defer s.Close()
	_, err := clientFor(t, s.URL).Do(context.Background(), "GET", []string{"api"}, nil, nil, "", "")
	_ = transportError(t, err, 4)
	file := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := New(s.URL, 0, file)
	if err != nil {
		t.Fatal(err)
	}
	defer c.http.CloseIdleConnections()
	if _, err := c.Do(context.Background(), "GET", []string{"api"}, nil, nil, "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestResponseValidationAndSafeFailures(t *testing.T) {
	cases := []struct {
		name, contentType, body string
		status, exit            int
		unknown                 bool
	}{
		{"duplicate", "application/json", `{"canary":1,"canary":2}`, 200, 5, true},
		{"trailing", "application/json", `{} {}`, 200, 5, true},
		{"html", "text/html", `<html>canary-secret</html>`, 200, 5, true},
		{"utf8", "application/json", "\"\xff\"", 200, 5, true},
		{"malformed", "application/json", `{"canary-secret":`, 200, 5, true},
		{"oversize", "application/json", `"` + strings.Repeat("x", maxResponseBytes) + `"`, 200, 5, true},
		{"auth", "text/html", "canary-secret", 401, 3, false},
		{"forbidden", "application/json", `{"secret":"canary-secret"}`, 403, 3, false},
		{"rate-limit", "text/html", "canary-secret", 429, 5, false},
		{"bad-gateway", "text/html", "canary-secret", 502, 5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Retry-After", "12")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			_, err := clientFor(t, s.URL).Do(context.Background(), "POST", []string{"api"}, nil, nil, "", "")
			e := transportError(t, err, tc.exit)
			if e.OutcomeUnknown != tc.unknown || strings.Contains(e.Error(), "canary") || calls.Load() != 1 {
				t.Fatalf("unsafe error or retry: %#v, calls=%d", e, calls.Load())
			}
			if tc.status == 429 && (e.RetryAfterSeconds == nil || *e.RetryAfterSeconds != 12) {
				t.Fatal("missing Retry-After")
			}
		})
	}
}

func TestLargeValidResponseAnd204(t *testing.T) {
	large := `"` + strings.Repeat("x", 9<<20) + `"`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/empty/" {
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(large))
	}))
	defer s.Close()
	c := clientFor(t, s.URL)
	r, err := c.Do(context.Background(), "GET", []string{"large"}, nil, nil, "", "")
	if err != nil || string(r.Data) != large {
		t.Fatalf("valid response rejected: %v", err)
	}
	r, err = c.Do(context.Background(), "DELETE", []string{"empty"}, nil, nil, "", "")
	if err != nil || r.Status != 204 || string(r.Data) != "null" {
		t.Fatalf("204 = %#v, %v", r, err)
	}
}

func TestCancellationAndDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "interrupt", true: "deadline"}[deadline], func(t *testing.T) {
			started := make(chan struct{})
			s := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
			defer s.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := clientFor(t, s.URL)
			if deadline {
				c.http.Timeout = 50 * time.Millisecond
			}
			done := make(chan error, 1)
			go func() { _, err := c.Do(ctx, "POST", []string{"api"}, nil, nil, "", ""); done <- err }()
			<-started
			if !deadline {
				cancel()
			}
			exit := 130
			if deadline {
				exit = 4
			}
			e := transportError(t, <-done, exit)
			if !e.OutcomeUnknown {
				t.Fatal("lost mutation certainty")
			}
		})
	}
}

func TestRetryAfterBounds(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, value := range []string{"", "-1", "+1", "86401", "99999999999999999999", "canary-secret", now.Add(-time.Hour).Format(http.TimeFormat)} {
		if got, valid := retryAfter(value, now); valid {
			t.Errorf("accepted %q: %s", value, got)
		}
	}
	for _, value := range []string{"0", "86400", now.Add(time.Minute).Format(http.TimeFormat)} {
		if _, valid := retryAfter(value, now); !valid {
			t.Errorf("rejected %q", value)
		}
	}
}

type retryTestTransport func(*http.Request) (*http.Response, error)

func (f retryTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func retryTestResponse(status int, retryAfter ...string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/problem+json"}, "Retry-After": retryAfter},
		Body:       io.NopCloser(strings.NewReader(`{"type":"rate_limited","code":"rate_limited"}`)),
	}, nil
}

func TestRetryOn429Policy(t *testing.T) {
	for _, tc := range []struct {
		name, operation string
		optIn           bool
		headers         []string
		status, calls   int
	}{
		{name: "disabled", headers: []string{"0"}, status: 429, calls: 1},
		{name: "one-retry-only", optIn: true, headers: []string{"0"}, status: 429, calls: 2},
		{name: "missing-header", optIn: true, status: 429, calls: 1},
		{name: "invalid-header", optIn: true, headers: []string{"invalid"}, status: 429, calls: 1},
		{name: "multiple-headers", optIn: true, headers: []string{"0", "5"}, status: 429, calls: 1},
		{name: "server-error", optIn: true, headers: []string{"0"}, status: 503, calls: 1},
		{name: "mutation-even-with-GET", operation: "mutation", optIn: true, headers: []string{"0"}, status: 429, calls: 1},
		{name: "generic-send", operation: "generic", optIn: true, headers: []string{"0"}, status: 429, calls: 1},
		{name: "download-redirect", operation: "download", optIn: true, headers: []string{"0"}, status: 429, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &Client{base: "https://example.invalid", RetryOn429: tc.optIn}
			client.http = &http.Client{Transport: retryTestTransport(func(_ *http.Request) (*http.Response, error) {
				calls++
				return retryTestResponse(tc.status, tc.headers...)
			})}
			ctx := context.Background()
			segments := []string{"api", "v2", "probe"}
			var err error
			switch tc.operation {
			case "mutation":
				_, err = client.DoMutation(ctx, http.MethodGet, segments, nil, nil, "", "")
			case "generic":
				_, err = client.Do(ctx, http.MethodGet, segments, nil, nil, "", "")
			case "download":
				_, err = client.DownloadLocation(ctx, segments, "", "")
			default:
				_, err = client.DoRead(ctx, http.MethodGet, segments, nil, nil, "", "")
			}
			remote := transportError(t, err, 5)
			if calls != tc.calls || remote.Status != tc.status || remote.OutcomeUnknown {
				t.Fatalf("calls=%d; error=%#v", calls, remote)
			}
		})
	}
}

func TestRetryOn429DeadlineBudget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		budget time.Duration
		wait   time.Duration
		date   bool
	}{
		{name: "delay-seconds", budget: 3 * time.Second, wait: 2 * time.Second},
		{name: "http-date", budget: 1900 * time.Millisecond, wait: 1750 * time.Millisecond, date: true},
		{name: "insufficient-budget", budget: time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				time.Sleep(250 * time.Millisecond)
				started := time.Now()
				header := "2"
				if tc.date {
					header = started.Add(tc.wait).UTC().Format(http.TimeFormat)
				}
				ctx, cancel := context.WithTimeout(t.Context(), tc.budget)
				defer cancel()
				calls := 0
				client := &Client{base: "https://example.invalid", RetryOn429: true}
				client.http = &http.Client{Transport: retryTestTransport(func(req *http.Request) (*http.Response, error) {
					calls++
					if calls == 1 {
						return retryTestResponse(http.StatusTooManyRequests, header)
					}
					if got, ok := req.Context().Deadline(); !ok || !got.Equal(started.Add(tc.budget)) {
						t.Errorf("retry reset the total deadline: %v", got)
					}
					return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
				})}
				response, err := client.DoRead(ctx, http.MethodGet, []string{"api", "v2"}, nil, nil, "", "")
				if tc.wait == 0 {
					remote := transportError(t, err, 5)
					if calls != 1 || remote.Status != 429 || remote.UpstreamCode != "rate_limited" || remote.RetryAfterSeconds == nil || *remote.RetryAfterSeconds != 2 {
						t.Fatalf("calls=%d; original diagnostic=%#v", calls, remote)
					}
				} else if err != nil || calls != 2 || response.Status != http.StatusNoContent {
					t.Fatalf("calls=%d; response=%#v; err=%v", calls, response, err)
				}
				if elapsed := time.Since(started); elapsed != tc.wait {
					t.Fatalf("waited %s; want %s", elapsed, tc.wait)
				}
			})
		})
	}
}

func TestRetryOn429CancellationWhileWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		calls := 0
		client := &Client{base: "https://example.invalid", RetryOn429: true}
		client.http = &http.Client{Transport: retryTestTransport(func(_ *http.Request) (*http.Response, error) {
			calls++
			return retryTestResponse(http.StatusTooManyRequests, "60")
		})}
		started := time.Now()
		result := make(chan error, 1)
		go func() {
			_, err := client.DoRead(ctx, http.MethodGet, []string{"api", "v2"}, nil, nil, "", "")
			result <- err
		}()
		synctest.Wait()
		if calls != 1 {
			t.Fatalf("requests before cancellation = %d; want 1", calls)
		}
		cancel()
		remote := transportError(t, <-result, 130)
		if calls != 1 || remote.Code != "interrupted" || time.Since(started) != 0 {
			t.Fatalf("cancellation did not stop the wait: requests=%d; error=%#v", calls, remote)
		}
	})
}

func TestConnectionLossIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer s.Close()
	c := clientFor(t, s.URL)
	_, err := c.Do(context.Background(), "POST", []string{"api"}, nil, json.RawMessage(`{"action":true}`), "", "")
	e := transportError(t, err, 4)
	if calls.Load() != 1 || !e.OutcomeUnknown {
		t.Fatalf("calls=%d, error=%#v", calls.Load(), e)
	}
}

func TestHTTP2ServerUsesHTTP1SingleMutationAttempt(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.ProtoMajor != 1 || r.TLS == nil || r.TLS.NegotiatedProtocol != "http/1.1" {
			t.Errorf("protocol = %q, TLS = %#v; want negotiated HTTP/1.1", r.Proto, r.TLS)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	s.EnableHTTP2 = true
	s.TLS = &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
	s.StartTLS()
	defer s.Close()
	file := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := New(s.URL, 0, file)
	if err != nil {
		t.Fatal(err)
	}
	defer c.http.CloseIdleConnections()
	_, err = c.Do(context.Background(), http.MethodPost, []string{"api"}, nil, json.RawMessage(`{"action":true}`), "", "")
	e := transportError(t, err, 4)
	if calls.Load() != 1 || !e.OutcomeUnknown {
		t.Fatalf("calls=%d, error=%#v", calls.Load(), e)
	}
}
