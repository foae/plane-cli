package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/foae/plane-cli/internal/config"
)

// Bundle is stored only in the selected protected backend, never in config or output.
type Bundle struct {
	Version        int       `json:"version"`
	ConnectionID   string    `json:"connection_id"`
	BaseURL        string    `json:"base_url"`
	Mode           string    `json:"mode"`
	ClientID       string    `json:"client_id,omitempty"`
	InstallationID string    `json:"installation_id,omitempty"`
	AccessToken    string    `json:"access_token,omitempty"`
	RefreshToken   string    `json:"refresh_token,omitempty"`
	ClientSecret   string    `json:"client_secret,omitempty"`
	Expiry         time.Time `json:"expiry,omitempty"`
	Invalid        bool      `json:"invalid,omitempty"`
}

// TransportOptions contains invocation-effective OAuth transport settings.
// A nonnil Manager.Transport replaces both saved settings, including an empty CAFile.
type TransportOptions struct {
	RequestTimeout time.Duration
	CAFile         string
}

type Manager struct {
	Store     *config.Store
	Keyring   Keyring
	Transport *TransportOptions
}

func OpenManager() (*Manager, error) {
	path, err := config.Directory()
	if err != nil {
		return nil, err
	}
	s, err := config.Open(path, true)
	if err != nil {
		return nil, err
	}
	return &Manager{Store: s, Keyring: NativeKeyring()}, nil
}
func (m *Manager) Close() error { return m.Store.Close() }

func recordName(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return "credential-" + hex.EncodeToString(sum[:]) + ".json"
}
func (m *Manager) load(ctx context.Context, backend, ref string) (Bundle, error) {
	var b Bundle
	if ref == "" {
		return b, ErrCredentialNotFound
	}
	var data []byte
	var err error
	switch backend {
	case "file":
		data, err = m.Store.Read(recordName(ref))
		if errors.Is(err, os.ErrNotExist) {
			err = ErrCredentialNotFound
		}
	case "keyring":
		var text string
		text, err = m.Keyring.Get(ctx, ref)
		data = []byte(text)
	default:
		return b, &InputError{Message: "select file or keyring for persistent credentials"}
	}
	if err != nil {
		return b, err
	}
	if err = json.Unmarshal(data, &b); err != nil || b.Version != 1 {
		return Bundle{}, &LoginError{Message: "credential record is invalid; log in again"}
	}
	return b, nil
}
func (m *Manager) save(ctx context.Context, backend, ref string, b Bundle) error {
	b.Version = 1
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	switch backend {
	case "file":
		return m.Store.Write(recordName(ref), data)
	case "keyring":
		return m.Keyring.Set(ctx, ref, string(data))
	default:
		return &InputError{Message: "select file or keyring for persistent credentials"}
	}
}
func (m *Manager) remove(ctx context.Context, backend, ref string) error {
	if ref == "" {
		return nil
	}
	switch backend {
	case "file":
		return m.Store.Remove(recordName(ref))
	case "keyring":
		err := m.Keyring.Delete(ctx, ref)
		if errors.Is(err, ErrCredentialNotFound) {
			return nil
		}
		return err
	default:
		return &InputError{Message: "invalid credential backend"}
	}
}
func bound(b Bundle, c config.Connection) bool {
	return b.ConnectionID == c.ID && b.BaseURL == c.BaseURL && b.Mode == c.AuthMode && b.ClientID == c.OAuthClientID && b.InstallationID == c.InstallationID
}
func newBundle(c config.Connection) Bundle {
	return Bundle{Version: 1, ConnectionID: c.ID, BaseURL: c.BaseURL, Mode: c.AuthMode, ClientID: c.OAuthClientID, InstallationID: c.InstallationID}
}
func (m *Manager) current(name string) (config.Connection, error) {
	f, err := m.Store.Load()
	if err != nil {
		return config.Connection{}, err
	}
	c, ok := f.Connections[name]
	if !ok {
		return c, &LoginError{Message: "connection no longer exists"}
	}
	return c, nil
}

