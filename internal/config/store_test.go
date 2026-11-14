package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreProcess(t *testing.T) {
	if path := os.Getenv("PLANE_TEST_STORE"); path != "" {
		s, e := Open(path, true)
		if e != nil {
			t.Fatal(e)
		}
		defer func() { _ = s.Close() }()
		e = s.Update(context.Background(), func(f *File) error {
			f.Connections[os.Getenv("PLANE_TEST_NAME")] = Connection{BaseURL: "https://example.test", APIVersion: "v2"}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
		return
	}
	path := filepath.Join(privateTempDir(t), "config")
	a := exec.Command(os.Args[0], "-test.run=^TestStoreProcess$")
	b := exec.Command(os.Args[0], "-test.run=^TestStoreProcess$")
	a.Env = append(os.Environ(), "PLANE_TEST_STORE="+path, "PLANE_TEST_NAME=a")
	b.Env = append(os.Environ(), "PLANE_TEST_STORE="+path, "PLANE_TEST_NAME=b")
	if e := a.Start(); e != nil {
		t.Fatal(e)
	}
	if e := b.Start(); e != nil {
		t.Fatal(e)
	}
	if e := a.Wait(); e != nil {
		t.Fatal(e)
	}
	if e := b.Wait(); e != nil {
		t.Fatal(e)
	}
	s, e := Open(path, false)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	f, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := f.Connections["a"]; !ok {
		t.Fatal("lost a")
	}
	if _, ok := f.Connections["b"]; !ok {
		t.Fatal("lost b")
	}
	info, e := os.Stat(filepath.Join(path, "config.json"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe config: %v %v", info, e)
	}
}
func TestStoreRefusesSymlinksAndFutureVersions(t *testing.T) {
	dir := privateTempDir(t)
	s, e := Open(dir, true)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	outside := filepath.Join(t.TempDir(), "record")
	if e = os.WriteFile(outside, []byte("unchanged"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, filepath.Join(dir, "secret")); e != nil {
		t.Fatal(e)
	}
	if e = s.Write("secret", []byte("changed")); e == nil {
		t.Fatal("followed symlink")
	}
	b, e := os.ReadFile(outside)
	if e != nil || string(b) != "unchanged" {
		t.Fatal("outside modified")
	}
	if e = s.Write("config.json", []byte(`{"version":99}`)); e != nil {
		t.Fatal(e)
	}
	if e = s.Update(context.Background(), func(f *File) error { f.Default = "changed"; return nil }); e == nil {
		t.Fatal("accepted future version")
	}
	b, e = s.Read("config.json")
	if e != nil || string(b) != `{"version":99}` {
		t.Fatal("future file changed")
	}
	parent := filepath.Join(t.TempDir(), "link")
	if e = os.Symlink(dir, parent); e != nil {
		t.Fatal(e)
	}
	if other, e := Open(parent, true); e == nil {
		_ = other.Close()
		t.Fatal("followed parent symlink")
	}
}
func TestStoreRejectsOversizedUpdate(t *testing.T) {
	s, e := Open(privateTempDir(t), true)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	if e = s.Update(context.Background(), func(f *File) error {
		f.Default = "saved"
		f.Connections["saved"] = Connection{BaseURL: "https://example.test", APIVersion: "v2"}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	before, e := s.Read("config.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Update(context.Background(), func(f *File) error {
		f.Default = strings.Repeat("x", 8<<20)
		return nil
	}); e == nil {
		t.Fatal("accepted oversized update")
	}
	after, e := s.Read("config.json")
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("oversized update changed saved configuration")
	}
	f, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	if f.Default != "saved" || f.Connections["saved"].BaseURL != "https://example.test" {
		t.Fatal("saved configuration was not retained")
	}
}

func TestStoreRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, record := range []string{
		`{"version":1,"unknown":"preserve"}`,
		`{"version":1,"connections":{"saved":{"unknown":"preserve"}}}`,
		`{"version":1} {"version":1}`,
		`{"version":1} trailing`,
	} {
		t.Run(record, func(t *testing.T) {
			s, e := Open(privateTempDir(t), true)
			if e != nil {
				t.Fatal(e)
			}
			defer func() { _ = s.Close() }()
			if e = s.Write("config.json", []byte(record)); e != nil {
				t.Fatal(e)
			}
			if _, e = s.Load(); e == nil {
				t.Fatal("accepted invalid configuration")
			}
			called := false
			if e = s.Update(context.Background(), func(f *File) error {
				called = true
				f.Default = "changed"
				return nil
			}); e == nil {
				t.Fatal("accepted update of invalid configuration")
			}
			if called {
				t.Fatal("called update callback for invalid configuration")
			}
			after, e := s.Read("config.json")
			if e != nil {
				t.Fatal(e)
			}
			if string(after) != record {
				t.Fatal("invalid configuration changed")
			}
		})
	}
}

func TestLockCancellation(t *testing.T) {
	s, e := Open(privateTempDir(t), true)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	unlock, e := s.ConnectionLock(context.Background(), "same")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = unlock() }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.ConnectionLock(ctx, "same"); !errors.Is(e, context.Canceled) {
		t.Fatalf("lock cancellation: %v", e)
	}
}
func TestResolution(t *testing.T) {
	f := File{Version: 1, Default: "saved", Connections: map[string]Connection{"saved": {BaseURL: "https://EXAMPLE.test:443/prefix/", APIVersion: "v1", AuthMode: "api-key", Backend: "file"}}}
	env := map[string]string{"PLANE_CLI_API_VERSION": "v2", "PLANE_CLI_TIMEOUT": "1m"}
	r, e := Resolve(f, map[string]string{"timeout": "2m"}, func(s string) string { return env[s] })
	if e != nil {
		t.Fatal(e)
	}
	total, _ := r.Deadlines()
	if r.BaseURL != "https://example.test/prefix" || r.APIVersion != "v2" || total != 2*time.Minute || r.Sources["timeout"] != "flag" {
		t.Fatalf("bad resolution: %+v", r)
	}
	for _, base := range []string{"https://user:pass@host", "https://host/api/v2/", "https://host/a/../b", "https://host/%2fsecret", "https://host/a//b", "https://host?x=y", "https://host/#", "https://host/%252f"} {
		if _, e := CanonicalBase(base); e == nil {
			t.Errorf("accepted unsafe base %s", base)
		}
	}
}

func privateTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
