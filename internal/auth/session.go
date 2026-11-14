package auth

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/foae/plane-cli/internal/config"
)

// PartialError means a remote credential may have rotated without a durable
// local replacement. It must not be presented as an ordinary login failure.
type PartialError struct{}

func (*PartialError) Error() string {
	return "credential exchange may have succeeded but local persistence failed; log in again"
}

func ResolveStored(ctx context.Context, r config.Resolved, getenv func(string) string, renew bool) (Credential, Status, error) {
	credential, status, err := Resolve(r, getenv)
	if err != nil || status.ExternallyManaged || r.Backend == "env" || r.Name == "" {
		return credential, status, err
	}
	m, err := OpenManager()
	if err != nil {
		return credential, status, err
	}
	defer func() { _ = m.Close() }()
	_, request := r.Deadlines()
	m.Transport = &TransportOptions{RequestTimeout: request, CAFile: r.CAFile}
	return m.Resolve(ctx, r, renew)
}

func (m *Manager) Resolve(ctx context.Context, r config.Resolved, renew bool) (Credential, Status, error) {
	status := Status{Mode: r.AuthMode, Backend: r.Backend, Status: "missing"}
	unlock, err := m.Store.ConnectionLock(ctx, r.Name)
	if err != nil {
		return Credential{}, status, err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(r.Name)
	if err != nil {
		return Credential{}, status, err
	}
	if c.ID != r.ID || c.Generation != r.Generation || c.BaseURL != r.BaseURL || c.AuthMode != r.AuthMode || c.Backend != r.Backend || c.OAuthClientID != r.OAuthClientID || c.InstallationID != r.InstallationID {
		return Credential{}, status, &LoginError{Message: "stored credential binding differs from effective settings; log in explicitly"}
	}
	b, err := m.load(ctx, c.Backend, c.CredentialRef)
	if errors.Is(err, ErrCredentialNotFound) {
		return Credential{}, status, nil
	}
	if err != nil {
		return Credential{}, status, err
	}
	if !bound(b, c) {
		return Credential{}, status, &LoginError{Message: "credential binding mismatch; log in again"}
	}
	status.Binding = c.ID
	if !b.Expiry.IsZero() {
		status.Expiry = &b.Expiry
	}
	if b.Invalid {
		return Credential{}, status, &LoginError{Message: "credential renewal was interrupted or rejected; log in again"}
	}
	if !b.Expiry.IsZero() && !time.Now().Add(30*time.Second).Before(b.Expiry) {
		status.Status = "expired"
		if !renew {
			return Credential{}, status, nil
		}
		if c.AuthMode != "oauth-user" && c.AuthMode != "oauth-bot" {
			return Credential{}, status, &LoginError{Message: "externally issued token expired; replace it"}
		}
		var e error
		transportConnection := c
		transportConnection.RequestTimeout, transportConnection.CAFile = r.RequestTimeout, r.CAFile
		b, e = m.renew(ctx, transportConnection, b)
		if e != nil {
			return Credential{}, status, e
		}
	}
	credential, _, err := Resolve(config.Resolved{Connection: c}, func(key string) string {
		if (c.AuthMode == "api-key" && key == "PLANE_CLI_API_KEY") || (c.AuthMode != "api-key" && key == "PLANE_CLI_ACCESS_TOKEN") {
			return b.AccessToken
		}
		return ""
	})
	if err != nil {
		return Credential{}, status, err
	}
	if b.AccessToken == "" {
		return Credential{}, status, &LoginError{Message: "stored token is empty; log in again"}
	}
	status.Status = "stored"
	if !b.Expiry.IsZero() {
		status.Expiry = &b.Expiry
	}
	return credential, status, nil
}

func (m *Manager) oauthClient(c config.Connection) (*OAuthClient, error) {
	_, timeout := config.Resolved{Connection: c}.Deadlines()
	caFile := c.CAFile
	if m.Transport != nil {
		timeout, caFile = m.Transport.RequestTimeout, m.Transport.CAFile
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return NewOAuthClient(c.BaseURL, timeout, caFile)
}

func (m *Manager) renew(ctx context.Context, c config.Connection, b Bundle) (Bundle, error) {
	var form url.Values
	basicID, basicSecret := "", ""
	if c.AuthMode == "oauth-user" {
		if b.RefreshToken == "" || b.ClientSecret == "" {
			return b, &LoginError{Message: "refresh credentials missing; log in again"}
		}
		form = url.Values{"grant_type": {"refresh_token"}, "refresh_token": {b.RefreshToken}, "client_id": {b.ClientID}, "client_secret": {b.ClientSecret}}
	} else {
		form = url.Values{"grant_type": {"client_credentials"}, "app_installation_id": {b.InstallationID}, "scope": {c.Scopes}}
		basicID, basicSecret = b.ClientID, b.ClientSecret
	}
	client, err := m.oauthClient(c)
	if err != nil {
		return b, err
	}
	defer client.Close()
	// Only user refresh tokens can be consumed by an uncertain exchange.
	// Bot client_credentials grants are repeatable after transient failures.
	if c.AuthMode == "oauth-user" {
		b.Invalid = true
		if err = m.save(ctx, c.Backend, c.CredentialRef, b); err != nil {
			return b, err
		}
	}
	token, err := client.Exchange(ctx, form, basicID, basicSecret)
	if err != nil {
		return b, err
	}
	b.AccessToken = token.AccessToken
	if token.RefreshToken != "" {
		b.RefreshToken = token.RefreshToken
	}
	b.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	b.Invalid = false
	if err = m.save(ctx, c.Backend, c.CredentialRef, b); err != nil {
		return b, &PartialError{}
	}
	return b, nil
}

func (m *Manager) Import(ctx context.Context, name, token string) error {
	unlock, err := m.Store.ConnectionLock(ctx, name)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(name)
	if err != nil {
		return err
	}
	if c.AuthMode != "api-key" && c.AuthMode != "bearer" {
		return &InputError{Message: "token import requires api-key or bearer mode"}
	}
	if token == "" {
		return &InputError{Message: "token input is empty"}
	}
	if _, _, err = Resolve(config.Resolved{Connection: c}, func(key string) string {
		if c.AuthMode == "api-key" && key == "PLANE_CLI_API_KEY" || c.AuthMode == "bearer" && key == "PLANE_CLI_ACCESS_TOKEN" {
			return token
		}
		return ""
	}); err != nil {
		return err
	}
	if err = m.clearPending(ctx, c); err != nil {
		return err
	}
	b := newBundle(c)
	b.AccessToken = token
	return m.commitBundle(ctx, name, c, b, c.Backend)
}

// CleanupLocked is shared with connection delete/rebind. Caller holds the
// connection-name lock and invokes it before acquiring the configuration lock.
func (m *Manager) CleanupLocked(ctx context.Context, c config.Connection) error {
	if err := m.clearPending(ctx, c); err != nil {
		return &CleanupError{Phase: "Pending login cleanup failed; connection retained for repair"}
	}
	if err := m.remove(ctx, c.StagedBackend, c.StagedRef); err != nil {
		return &CleanupError{Phase: "Pending login cleaned; staged credential cleanup failed; connection retained for repair"}
	}
	if err := m.remove(ctx, c.Backend, c.CredentialRef); err != nil {
		return &CleanupError{Phase: "Pending and staged credentials cleaned; current credential cleanup failed; connection retained for repair"}
	}
	if err := m.remove(ctx, c.CleanupBackend, c.CleanupRef); err != nil {
		return &CleanupError{Phase: "Current credential cleaned; previous backend cleanup failed; connection retained for repair"}
	}
	return nil
}
func (m *Manager) Logout(ctx context.Context, name string) error {
	unlock, err := m.Store.ConnectionLock(ctx, name)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(name)
	if err != nil {
		return err
	}
	// Invalidate in-flight consent even if a later backend cleanup fails.
	err = m.Store.Update(ctx, func(f *config.File) error {
		next := f.Connections[name]
		next.Generation++
		f.Connections[name] = next
		return nil
	})
	if err != nil {
		return err
	}
	if err = m.CleanupLocked(ctx, c); err != nil {
		return err
	}
	return m.Store.Update(ctx, func(f *config.File) error {
		next := f.Connections[name]
		next.CredentialRef = ""
		next.CleanupRef = ""
		next.CleanupBackend = ""
		next.StagedRef = ""
		next.StagedBackend = ""
		f.Connections[name] = next
		return nil
	})
}
func (m *Manager) Migrate(ctx context.Context, name, backend string) error {
	if backend != "file" && backend != "keyring" {
		return &InputError{Message: "migration destination must be file or keyring"}
	}
	unlock, err := m.Store.ConnectionLock(ctx, name)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(name)
	if err != nil {
		return err
	}
	if c.Backend == backend {
		return &InputError{Message: "connection already uses this backend"}
	}
	if err = m.clearPending(ctx, c); err != nil {
		return err
	}
	b, err := m.load(ctx, c.Backend, c.CredentialRef)
	if err != nil {
		return err
	}
	if !bound(b, c) {
		return &LoginError{Message: "credential binding mismatch"}
	}
	return m.commitBundle(ctx, name, c, b, backend)
}
