package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxInputBytes = 8 << 20

var ErrInput = errors.New("input does not match the command schema")

// Decode preserves numbers and rejects duplicate keys at every depth and trailing values.
func Decode(data []byte) (any, error) {
	if len(data) > MaxInputBytes {
		return nil, ErrInput
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := decodeValue(d, 0)
	if err != nil {
		return nil, ErrInput
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrInput
	}
	return v, nil
}

func decodeValue(d *json.Decoder, depth int) (any, error) {
	if depth > 256 {
		return nil, ErrInput
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t {
	case json.Delim('{'):
		m := make(map[string]any)
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			k, ok := key.(string)
			if !ok {
				return nil, ErrInput
			}
			if _, exists := m[k]; exists {
				return nil, ErrInput
			}
			v, err := decodeValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, ErrInput
		}
		return m, nil
	case json.Delim('['):
		a := make([]any, 0)
		for d.More() {
			v, err := decodeValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, ErrInput
		}
		return a, nil
	default:
		if _, ok := t.(json.Delim); ok {
			return nil, ErrInput
		}
		return t, nil
	}
}

type offlineLoader struct{}

func (offlineLoader) Load(string) (any, error) {
	return nil, errors.New("external schema loading is forbidden")
}

func compile(schema json.RawMessage, resources map[string]json.RawMessage) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(offlineLoader{})
	for uri, raw := range resources {
		v, err := Decode(raw)
		if err != nil {
			return nil, err
		}
		if err := c.AddResource(uri, v); err != nil {
			return nil, err
		}
	}
	v, err := Decode(schema)
	if err != nil {
		return nil, err
	}
	const uri = "urn:plane-cli:selected-schema"
	if err := c.AddResource(uri, v); err != nil {
		return nil, err
	}
	return c.Compile(uri)
}

// Compile checks references and schema syntax with external retrieval disabled.
func Compile(schema json.RawMessage, resources map[string]json.RawMessage) error {
	_, err := compile(schema, resources)
	return err
}

// ErrSchema identifies invalid embedded metadata, not invalid caller input.
var ErrSchema = errors.New("command schema could not be compiled offline")

func Validate(schema json.RawMessage, input []byte) error {
	return ValidateWithResources(schema, input, nil)
}

func ValidateWithResources(schema json.RawMessage, input []byte, resources map[string]json.RawMessage) error {
	v, err := Decode(input)
	if err != nil {
		return ErrInput
	}
	s, err := compile(schema, resources)
	if err != nil {
		return ErrSchema
	}
	// Library errors include instance values; never expose them to a CLI diagnostic.
	if err := s.Validate(v); err != nil {
		return ErrInput
	}
	return nil
}
