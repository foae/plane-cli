// Command genspec emits the reviewed offline command catalog.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foae/plane-cli/internal/catalogsource"
	"github.com/foae/plane-cli/internal/spec"
)

func main() {
	input := flag.String("input", "api/catalog", "Editable catalog source directory")
	catalog := flag.String("catalog", "api/catalog.json", "Generated aggregate catalog")
	output := flag.String("output", "internal/spec/catalog.gen.json", "Generated catalog")
	flag.Parse()
	if err := run(*input, *catalog, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, catalog, output string) error {
	data, err := catalogsource.Assemble(input)
	if err != nil {
		return err
	}
	generated, err := spec.Generate(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(catalog, data, 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	return os.WriteFile(output, generated, 0644)
}
