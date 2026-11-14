// Package auth resolves credentials independently of transport construction.
package auth

import (
	"errors"

	"github.com/foae/plane-cli/internal/config"
)

type Credential struct {
	Header string
	Token  string
}
type Status struct {
	Mode              string `json:"mode"`
	Backend           string `json:"backend"`
	Status            string `json:"status"`
	ExternallyManaged bool   `json:"externally_managed"`
}

// Resolve never consults persistent stores in P01. External mode deliberately
// returns before even reading token environment variables.
func Resolve(r config.Resolved, getenv func(string) string) (Credential, Status, error) {
	status := Status{Mode: r.AuthMode, Backend: r.Backend}
	if r.AuthMode == "external" {
		status.Status = "externally_managed"
		status.ExternallyManaged = true
		return Credential{}, status, nil
	}
	key, token := getenv("PLANE_CLI_API_KEY"), getenv("PLANE_CLI_ACCESS_TOKEN")
	if key != "" && token != "" {
		return Credential{}, status, errors.New("ambiguous token overrides: set only the token for the selected auth mode")
	}
	var c Credential
	switch r.AuthMode {
	case "api-key":
		c = Credential{Header: "X-Api-Key", Token: key}
		if token != "" {
			return c, status, errors.New("access-token override does not match api-key mode")
		}
	case "bearer":
		c = Credential{Header: "Authorization", Token: token}
		if key != "" {
			return c, status, errors.New("API-key override does not match bearer mode")
		}
	default:
		return c, status, errors.New("OAuth is not available in this transport milestone")
	}
	if c.Token == "" {
		status.Status = "missing"
		return c, status, nil
	}
	for i := range len(c.Token) {
		if (c.Token[i] < 0x20 && c.Token[i] != '\t') || c.Token[i] == 0x7f {
			return Credential{}, status, errors.New("credential contains invalid header characters")
		}
	}
	status.Status = "externally_managed"
	status.ExternallyManaged = true
	if c.Header == "Authorization" {
		c.Token = "Bearer " + c.Token
	}
	return c, status, nil
}
