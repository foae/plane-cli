package catalogsource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, path, content string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	put(t, root, "metadata.json", `{"schema_version":1}`)
	put(t, root, "commands/b.json", `[{"path":["z"],"schema":{"maximum":9007199254740993}}]`)
	put(t, root, "commands/a.json", `[{"path":["a"]}]`)
	put(t, root, "operations/P01.json", `[{"id":"first","method":"GET"}]`)
	put(t, root, "sources/P01.json", `[]`)
	return root
}

func TestAssemblePreservesContracts(t *testing.T) {
	root := fixture(t)
	data, err := Assemble(root)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "commands": [
    {
      "path": [
        "a"
      ]
    },
    {
      "path": [
        "z"
      ],
      "schema": {
        "maximum": 9007199254740993
      }
    }
  ],
  "operations": [
    {
      "id": "first",
      "method": "GET"
    }
  ],
  "schema_version": 1,
  "sources": []
}
`
	if string(data) != want {
		t.Fatalf("catalog values/order changed:\n%s", data)
	}
	put(t, root, "aggregate.json", string(data))
	if err := Check(root, filepath.Join(root, "aggregate.json")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "operations/P01.json", `[{"id":"first","method":"POST"}]`)
	if err := Check(root, filepath.Join(root, "aggregate.json")); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("changed operation must invalidate aggregate: %v", err)
	}
}

func TestAssembleRejectsAmbiguousSources(t *testing.T) {
	for _, tc := range []struct {
		name, path, content, want string
	}{
		{"duplicate-metadata", "metadata.json", `{"schema_version":1,"schema_version":2}`, "metadata.json"},
		{"duplicate-contract", "operations/P01.json", `[{"method":"GET","method":"POST"}]`, "P01.json"},
		{"shadowed-section", "metadata.json", `{"commands":[]}`, "sharded section commands"},
		{"null-shard", "sources/P01.json", `null`, "must be an array"},
		{"trailing-value", "sources/P01.json", `[] []`, "P01.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			put(t, root, tc.path, tc.content)
			if _, err := Assemble(root); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}
