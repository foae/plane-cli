package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/foae/plane-cli/internal/config"
)

type LoginOptions struct {
	ClientID, ClientSecret, RedirectURI, Scopes, InstallationID string
}

// ErrLoginEnded distinguishes a terminal callback from unrelated/malformed traffic.
var ErrLoginEnded = errors.New("login ended; start a fresh authorization attempt")

type Pending struct {
	ID           string    `json:"login_id"`
	ConnectionID string    `json:"connection_id"`
	Generation   uint64    `json:"generation"`
	BaseURL      string    `json:"base_url"`
	ClientID     string    `json:"client_id"`
	RedirectURI  string    `json:"redirect_uri"`
	Scopes       string    `json:"scopes"`
	Mode         string    `json:"mode"`
	Backend      string    `json:"backend"`
	State        string    `json:"state"`
	SecretRef    string    `json:"secret_ref"`
	Expires      time.Time `json:"expires"`
	Consumed     bool      `json:"consumed"`
}

func pendingName(c config.Connection) string { return "pending-" + recordName(c.ID) }
func (m *Manager) pending(c config.Connection) (Pending, error) {
	var p Pending
	data, err := m.Store.Read(pendingName(c))
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(data, &p); err != nil || p.ConnectionID != c.ID || p.ID == "" || p.SecretRef == "" {
		return p, &LoginError{Message: "pending authentication record is invalid; start login again"}
	}
	return p, nil
}
func (m *Manager) savePending(c config.Connection, p Pending) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return m.Store.Write(pendingName(c), data)
}
func (m *Manager) clearPending(ctx context.Context, c config.Connection) error {
	p, err := m.pending(c)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = m.remove(ctx, p.Backend, p.SecretRef); err != nil {
		return &LoginError{Message: "staged app credential cleanup failed; retry login or run auth logout --yes"}
	}
	return m.Store.Remove(pendingName(c))
}
func ValidateRedirect(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || strings.Contains(raw, "#") {
		return nil, &InputError{Message: "redirect must be an exact registered HTTP(S) URI without query, fragment or credentials"}
	}
	return u, nil
}

