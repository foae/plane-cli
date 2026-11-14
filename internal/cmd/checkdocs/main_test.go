package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func docsFixture(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, dir := range []string{"docs", "api"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	put(t, "api/catalog.json", string(data))
	put(t, "docs/usage.md", "# Current usage\n\n```sh\nplane-cli version\nplane-cli commands --api-version v2\nplane-cli schema version --api-version v2\n```\n")
	put(t, "README.md", "# Offline docs\n\n[Usage](docs/usage.md#current-usage)\n[Schema](api/catalog.json#/commands/0/versions/v2/schema)\n")
}

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalDocumentation(t *testing.T) {
	docsFixture(t)
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBrokenDocumentation(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"missing-target", "[Missing](docs/missing.md)", "missing target"},
		{"missing-fragment", "[Missing](docs/usage.md#not-present)", "missing fragment"},
		{"missing-schema-fragment", "[Missing](api/catalog.json#/commands/999/schema)", "missing JSON fragment"},
		{"escape", "[Outside](../secret.md)", "leaves repository"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			docsFixture(t)
			put(t, "README.md", test.content)
			if err := run(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
}

func TestRejectRemoteSchemaLoading(t *testing.T) {
	docsFixture(t)
	put(t, "api/catalog.json", `{"commands":[],"operations":[{"request_schema":{"$ref":"https://example.invalid/schema.json"},"response_schema":true}],"resources":{}}`)
	if err := run(); err == nil || !strings.Contains(err.Error(), "embedded schema") {
		t.Fatalf("got %v", err)
	}
}

func TestRejectUnimplementedCurrentExample(t *testing.T) {
	docsFixture(t)
	put(t, "docs/usage.md", "# Current usage\n\n```sh\nplane-cli project get --api-version v2\n```\n")
	if err := usageExamples(); err == nil || !strings.Contains(err.Error(), "not an implemented local command") {
		t.Fatalf("got %v", err)
	}
}

func TestDesignExamplesDoNotExecute(t *testing.T) {
	docsFixture(t)
	put(t, "docs/future.md", "# Future\n\n```sh\nplane-cli project delete --yes\n```\n")
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

func TestRepeatedHeadingAnchors(t *testing.T) {
	ids := anchors("# Header\n\n## Repeat\n\n## Repeat\n\n```md\n# Not a heading\n```\n")
	if !ids["repeat"] || !ids["repeat-1"] || ids["not-a-heading"] {
		t.Fatalf("anchors: %v", ids)
	}
}

func TestConfigurationExamplesAreIsolated(t *testing.T) {
	docsFixture(t)
	home := t.TempDir()
	t.Setenv("PLANE_CLI_CONFIG_DIR", home)
	t.Setenv("PLANE_CLI_TIMEOUT", "invalid")
	put(t, "docs/usage.md", "plane-cli connection add example --base-url http://example.test --api-version v2\nplane-cli connection use example\nplane-cli connection delete example --yes\n")
	if err := usageExamples(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("examples modified original configuration: %v %v", entries, err)
	}
	if os.Getenv("PLANE_CLI_CONFIG_DIR") != home || os.Getenv("PLANE_CLI_TIMEOUT") != "invalid" {
		t.Fatal("examples did not restore settings")
	}
}

func TestManualAuthenticationExamplesDoNotExecute(t *testing.T) {
	docsFixture(t)
	put(t, "docs/usage.md", "```sh manual\nplane-cli auth login --connection absent --api-key-file /unavailable\n```\n```sh\nplane-cli version\n```\n")
	if err := usageExamples(); err != nil {
		t.Fatal(err)
	}
}