// repairStaged removes only an uncommitted destination. Keep its journal until
// removal is confirmed, and never delete the previously committed credential.
func (m *Manager) repairStaged(ctx context.Context, name string, c config.Connection) (config.Connection, error) {
	if c.StagedRef == "" {
		return c, nil
	}
	if err := m.remove(ctx, c.StagedBackend, c.StagedRef); err != nil {
		return c, &CleanupError{Phase: "Previous credentials retained; staged credential cleanup failed; retry login or run auth logout --yes"}
	}
	if err := m.Store.Update(ctx, func(f *config.File) error {
		now, ok := f.Connections[name]
		if !ok || now.ID != c.ID || now.Generation != c.Generation || now.StagedRef != c.StagedRef || now.StagedBackend != c.StagedBackend {
			return &LoginError{Message: "connection changed during staged credential cleanup"}
		}
		now.StagedBackend, now.StagedRef = "", ""
		f.Connections[name] = now
		return nil
	}); err != nil {
		return c, &CleanupError{Phase: "Previous credentials retained; staged credential removed but cleanup journal update failed; retry login or run auth logout --yes"}
	}
	c.StagedBackend, c.StagedRef = "", ""
	return c, nil
}

// commitBundle stages a new record before committing its reference. The old
// reference remains recoverable until cleanup completes, including after crashes.
// Caller holds the connection lock, never the config lock.
func (m *Manager) commitBundle(ctx context.Context, name string, c config.Connection, b Bundle, backend string) error {
	var err error
	c, err = m.repairStaged(ctx, name, c)
	if err != nil {
		return err
	}
	if c.CleanupRef != "" {
		return &CleanupError{Phase: "Credentials saved; previous backend cleanup is pending; run auth logout --yes to repair"}
	}
	ref, err := config.NewIdentity()
	if err != nil {
		return err
	}
	// Journal the destination before writing secrets, including a keyring write
	// that might succeed remotely and then return an error.
	err = m.Store.Update(ctx, func(f *config.File) error {
		now, ok := f.Connections[name]
		if !ok || now.ID != c.ID || now.Generation != c.Generation {
			return &LoginError{Message: "connection changed during authentication"}
		}
		now.StagedBackend, now.StagedRef = backend, ref
		f.Connections[name] = now
		return nil
	})
	if err != nil {
		return err
	}
	if err = m.save(ctx, backend, ref, b); err != nil {
		return err
	}
	err = m.Store.Update(ctx, func(f *config.File) error {
		now, ok := f.Connections[name]
		if !ok || now.ID != c.ID || now.Generation != c.Generation {
			return &LoginError{Message: "connection changed during authentication"}
		}
		now.CleanupBackend, now.CleanupRef = c.Backend, c.CredentialRef
		now.StagedBackend, now.StagedRef = "", ""
		now.Backend, now.CredentialRef = backend, ref
		now.InstallationID = c.InstallationID
		f.Connections[name] = now
		return nil
	})
	if err != nil {
		return err
	}
	if err = m.remove(ctx, c.Backend, c.CredentialRef); err != nil {
		return &CleanupError{Phase: "New credentials saved; previous backend cleanup failed; run auth logout --yes to repair"}
	}
	if err = m.Store.Update(ctx, func(f *config.File) error {
		now := f.Connections[name]
		now.CleanupBackend = ""
		now.CleanupRef = ""
		f.Connections[name] = now
		return nil
	}); err != nil {
		return &CleanupError{Phase: "New credentials saved; previous credential removed but cleanup journal update failed; run auth logout --yes to repair"}
	}
	return nil
}

// CleanupError contains only fixed phase descriptions, never backend errors.
type CleanupError struct{ Phase string }

func (e *CleanupError) Error() string { return e.Phase }