// Begin stores a single pending attempt without holding a lock during consent.
// A supplied bot installation completes immediately; otherwise consentURL is
// intentionally secret-bearing output for the operator, never diagnostics.
func (m *Manager) Begin(ctx context.Context, name string, o LoginOptions) (loginID, consentURL string, err error) {
	unlock, err := m.Store.ConnectionLock(ctx, name)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(name)
	if err != nil {
		return "", "", err
	}
	if c.Backend == "env" {
		return "", "", &InputError{Message: "login requires persistent backend file or keyring"}
	}
	if c.AuthMode != "oauth-user" && c.AuthMode != "oauth-bot" {
		return "", "", &InputError{Message: "OAuth login requires oauth-user or oauth-bot connection mode"}
	}
	c, err = m.repairStaged(ctx, name, c)
	if err != nil {
		return "", "", err
	}
	if c.CleanupRef != "" {
		return "", "", &CleanupError{Phase: "Credentials saved; previous backend cleanup is pending; run auth logout --yes to repair"}
	}
	if o.ClientID == "" {
		o.ClientID = c.OAuthClientID
	}
	if o.RedirectURI == "" {
		o.RedirectURI = c.RedirectURI
	}
	if o.Scopes == "" {
		o.Scopes = c.Scopes
	}
	if o.InstallationID == "" {
		o.InstallationID = c.InstallationID
	}
	if o.ClientID == "" {
		return "", "", &InputError{Message: "OAuth client ID is required"}
	}
	if o.ClientSecret == "" {
		b, e := m.load(ctx, c.Backend, c.CredentialRef)
		if e != nil {
			return "", "", &LoginError{Message: "supply protected OAuth client secret"}
		}
		if !bound(b, c) || b.ClientID != o.ClientID {
			return "", "", &LoginError{Message: "saved app credentials do not match requested client"}
		}
		o.ClientSecret = b.ClientSecret
	}
	if o.ClientSecret == "" {
		return "", "", &InputError{Message: "OAuth client secret is required"}
	}
	if c.AuthMode != "oauth-bot" || o.InstallationID == "" {
		if _, err = ValidateRedirect(o.RedirectURI); err != nil {
			return "", "", err
		}
	}
	if err = m.clearPending(ctx, c); err != nil {
		return "", "", err
	}
	// A changed app identity is a rebind: remove old credentials before changing
	// the configuration, retaining its reference for repair if cleanup fails.
	if c.OAuthClientID != o.ClientID || c.RedirectURI != o.RedirectURI || c.Scopes != o.Scopes || c.InstallationID != o.InstallationID {
		if err = m.CleanupLocked(ctx, c); err != nil {
			return "", "", err
		}
		c.OAuthClientID = o.ClientID
		c.RedirectURI = o.RedirectURI
		c.Scopes = o.Scopes
		c.InstallationID = o.InstallationID
		c.Generation++
		c.CredentialRef = ""
		c.CleanupRef = ""
		c.CleanupBackend = ""
		if err = m.Store.Update(ctx, func(f *config.File) error { f.Connections[name] = c; return nil }); err != nil {
			return "", "", err
		}
	}
	b := newBundle(c)
	b.ClientSecret = o.ClientSecret
	if c.AuthMode == "oauth-bot" && o.InstallationID != "" {
		b, err = m.exchange(ctx, c, b, "", "")
		if err != nil {
			return "", "", err
		}
		if err = m.commitBundle(ctx, name, c, b, c.Backend); err != nil {
			var cleanup *CleanupError
			if errors.As(err, &cleanup) {
				return "", "", err
			}
			return "", "", &PartialError{}
		}
		return "", "", nil
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return "", "", err
	}
	id, err := config.NewIdentity()
	if err != nil {
		return "", "", err
	}
	ref, err := config.NewIdentity()
	if err != nil {
		return "", "", err
	}
	p := Pending{ID: id, ConnectionID: c.ID, Generation: c.Generation, BaseURL: c.BaseURL, ClientID: c.OAuthClientID, RedirectURI: c.RedirectURI, Scopes: c.Scopes, Mode: c.AuthMode, Backend: c.Backend, State: base64.RawURLEncoding.EncodeToString(random), SecretRef: ref, Expires: time.Now().Add(10 * time.Minute)}
	// Persist the reference before the secret so interruption cannot orphan it.
	if err = m.savePending(c, p); err != nil {
		return "", "", err
	}
	if err = m.save(ctx, c.Backend, ref, b); err != nil {
		return "", "", err
	}
	u, _ := url.Parse(c.BaseURL + "/auth/o/authorize-app/")
	u.RawQuery = url.Values{"response_type": {"code"}, "client_id": {c.OAuthClientID}, "redirect_uri": {c.RedirectURI}, "scope": {c.Scopes}, "state": {p.State}}.Encode()
	return id, u.String(), nil
}

