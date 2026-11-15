package cli

import (
	"errors"
	"io"
	"slices"
	"strings"

	"github.com/foae/plane-cli/internal/output"
	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

func commandsHandler(catalog spec.Catalog, out io.Writer) handler {
	return func(cmd *cobra.Command, args []string, _ map[string]any) error {
		version := cmd.Annotations["api-version"]
		type summary struct {
			Path       []string `json:"path"`
			Summary    string   `json:"summary"`
			Available  bool     `json:"available"`
			Versions   []string `json:"versions"`
			SideEffect string   `json:"side_effect"`
		}
		result := make([]summary, 0)
		for _, d := range catalog.Commands {
			if len(args) != 0 && args[0] != d.Path[0] {
				continue
			}
			versions := make([]string, 0, len(d.Versions))
			for v := range d.Versions {
				versions = append(versions, v)
			}
			slices.Sort(versions)
			_, available := d.Versions[version]
			result = append(result, summary{d.Path, d.Summary, available, versions, d.SideEffect})
		}
		if len(args) != 0 && len(result) == 0 {
			return errors.New("unknown command group")
		}
		return output.Success(out, commandName(cmd), result)
	}
}

func schemaHandler(catalog spec.Catalog, out io.Writer) handler {
	return func(cmd *cobra.Command, args []string, _ map[string]any) error {
		d, ok := catalog.Find(args)
		if !ok {
			return errors.New("unknown command path")
		}
		version := cmd.Annotations["api-version"]
		v, ok := d.Versions[version]
		if !ok {
			return unavailable("schema")
		}
		return output.Success(out, commandName(cmd), struct {
			Command    string       `json:"command"`
			Local      bool         `json:"local"`
			SideEffect string       `json:"side_effect"`
			Descriptor spec.Variant `json:"descriptor"`
		}{strings.Join(d.Path, " "), d.Local, d.SideEffect, v})
	}
}
