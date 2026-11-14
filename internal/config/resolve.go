package config

import (
	"errors"
	"os"
	"time"
)

type Resolved struct {
	Name string `json:"connection,omitempty"`
	Connection
	Sources map[string]string `json:"sources"`
}

// Resolve uses only nonsecret configuration. The flag map retains presence so
// explicit empty values are not silently replaced by environment defaults.
func Resolve(f File, flags map[string]string, getenv func(string) string) (Resolved, error) {
	r := Resolved{Sources: map[string]string{}}
	name, source := f.Default, "default"
	if s := getenv("PLANE_CLI_CONNECTION"); s != "" {
		name, source = s, "environment"
	}
	if s, ok := flags["connection"]; ok {
		name, source = s, "flag"
	}
	r.Name = name
	r.Sources["connection"] = source
	if name != "" {
		c, ok := f.Connections[name]
		if !ok {
			return r, errors.New("selected connection does not exist")
		}
		r.Connection = c
	}
	fields := []struct {
		key, env string
		dst      *string
		fallback string
	}{
		{"base-url", "BASE_URL", &r.BaseURL, ""}, {"api-version", "API_VERSION", &r.APIVersion, ""},
		{"workspace", "WORKSPACE", &r.Workspace, ""}, {"project", "PROJECT", &r.Project, ""},
		{"auth-mode", "AUTH_MODE", &r.AuthMode, "api-key"}, {"credential-backend", "CREDENTIAL_BACKEND", &r.Backend, "file"},
		{"timeout", "TIMEOUT", &r.Timeout, "5m"}, {"request-timeout", "REQUEST_TIMEOUT", &r.RequestTimeout, "30s"}, {"ca-file", "CA_FILE", &r.CAFile, ""},
	}
	for _, field := range fields {
		src := "connection"
		if *field.dst == "" {
			*field.dst = field.fallback
			src = "default"
		}
		if s := getenv("PLANE_CLI_" + field.env); s != "" {
			*field.dst = s
			src = "environment"
		}
		if s, ok := flags[field.key]; ok {
			*field.dst = s
			src = "flag"
		}
		r.Sources[field.key] = src
	}
	c, e := Validate(r.Connection)
	r.Connection = c
	return r, e
}
func Read() (File, error) {
	path, e := Directory()
	if e != nil {
		return File{}, e
	}
	s, e := Open(path, false)
	if errors.Is(e, os.ErrNotExist) {
		return File{Version: Version, Connections: map[string]Connection{}}, nil
	}
	if e != nil {
		return File{}, e
	}
	defer func() { _ = s.Close() }()
	return s.Load()
}
func (r Resolved) Deadlines() (time.Duration, time.Duration) {
	total, _ := time.ParseDuration(r.Timeout)
	request, _ := time.ParseDuration(r.RequestTimeout)
	return total, request
}
