package spec

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

//go:embed catalog.gen.json
var embedded []byte

type Catalog struct {
	SchemaVersion int                        `json:"schema_version"`
	Commands      []Command                  `json:"commands"`
	Resources     map[string]json.RawMessage `json:"resources,omitempty"`
}

type Command struct {
	Path       []string           `json:"path"`
	Handler    string             `json:"handler"`
	Summary    string             `json:"summary"`
	SideEffect string             `json:"side_effect"`
	Examples   []string           `json:"examples"`
	Versions   map[string]Variant `json:"versions"`
	Local      bool               `json:"local"`
}

type Variant struct {
	Schema             json.RawMessage   `json:"schema"`
	Flags              []Flag            `json:"flags"`
	Method             string            `json:"method,omitempty"`
	Route              string            `json:"route,omitempty"`
	Routes             map[string]string `json:"routes,omitempty"`
	Pagination         string            `json:"pagination,omitempty"`
	TableColumns       []string          `json:"table_columns,omitempty"`
	ResponseKind       string            `json:"response_kind,omitempty"`
	Scopes             []string          `json:"scopes,omitempty"`
	Sensitivity        json.RawMessage   `json:"sensitivity,omitempty"`
	EmptySuccessStatus int               `json:"empty_success_status,omitempty"`
}

type Flag struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Binding     string `json:"binding"`
	Description string `json:"description"`
}

func Load() (Catalog, error) {
	var c Catalog
	err := json.Unmarshal(embedded, &c)
	return c, err
}

// Generate emits only callable descriptors and their offline resources, never pending APIs.
func Generate(source []byte) ([]byte, error) {
	// The build-time catalog grows with API coverage; the runtime body limit
	// applies to caller input, not this already-loaded repository artifact.
	if _, err := DecodeWithLimit(source, len(source)); err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(source, &c); err != nil {
		return nil, err
	}
	if err := CheckCatalog(c); err != nil {
		return nil, err
	}
	slices.SortFunc(c.Commands, func(a, b Command) int { return strings.Compare(strings.Join(a.Path, " "), strings.Join(b.Path, " ")) })
	out, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func (c Catalog) Find(path []string) (Command, bool) {
	for _, d := range c.Commands {
		if slices.Equal(d.Path, path) {
			return d, true
		}
	}
	return Command{}, false
}

func CheckCatalog(c Catalog) error {
	if c.SchemaVersion != 1 {
		return errors.New("unsupported catalog schema version")
	}
	paths := make(map[string]bool)
	for _, d := range c.Commands {
		path := strings.Join(d.Path, " ")
		if len(d.Path) == 0 || paths[path] || d.Handler == "" || d.Summary == "" || d.SideEffect == "" || len(d.Versions) == 0 {
			return fmt.Errorf("invalid or duplicate command descriptor: %q", path)
		}
		paths[path] = true
		for _, token := range d.Path {
			if !validName(token) {
				return errors.New("invalid command path token")
			}
		}
		union := make(map[string]Flag)
		for version, v := range d.Versions {
			if version != "v1" && version != "v2" {
				return errors.New("invalid API version")
			}
			if v.EmptySuccessStatus != 0 && (v.EmptySuccessStatus < 200 || v.EmptySuccessStatus >= 300) {
				return errors.New("empty success status must be a 2xx status")
			}
			if err := Compile(v.Schema, c.Resources); err != nil {
				return fmt.Errorf("schema for %s: %w", path, err)
			}
			seen := make(map[string]bool)
			for _, f := range v.Flags {
				if !validName(f.Name) || seen[f.Name] || f.Description == "" {
					return errors.New("invalid or duplicate flag descriptor")
				}
				seen[f.Name] = true
				if f.Type != "string" && f.Type != "boolean" && f.Type != "integer" {
					return errors.New("unsupported flag type")
				}
				if prior, ok := union[f.Name]; ok && (prior.Type != f.Type || prior.Binding != f.Binding) {
					return errors.New("inconsistent cross-version flag binding")
				}
				union[f.Name] = f
				if !d.Local && f.Name == "api-version" && (f.Type != "string" || !f.Required || f.Binding != "/flags/api-version") {
					return fmt.Errorf("%s %s: api-version must be a required string bound to /flags/api-version", path, version)
				}
				if err := checkBinding(v.Schema, f); err != nil {
					return fmt.Errorf("%s --%s: %w", path, f.Name, err)
				}
			}
			if !d.Local && !seen["api-version"] {
				return fmt.Errorf("%s %s: missing required api-version selector", path, version)
			}
		}
	}
	return nil
}

func validName(s string) bool {
	if s == "" || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, ch := range s {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}

func checkBinding(raw json.RawMessage, f Flag) error {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return err
	}
	parts := strings.Split(f.Binding, "/")
	if len(parts) < 3 || parts[0] != "" || (parts[1] != "flags" && parts[1] != "body" && parts[1] != "query") {
		return errors.New("flag must bind to a flags/body/query schema property")
	}
	required := true
	for _, part := range parts[1:] {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		propertyRequired := false
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				if r == part {
					propertyRequired = true
				}
			}
		}
		required = required && propertyRequired
		props, _ := schema["properties"].(map[string]any)
		next, ok := props[part].(map[string]any)
		if !ok {
			return errors.New("flag has no schema property mapping")
		}
		schema = next
	}
	typeMatches := schema["type"] == f.Type
	if types, ok := schema["type"].([]any); ok {
		typeMatches = slices.Contains(types, any(f.Type))
	}
	bodyConvenience := parts[1] == "body" && !f.Required
	if !typeMatches || (required != f.Required && !bodyConvenience) {
		return errors.New("flag type/requiredness differs from schema")
	}
	return nil
}
