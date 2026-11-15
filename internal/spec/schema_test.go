package spec

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSchemaPresenceCompositionAndPrecision(t *testing.T) {
	schema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["value"],"properties":{"value":{"anyOf":[{"type":"null"},{"type":"integer","minimum":9007199254740993,"maximum":9007199254740993}]},"enabled":{"type":"boolean"}},"additionalProperties":false}`)
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"value":9007199254740993}`, true}, {`{"value":9007199254740992}`, false}, {`{"value":9007199254740994}`, false},
		{`{"value":null,"enabled":false}`, true}, {`{}`, false}, {`{"value":"9007199254740993"}`, false},
		{`{"value":null,"value":9007199254740993}`, false}, {`{"value":null} {}`, false}, {`{"value":null,"unknown":1}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			err := Validate(schema, []byte(tc.body))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
	for _, data := range []string{`{"a":{"x":1,"x":2}}`, `[{"x":1,"x":2}]`, `[1,]`, `null true`} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("accepted invalid JSON %s", data)
		}
	}
}

func TestOfflineReferences(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.Close()
	schema := json.RawMessage(`{"$ref":"` + s.URL + `/schema"}`)
	if err := Compile(schema, nil); err == nil {
		t.Fatal("external reference accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("schema compiler accessed network")
	}
	resources := map[string]json.RawMessage{s.URL + "/schema": json.RawMessage(`{"type":"integer","const":9007199254740993}`)}
	if err := ValidateWithResources(schema, []byte(`9007199254740993`), resources); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWithResources(schema, []byte(`9007199254740992`), resources); err == nil {
		t.Fatal("rounded reference integer accepted")
	}
}

func TestDescriptorBindingRejectsDrift(t *testing.T) {
	d := Command{Path: []string{"fixture", "create"}, Handler: "fixture", Summary: "Fixture", SideEffect: "create", Versions: map[string]Variant{"v2": {
		Schema: json.RawMessage(`{"type":"object","required":["body","flags"],"properties":{"body":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},"flags":{"type":"object","properties":{"api-version":{"type":"string"}}}}}`),
		Flags: []Flag{
			{Name: "name", Type: "string", Required: true, Binding: "/body/name", Description: "Name"},
			{Name: "api-version", Type: "string", Required: false, Binding: "/flags/api-version", Description: "API generation"},
		},
	}}}
	c := Catalog{SchemaVersion: 1, Commands: []Command{d}}
	if err := CheckCatalog(c); err != nil {
		t.Fatal(err)
	}
	v := d.Versions["v2"]
	v.Flags[0].Type = "integer"
	if err := CheckCatalog(c); err == nil {
		t.Fatal("schema and flag type disagreement accepted")
	}
}

func TestDescriptorBindingRequiredAncestors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schema   string
		required bool
		valid    bool
	}{
		{"required chain", `{"type":"object","required":["body"],"properties":{"body":{"type":"object","required":["details"],"properties":{"details":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}}}}`, true, true},
		{"optional body rejects required flag", `{"type":"object","properties":{"body":{"type":"object","required":["details"],"properties":{"details":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}}}}`, true, false},
		{"optional intermediate rejects required flag", `{"type":"object","required":["body"],"properties":{"body":{"type":"object","properties":{"details":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}}}}`, true, false},
		{"optional ancestor permits optional flag", `{"type":"object","properties":{"body":{"type":"object","required":["details"],"properties":{"details":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}}}}`, false, true},
		{"optional leaf permits optional flag", `{"type":"object","required":["body"],"properties":{"body":{"type":"object","required":["details"],"properties":{"details":{"type":"object","properties":{"name":{"type":"string"}}}}}}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Catalog{SchemaVersion: 1, Commands: []Command{{
				Path: []string{"fixture"}, Handler: "fixture", Summary: "Fixture", SideEffect: "none", Local: true,
				Versions: map[string]Variant{"v2": {
					Schema: json.RawMessage(tc.schema),
					Flags:  []Flag{{Name: "name", Type: "string", Required: tc.required, Binding: "/body/details/name", Description: "Name"}},
				}},
			}}}
			if err := CheckCatalog(c); (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}

func TestCatalogRequiresAPISelector(t *testing.T) {
	const schema = `{"type":"object","required":["flags"],"properties":{"flags":{"type":"object","properties":{"api-version":{"type":"string"}}}}}`
	selector := Flag{Name: "api-version", Type: "string", Required: false, Binding: "/flags/api-version", Description: "API generation"}
	for _, tc := range []struct {
		name   string
		change func(*Variant)
		valid  bool
	}{
		{"optional selector", func(v *Variant) {}, true},
		{"missing selector", func(v *Variant) { v.Flags = nil }, false},
		{"renamed selector", func(v *Variant) { v.Flags[0].Name = "generation" }, false},
		{"required selector", func(v *Variant) {
			v.Flags[0].Required = true
			v.Schema = json.RawMessage(`{"type":"object","required":["flags"],"properties":{"flags":{"type":"object","required":["api-version"],"properties":{"api-version":{"type":"string"}}}}}`)
		}, false},
		{"schema-required selector", func(v *Variant) {
			v.Schema = json.RawMessage(`{"type":"object","required":["flags"],"properties":{"flags":{"type":"object","required":["api-version"],"properties":{"api-version":{"type":"string"}}}}}`)
		}, false},
		{"optional selector ancestor", func(v *Variant) {
			v.Schema = json.RawMessage(`{"type":"object","properties":{"flags":{"type":"object","properties":{"api-version":{"type":"string"}}}}}`)
		}, true},
		{"missing schema selector", func(v *Variant) {
			v.Schema = json.RawMessage(`{"type":"object","properties":{"flags":{"type":"object"}}}`)
		}, false},
		{"non-string selector", func(v *Variant) {
			v.Flags[0].Type = "boolean"
			v.Schema = json.RawMessage(`{"type":"object","required":["flags"],"properties":{"flags":{"type":"object","properties":{"api-version":{"type":"boolean"}}}}}`)
		}, false},
		{"body-bound selector", func(v *Variant) {
			v.Flags[0].Binding = "/body/api-version"
			v.Schema = json.RawMessage(`{"type":"object","required":["body"],"properties":{"body":{"type":"object","properties":{"api-version":{"type":"string"}}}}}`)
		}, false},
	} {
		for _, version := range []string{"v1", "v2"} {
			t.Run(tc.name+"/"+version, func(t *testing.T) {
				variants := map[string]Variant{
					"v1": {Schema: json.RawMessage(schema), Flags: []Flag{selector}},
					"v2": {Schema: json.RawMessage(schema), Flags: []Flag{selector}},
				}
				v := variants[version]
				tc.change(&v)
				variants[version] = v
				c := Catalog{SchemaVersion: 1, Commands: []Command{{
					Path: []string{"fixture"}, Handler: "fixture", Summary: "Fixture", SideEffect: "none", Versions: variants,
				}}}
				if err := CheckCatalog(c); (err == nil) != tc.valid {
					t.Fatalf("valid=%v error=%v", tc.valid, err)
				}
			})
		}
	}
}
