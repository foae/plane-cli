package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Copy the actual recipes and implementation into a standalone temporary module:
// no mocked go/make executable, no project tags, no working-copy mutations.
func TestActualMakeReleaseRecipesInTemporaryRepository(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate recipe fixtures")
	}
	sourceDir := filepath.Dir(sourceFile)
	repository := filepath.Clean(filepath.Join(sourceDir, "..", "..", ".."))
	c := repoFixture(t, initialVersion)
	copyFile := func(source, relative string) {
		t.Helper()
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		put(t, c.root, relative, string(data))
	}
	for _, path := range []string{"Makefile", "tools/versions.mk", "go.mod", "go.sum", "internal/semver/version.go"} {
		copyFile(filepath.Join(repository, path), path)
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			copyFile(filepath.Join(sourceDir, entry.Name()), filepath.Join("internal/cmd/release", entry.Name()))
		}
	}
	base := commitFixture(t, c.root)
	makeRun := func(wantSuccess bool, args ...string) string {
		t.Helper()
		cmd := exec.Command("make", args...)
		cmd.Dir = c.root
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "RELEASE_PREPARATION=")
		data, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("make %v: %v\n%s", args, err, data)
		}
		return string(data)
	}
	summary := "Add 'quoted' behavior; preserve $PLANE_TOKEN, $(REFERENCE), ${VALUE} and $$"
	makeRun(true, "change", "LEVEL=minor", "SUMMARY="+summary)
	s := stateFixture(t, c)
	if len(s.fragments) != 1 || s.fragments[0].note.Level != "minor" || s.fragments[0].note.Summary != summary {
		t.Fatalf("actual Make change did not preserve fragment: %+v", s.fragments)
	}
	commitFixture(t, c.root)
	makeRun(true, "version-check", "BASE="+base)
	planOutput := makeRun(true, "version-plan")
	if !strings.Contains(planOutput, `"required_bump": "minor"`) || !strings.Contains(planOutput, `"minimum_version": "1.0.0-alpha.1"`) || !strings.Contains(planOutput, summary) {
		t.Fatalf("actual Make plan omitted release inputs:\n%s", planOutput)
	}
	prepareBase := gitTest(t, c.root, "rev-parse", "HEAD")
	makeRun(true, "release-prepare", "VERSION=1.0.0-alpha.1")
	s = stateFixture(t, c)
	if s.current.String() != "1.0.0-alpha.1" || len(s.fragments) != 0 || s.history[s.current.String()].Changes[0].Summary != summary {
		t.Fatalf("actual Make preparation produced incorrect state: %+v", s)
	}
	commitFixture(t, c.root)
	makeRun(true, "version-check", "BASE="+prepareBase, "RELEASE_PREPARATION=1")
	makeRun(false, "version-check", "BASE="+prepareBase)
}
