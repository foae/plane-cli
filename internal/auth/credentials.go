// Package auth resolves credentials independently of transport construction.
package auth

import (
	"time"

	"github.com/foae/plane-cli/internal/config"
)

// InputError is fixed, safe guidance for invalid invocation settings or input.
// Message must never contain provider, backend, path or credential values.
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

// LoginError is fixed, safe guidance about authentication lifecycle state.
// The CLI decides its exit code separately from invocation validation.
type LoginError struct{ Message string }

func (e *LoginError) Error() string { return e.Message }

type Credential struct {
	Header string
	Token  string
}
type Status struct {
	Mode              string     `json:"mode"`
	Backend           string     `json:"backend"`
	Status            string     `json:"status"`
	ExternallyManaged bool       `json:"externally_managed"`
	Binding           string     `json:"binding,omitempty"`
	Expiry            *time.Time `json:"expiry,omitempty"`
}

// Resolve handles only invocation overrides. Persistent resolution is separate
// so external authentication never opens the local store or keyring.
func Resolve(r config.Resolved, getenv func(string) string) (Credential, Status, error) {
	status := Status{Mode: r.AuthMode, Backend: r.Backend}
	if r.AuthMode == "external" {
		status.Status = "externally_managed"
		status.ExternallyManaged = true
		return Credential{}, status, nil
	}
	key, token := getenv("PLANE_CLI_API_KEY"), getenv("PLANE_CLI_ACCESS_TOKEN")
	if key != "" && token != "" {
		return Credential{}, status, &InputError{Message: "ambiguous token overrides: set only the token for the selected auth mode"}
	}
	var c Credential
	switch r.AuthMode {
	case "api-key":
		c = Credential{Header: "X-Api-Key", Token: key}
		if token != "" {
			return c, status, &InputError{Message: "access-token override does not match api-key mode"}
		}
	case "bearer", "oauth-user", "oauth-bot":
		c = Credential{Header: "Authorization", Token: token}
		if key != "" {
			return c, status, &InputError{Message: "API-key override does not match bearer mode"}
		}
	default:
		return c, status, &InputError{Message: "unsupported authentication mode"}
	}
	if c.Token == "" {
		status.Status = "missing"
		return c, status, nil
	}
	for i := range len(c.Token) {
		if (c.Token[i] < 0x20 && c.Token[i] != '\t') || c.Token[i] == 0x7f {
			return Credential{}, status, &InputError{Message: "credential contains invalid header characters"}
		}
	}
	status.Status = "externally_managed"
	status.ExternallyManaged = true
	if c.Header == "Authorization" {
		c.Token = "Bearer " + c.Token
	}
	return c, status, nil
}
