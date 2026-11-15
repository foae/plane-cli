// Package httpx implements the ordinary Plane API transport policy.
package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/foae/plane-cli/internal/spec"
)

const maxResponseBytes = 16 << 20
const maxRetryAfterSeconds = 86400

type Client struct {
	base string
	http *http.Client
	// EmptySuccessStatus permits a zero-byte body only for this documented status.
	// Zero leaves JSON validation unchanged.
	EmptySuccessStatus int
}

type Response struct {
	Data   json.RawMessage
	Status int
	Upsert string
	// Location is populated only by an explicitly requested download phase.
	Location string
}

// Error contains only normalized, safe diagnostic fields.
type Error struct {
	Code              string
	Message           string
	Status            int
	Exit              int
	RetryAfterSeconds *int
	OutcomeUnknown    bool
}

func (e *Error) Error() string { return e.Message }

func New(base string, requestTimeout time.Duration, caFile string) (*Client, error) {
	if requestTimeout < 0 {
		return nil, &Error{Code: "invalid_transport_config", Message: "Invalid API transport configuration", Exit: 1}
	}
	if _, err := RequestURL(base, nil, nil); err != nil {
		return nil, err
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = http.ProxyFromEnvironment
	// HTTP/2 may replay requests independently of HTTP/1 connection pooling.
	// Restrict protocols and disable pooling to allow only one wire attempt.
	t.Protocols = new(http.Protocols)
	t.Protocols.SetHTTP1(true)
	t.ForceAttemptHTTP2 = false
	t.DisableKeepAlives = true
	// Clone can retain ALPN initialized by a prior DefaultTransport request.
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{}
	}
	t.TLSClientConfig.NextProtos = []string{"http/1.1"}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, &Error{Code: "invalid_ca", Message: "Cannot read custom CA file", Exit: 1}
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, &Error{Code: "invalid_ca", Message: "Cannot load system trust roots", Exit: 1}
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, &Error{Code: "invalid_ca", Message: "Custom CA file contains no certificates", Exit: 1}
		}
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		t.TLSClientConfig.RootCAs = roots
	}
	return &Client{base: base, http: &http.Client{Transport: t, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// RequestURL constructs the request URL without creating a transport or request.
func RequestURL(base string, segments []string, query url.Values) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", &Error{Code: "invalid_transport_config", Message: "Invalid API transport configuration", Exit: 1}
	}
	escaped := strings.TrimRight(u.EscapedPath(), "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\") {
			return "", &Error{Code: "invalid_path", Message: "Invalid API path segment", Exit: 2}
		}
		escaped += "/" + url.PathEscape(segment)
	}
	escaped += "/"
	u.Path, _ = url.PathUnescape(escaped)
	u.RawPath = escaped
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (c *Client) Do(ctx context.Context, method string, segments []string, query url.Values, body json.RawMessage, headerName, token string) (Response, error) {
	return c.do(ctx, method, segments, query, body, headerName, token, method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions, false)
}

// DoRead sends an operation declared read-only, independently of its HTTP method.
func (c *Client) DoRead(ctx context.Context, method string, segments []string, query url.Values, body json.RawMessage, headerName, token string) (Response, error) {
	return c.do(ctx, method, segments, query, body, headerName, token, false, false)
}

// DoMutation sends an operation declared mutating, independently of its HTTP method.
func (c *Client) DoMutation(ctx context.Context, method string, segments []string, query url.Values, body json.RawMessage, headerName, token string) (Response, error) {
	return c.do(ctx, method, segments, query, body, headerName, token, true, false)
}

