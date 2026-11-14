package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineProcessContract(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "plane-cli")
	source := t.TempDir()
	for _, dir := range []string{"cmd", "internal"} {
		if err := os.CopyFS(filepath.Join(source, dir), os.DirFS(filepath.Join("../..", dir))); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/plane-cli")
	build.Dir = source
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "config"), "HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1")
	var version string
	for _, tc := range []struct {
		args []string
		code int
		text bool
	}{
		{nil, 0, true}, {[]string{"--help"}, 0, true}, {[]string{"version"}, 0, false}, {[]string{"--version"}, 0, false},
		{[]string{"commands", "--api-version", "v2"}, 0, false}, {[]string{"schema", "version", "--api-version", "v1"}, 0, false},
		{[]string{"PRIVATE"}, 2, false}, {[]string{"version", "--PRIVATE"}, 2, false},
		{[]string{"help", "version"}, 0, true}, {[]string{"help", "PRIVATE"}, 2, false},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := exec.Command(binary, tc.args...)
			cmd.Env = env
			cmd.Dir = home
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.code {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			if code != 0 {
				if stdout.Len() != 0 || !json.Valid(stderr.Bytes()) || strings.Count(stderr.String(), "\n") != 1 || strings.Contains(stderr.String(), "PRIVATE") {
					t.Fatalf("unsafe failure stdout=%s stderr=%s", &stdout, &stderr)
				}
			} else if stderr.Len() != 0 || (!tc.text && !json.Valid(stdout.Bytes())) {
				t.Fatalf("stdout=%s stderr=%s", &stdout, &stderr)
			}
			if len(tc.args) == 1 && tc.args[0] == "version" {
				version = stdout.String()
			}
			if len(tc.args) == 1 && tc.args[0] == "--version" && stdout.String() != version {
				t.Fatal("version forms differ")
			}
		})
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("offline commands wrote home: %v", entries)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, args := range [][]string{{"version"}, {"help"}, {"help", "version"}} {
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		cmd.Stdout = w
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || !json.Valid(stderr.Bytes()) || !strings.Contains(stderr.String(), "output_error") {
			t.Fatalf("closed pipe err=%v stderr=%s", err, &stderr)
		}
	}
}