func (m *Manager) exchange(ctx context.Context, c config.Connection, b Bundle, code, installation string) (Bundle, error) {
	var form url.Values
	basicID, basicSecret := "", ""
	if c.AuthMode == "oauth-user" {
		form = url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {b.ClientID}, "client_secret": {b.ClientSecret}, "redirect_uri": {c.RedirectURI}}
	} else {
		if installation != "" {
			b.InstallationID = installation
		}
		form = url.Values{"grant_type": {"client_credentials"}, "app_installation_id": {b.InstallationID}, "scope": {c.Scopes}}
		basicID, basicSecret = b.ClientID, b.ClientSecret
	}
	client, err := m.oauthClient(c)
	if err != nil {
		return b, err
	}
	defer client.Close()
	token, err := client.Exchange(ctx, form, basicID, basicSecret)
	if err != nil {
		return b, err
	}
	b.AccessToken = token.AccessToken
	b.RefreshToken = token.RefreshToken
	b.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return b, nil
}
func (m *Manager) Complete(ctx context.Context, name, id, callback string) (result error) {
	ended := false
	defer func() {
		if ended && result != nil {
			result = errors.Join(ErrLoginEnded, result)
		}
	}()
	unlock, err := m.Store.ConnectionLock(ctx, name)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(name)
	if err != nil {
		return err
	}
	p, err := m.pending(c)
	if err != nil {
		return &LoginError{Message: "no usable pending login; start login again"}
	}
	if p.ID != id {
		return &LoginError{Message: "login ID does not match pending attempt"}
	}
	if p.Consumed || !time.Now().Before(p.Expires) || p.Generation != c.Generation || p.BaseURL != c.BaseURL || p.ClientID != c.OAuthClientID || p.RedirectURI != c.RedirectURI || p.Scopes != c.Scopes || p.Mode != c.AuthMode || p.Backend != c.Backend {
		ended = true
		if err = m.clearPending(ctx, c); err != nil {
			return err
		}
		return &LoginError{Message: "pending login expired, changed or was consumed; start again"}
	}
	u, err := url.Parse(callback)
	if err != nil {
		return &LoginError{Message: "callback URI is invalid"}
	}
	registered, _ := ValidateRedirect(p.RedirectURI)
	if registered == nil || u.Scheme != registered.Scheme || u.Host != registered.Host || u.EscapedPath() != registered.EscapedPath() || u.User != nil || u.Fragment != "" || strings.Contains(callback, "#") || u.Opaque != "" {
		return &LoginError{Message: "callback destination does not match registered redirect"}
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return &LoginError{Message: "callback query is invalid"}
	}
	for _, key := range []string{"state", "code", "app_installation_id", "error"} {
		if len(q[key]) > 1 {
			return &LoginError{Message: "callback contains duplicate authorization parameters"}
		}
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(p.State)) != 1 {
		return &LoginError{Message: "callback state does not match"}
	}
	if q.Get("error") != "" {
		ended = true
		if err = m.clearPending(ctx, c); err != nil {
			return err
		}
		return &LoginError{Message: "authorization denied; start login again"}
	}
	if c.AuthMode == "oauth-user" && q.Get("code") == "" || c.AuthMode == "oauth-bot" && q.Get("app_installation_id") == "" {
		return &LoginError{Message: "callback is missing authorization result"}
	}
	b, err := m.load(ctx, p.Backend, p.SecretRef)
	if err != nil {
		return err
	}
	p.Consumed = true
	if err = m.savePending(c, p); err != nil {
		return err
	}
	ended = true
	b, exchangeErr := m.exchange(ctx, c, b, q.Get("code"), q.Get("app_installation_id"))
	if exchangeErr != nil {
		if err = m.clearPending(ctx, c); err != nil {
			return err
		}
		return exchangeErr
	}
	c.InstallationID = b.InstallationID
	if err = m.commitBundle(ctx, name, c, b, c.Backend); err != nil {
		var cleanup *CleanupError
		if !errors.As(err, &cleanup) {
			return &PartialError{}
		}
		ended = false
		if pendingErr := m.clearPending(ctx, c); pendingErr != nil {
			return errors.Join(err, &CleanupError{Phase: "New credentials saved; pending login cleanup failed; run auth logout --yes to repair"})
		}
		return err
	}
	ended = false
	if err = m.clearPending(ctx, c); err != nil {
		return &CleanupError{Phase: "New credentials saved; pending login cleanup failed; run auth logout --yes to repair"}
	}
	return nil
}
func (m *Manager) Cancel(ctx context.Context, name, id string) error {
	unlock, err := m.Store.ConnectionLock(ctx, name)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	c, err := m.current(name)
	if err != nil {
		return err
	}
	p, err := m.pending(c)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if p.ID != id {
		return nil
	}
	return m.clearPending(ctx, c)
}