func (c *Client) do(ctx context.Context, method string, segments []string, query url.Values, body json.RawMessage, headerName, token string, mutation, download bool) (Response, error) {
	requestURL, err := RequestURL(c.base, segments, query)
	if err != nil {
		return Response{}, err
	}
	if (headerName != "" && headerName != "X-Api-Key" && headerName != "Authorization") || (headerName == "" && token != "") {
		return Response{}, &Error{Code: "invalid_auth", Message: "Invalid API authentication configuration", Exit: 2}
	}
	for i := range len(token) {
		if (token[i] < 0x20 && token[i] != '\t') || token[i] == 0x7f {
			return Response{}, &Error{Code: "invalid_auth", Message: "Invalid API authentication configuration", Exit: 2}
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return Response{}, &Error{Code: "invalid_request", Message: "Invalid API request", Exit: 2}
	}
	req.Header.Set("Accept", "application/json")
	if len(body) != 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if headerName != "" {
		req.Header.Set(headerName, token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, requestError(ctx, err, mutation, 0)
	}
	defer func() { _ = resp.Body.Close() }()
	result := Response{Status: resp.StatusCode}
	if download && resp.StatusCode == http.StatusFound {
		result.Location = resp.Header.Get("Location")
		if _, err := storageURL(result.Location); err != nil {
			return result, &Error{Code: "invalid_response", Message: "Invalid download destination", Status: resp.StatusCode, Exit: 5}
		}
		return result, nil
	}
	if upsert := resp.Header.Get("X-Plane-Upsert"); upsert == "created" || upsert == "updated" {
		result.Upsert = upsert
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		e := &Error{Code: "api_error", Message: "API request rejected", Status: resp.StatusCode, Exit: 5, RetryAfterSeconds: retryAfter(resp.Header.Get("Retry-After"), time.Now()), OutcomeUnknown: mutation && resp.StatusCode >= 500}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			e.Code = "auth_error"
			e.Message = "API authentication or authorization rejected"
			e.Exit = 3
		}
		return result, e
	}
	if resp.StatusCode == http.StatusNoContent {
		result.Data = json.RawMessage("null")
		return result, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return result, requestError(ctx, err, mutation, resp.StatusCode)
	}
	if len(raw) > maxResponseBytes {
		return result, &Error{Code: "response_limit", Message: "API response exceeds 16 MiB; use manual pagination", Status: resp.StatusCode, Exit: 5, OutcomeUnknown: mutation}
	}
	if len(raw) == 0 && resp.StatusCode == c.EmptySuccessStatus {
		result.Data = json.RawMessage("null")
		return result, nil
	}
	media, _, mediaErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaErr != nil || (media != "application/json" && (!strings.HasPrefix(media, "application/") || !strings.HasSuffix(media, "+json"))) || !utf8.Valid(raw) {
		return result, malformed(resp.StatusCode, mutation)
	}
	if _, err := spec.DecodeWithLimit(raw, maxResponseBytes); err != nil {
		return result, malformed(resp.StatusCode, mutation)
	}
	result.Data = raw
	return result, nil
}

func malformed(status int, mutation bool) *Error {
	return &Error{Code: "invalid_response", Message: "API returned an invalid JSON response", Status: status, Exit: 5, OutcomeUnknown: mutation}
}

func requestError(ctx context.Context, err error, mutation bool, status int) *Error {
	e := &Error{Code: "transport_error", Message: "API transport failed", Status: status, Exit: 4, OutcomeUnknown: mutation}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		e.Code = "interrupted"
		e.Message = "API request interrupted"
		e.Exit = 130
	} else if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		e.Code = "deadline_exceeded"
		e.Message = "API request deadline exceeded"
	}
	return e
}

func retryAfter(value string, now time.Time) *int {
	if value == "" {
		return nil
	}
	var seconds int
	if n, err := strconv.ParseUint(value, 10, 32); err == nil {
		if n > maxRetryAfterSeconds {
			return nil
		}
		seconds = int(n)
	} else {
		date, err := http.ParseTime(value)
		if err != nil {
			return nil
		}
		d := date.Sub(now)
		if d < 0 || d > maxRetryAfterSeconds*time.Second {
			return nil
		}
		seconds = int((d + time.Second - 1) / time.Second)
	}
	return &seconds
}
