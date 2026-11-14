package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

// prepareResourceBody loads the full document before the selected offline schema
// validates it. Explicit body field bindings and a document are alternatives.
func prepareResourceBody(cmd *cobra.Command, input map[string]any, flags []spec.Flag) error {
	if cmd.Flags().Lookup("body-file") == nil || !cmd.Flags().Changed("body-file") {
		return nil
	}
	for _, flag := range flags {
		if strings.HasPrefix(flag.Binding, "/body/") && cmd.Flags().Changed(flag.Name) {
			return errors.New("body-file and body field flags are mutually exclusive")
		}
	}
	path, _ := cmd.Flags().GetString("body-file")
	reader := cmd.InOrStdin()
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return errors.New("cannot open JSON body file")
		}
		defer func() { _ = file.Close() }()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("JSON body file must be a regular file")
		}
		reader = file
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	// stdin is caller-owned; cancellation must not close it. The bounded read
	// can complete later but cannot trigger a request after this call returns.
	go func() {
		data, err := io.ReadAll(io.LimitReader(reader, spec.MaxInputBytes+1))
		done <- result{data, err}
	}()
	select {
	case <-cmd.Context().Done():
		return cmd.Context().Err()
	case read := <-done:
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if read.err != nil {
			return errors.New("cannot read JSON body")
		}
		value, err := spec.Decode(read.data)
		if err != nil {
			return err
		}
		input["body"] = value
		return nil
	}
}

func resourceBody(_ *cobra.Command, input map[string]any) (json.RawMessage, error) {
	value, ok := input["body"]
	if !ok {
		return nil, nil
	}
	return json.Marshal(value)
}
