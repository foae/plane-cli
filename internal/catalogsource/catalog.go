// Package catalogsource assembles the editable, build-time API catalog shards.
package catalogsource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foae/plane-cli/internal/spec"
)

// Assemble combines metadata and sorted shard files without changing record values.
func Assemble(dir string) ([]byte, error) {
	read := func(path string) (any, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		value, err := spec.DecodeWithLimit(data, len(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return value, nil
	}
	value, err := read(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, err
	}
	catalog, ok := value.(map[string]any)
	if !ok || catalog == nil {
		return nil, fmt.Errorf("catalog metadata must be an object")
	}
	for _, section := range []string{"commands", "operations", "sources"} {
		if _, exists := catalog[section]; exists {
			return nil, fmt.Errorf("catalog metadata must not contain sharded section %s", section)
		}
		paths, err := filepath.Glob(filepath.Join(dir, section, "*.json"))
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("catalog section %s has no shards", section)
		}
		rows := []any{}
		for _, path := range paths {
			value, err := read(path)
			if err != nil {
				return nil, err
			}
			shard, ok := value.([]any)
			if !ok {
				return nil, fmt.Errorf("%s: catalog shard must be an array", path)
			}
			rows = append(rows, shard...)
		}
		catalog[section] = rows
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Check rejects a stale aggregate before coverage or release checks consume it.
func Check(dir, aggregate string) error {
	generated, err := Assemble(dir)
	if err != nil {
		return err
	}
	committed, err := os.ReadFile(aggregate)
	if err != nil {
		return err
	}
	if !bytes.Equal(generated, committed) {
		return fmt.Errorf("stale generated API catalog; run make generate")
	}
	return nil
}
