package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/foae/plane-cli/internal/spec"
)

func readBody(r io.Reader) (json.RawMessage, error) {
	data, err := io.ReadAll(io.LimitReader(r, spec.MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JSON body: %w", err)
	}
	if len(data) > spec.MaxInputBytes {
		return nil, fmt.Errorf("JSON body exceeds the 8 MiB limit: %w", spec.ErrInput)
	}
	if !utf8.Valid(data) {
		return nil, spec.ErrInput
	}
	if _, err := spec.Decode(data); err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
