package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTaggedSourceBuildIdentity(t *testing.T) {
	source := t.TempDir()
	root := filepath.Join("..", "..")
	for _, dir := range []string{"cmd", "internal", "scripts"} {
		if err := os.CopyFS(filepath.Join(source, dir), os.DirFS(filepath.Join(root, dir))); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{"VERSION": "1.0.0-alpha.0\n", ".gitignore": "bin/\n"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = source
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "RELEASE_BASE=", "RELEASE_PREPARATION=")
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, data)
		}
		return data
	}
	// Git mutations are confined to this disposable fixture.
	run("git", "init", "-q")
	run("git", "add", ".")
	run("git", "-c", "user.name=Build fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "build fixture")
	run("git", "-c", "tag.gpgsign=false", "tag", "v9.0.0")
	binary := filepath.Join(source, "bin", "plane-cli")
	assertVersion := func(want string) {
		t.Helper()
		data := run(binary, "version")
		var result struct {
			Data struct {
				Version string `json:"version"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if result.Data.Version != want {
			t.Fatalf("version = %q, want %q", result.Data.Version, want)
		}
	}
	run("go", "build", "-o", binary, "./cmd/plane-cli")
	assertVersion("dev")
	run("sh", "scripts/build.sh", binary)
	assertVersion("dev")
	run("git", "-c", "tag.gpgsign=false", "tag", "v1.0.0-alpha.0")
	run("git", "-c", "tag.gpgsign=false", "tag", "latest")
	run("sh", "scripts/build.sh", binary)
	assertVersion("v1.0.0-alpha.0")
	if err := os.WriteFile(filepath.Join(source, "dirty.txt"), []byte("untracked change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("sh", "scripts/build.sh", binary)
	assertVersion("dev")
}
