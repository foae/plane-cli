// Command genspec emits the reviewed offline command catalog.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foae/plane-cli/internal/spec"
)

func main() {
	input := flag.String("input", "api/catalog.json", "Reviewed catalog")
	output := flag.String("output", "internal/spec/catalog.gen.json", "Generated catalog")
	flag.Parse()
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	generated, err := spec.Generate(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	return os.WriteFile(output, generated, 0644)
}
