package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/foae/plane-cli/internal/config"
)

// ErrInvalidGrant requires a fresh login, not a retry of the same grant.
var ErrInvalidGrant = errors.New("OAuth grant is invalid or revoked; log in again")

// OAuthToken is the validated result of a single token exchange.
type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	Scope        string
	ExpiresIn    int64
}

// OAuthClient deliberately shares no API transport, cookies or retry policy.
type OAuthClient struct {
	client *http.Client
	base   string
}

func NewOAuthClient(base string, timeout time.Duration, caFile string) (*OAuthClient, error) {
	canonical, err := config.CanonicalBase(base)
	if err != nil {
		return nil, errors.New("invalid OAuth base URL")
	}
	if timeout <= 0 {
		return nil, errors.New("OAuth timeout must be positive")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, errors.New("cannot read OAuth CA certificate")
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			return nil, errors.New("cannot load OAuth system trust roots")
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("invalid OAuth CA certificate")
		}
		tlsConfig.RootCAs = roots
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: timeout, KeepAlive: -1}).DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
		TLSNextProto:          make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}
	return &OAuthClient{
		base: canonical,
		client: &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}, nil
}

func (c *OAuthClient) Close() { c.client.CloseIdleConnections() }

// Exchange sends exactly one request. All failures suppress provider and transport
// details because either can contain credentials or a consumed authorization code.
func (c *OAuthClient) Exchange(ctx context.Context, form url.Values, basicID, basicSecret string) (OAuthToken, error) {
	var zero OAuthToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/auth/o/token/", strings.NewReader(form.Encode()))
	if err != nil {
		return zero, errors.New("cannot construct OAuth exchange")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basicID != "" {
		req.SetBasicAuth(basicID, basicSecret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return zero, errors.New("OAuth exchange failed; outcome uncertain, start a fresh login")
	}
	defer func() { _ = resp.Body.Close() }()
	const maxResponse = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(body) > maxResponse {
		return zero, errors.New("invalid OAuth response")
	}
	var wire struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return zero, errors.New("invalid OAuth response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || wire.Error != "" {
		if wire.Error == "invalid_grant" && resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return zero, ErrInvalidGrant
		}
		return zero, errors.New("OAuth exchange rejected; start a fresh login")
	}
	if !strings.EqualFold(wire.TokenType, "Bearer") || wire.AccessToken == "" || wire.ExpiresIn <= 0 || wire.ExpiresIn > int64((1<<63-1)/time.Second) {
		return zero, errors.New("invalid OAuth response")
	}
	for i := range len(wire.AccessToken) {
		if wire.AccessToken[i] <= 0x20 || wire.AccessToken[i] >= 0x7f {
			return zero, errors.New("invalid OAuth response")
		}
	}
	return OAuthToken{AccessToken: wire.AccessToken, RefreshToken: wire.RefreshToken, Scope: wire.Scope, ExpiresIn: wire.ExpiresIn}, nil
}
