// Package config owns nonsecret connections and their local persistence.
package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const Version = 1

type Connection struct {
	ID             string `json:"id"`
	Generation     uint64 `json:"generation"`
	BaseURL        string `json:"base_url"`
	APIVersion     string `json:"api_version"`
	Workspace      string `json:"workspace,omitempty"`
	Project        string `json:"project,omitempty"`
	AuthMode       string `json:"auth_mode"`
	Backend        string `json:"credential_backend"`
	Timeout        string `json:"timeout,omitempty"`
	RequestTimeout string `json:"request_timeout,omitempty"`
	CAFile         string `json:"ca_file,omitempty"`
	OAuthClientID  string `json:"oauth_client_id,omitempty"`
	RedirectURI    string `json:"redirect_uri,omitempty"`
	Scopes         string `json:"scopes,omitempty"`
	InstallationID string `json:"installation_id,omitempty"`
	CredentialRef  string `json:"credential_ref,omitempty"`
	CleanupBackend string `json:"cleanup_backend,omitempty"`
	CleanupRef     string `json:"cleanup_ref,omitempty"`
	StagedBackend  string `json:"staged_backend,omitempty"`
	StagedRef      string `json:"staged_ref,omitempty"`
}

type File struct {
	Version     int                   `json:"version"`
	Default     string                `json:"default,omitempty"`
	Connections map[string]Connection `json:"connections"`
}

// CanonicalBase preserves an opaque proxy prefix, rejecting paths that could
// change meaning when routes are appended or a proxy decodes them again.
func CanonicalBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") {
		return "", errors.New("base URL must be an HTTP(S) origin with an optional proxy prefix, without credentials, query or fragment")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.New("base URL requires an explicit HTTP or HTTPS scheme and host")
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", errors.New("invalid base URL port")
		}
	}
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		decoded, e := url.PathUnescape(part)
		if e != nil || decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "/\\%\x00\r\n\t") || (part == "" && path != "") {
			return "", errors.New("base URL contains an ambiguous path")
		}
	}
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, "/api/v1") || strings.HasSuffix(lower, "/api/v2") {
		return "", errors.New("base URL must omit the /api/v1 or /api/v2 suffix")
	}
	decoded, _ := url.PathUnescape(path)
	u.Path = decoded
	u.RawPath = path
	return u.String(), nil
}

func Validate(c Connection) (Connection, error) {
	base, err := CanonicalBase(c.BaseURL)
	if err != nil {
		return c, err
	}
	c.BaseURL = base
	if c.APIVersion != "v1" && c.APIVersion != "v2" {
		return c, errors.New("select api-version v1 or v2")
	}
	switch c.AuthMode {
	case "api-key", "bearer", "external", "oauth-user", "oauth-bot":
	default:
		return c, errors.New("select a supported auth mode")
	}
	switch c.Backend {
	case "file", "keyring", "env":
	default:
		return c, errors.New("select credential backend file, keyring or env")
	}
	for _, s := range []string{c.Timeout, c.RequestTimeout} {
		if s != "" {
			d, e := time.ParseDuration(s)
			if e != nil || d <= 0 {
				return c, errors.New("timeouts must be positive durations")
			}
		}
	}
	return c, nil
}
