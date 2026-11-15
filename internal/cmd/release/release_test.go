package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func put(t *testing.T, root, path, data string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func commitFixture(t *testing.T, root string) string {
	t.Helper()
	gitTest(t, root, "add", "--all")
	gitTest(t, root, "commit", "-qm", "fixture")
	return gitTest(t, root, "rev-parse", "HEAD")
}

func notesFixture(t *testing.T, root, v string, changes ...change) {
	t.Helper()
	if len(changes) == 0 {
		changes = []change{{Level: "patch", Summary: "Earlier release"}}
	}
	data, err := encodeJSON(releaseNotes{Version: v, Date: "2026-01-01", Changes: changes})
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "releases/"+v+".json", string(data))
}

func repoFixture(t *testing.T, v string) command {
	t.Helper()
	for _, key := range []string{"RELEASE_LEVEL", "RELEASE_SUMMARY", "RELEASE_VERSION", "RELEASE_BASE", "RELEASE_PREPARATION"} {
		t.Setenv(key, "")
	}
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "config", "user.name", "Release Fixture")
	gitTest(t, root, "config", "user.email", "release@example.invalid")
	gitTest(t, root, "config", "commit.gpgsign", "false")
	put(t, root, "VERSION", v+"\n")
	if v != initialVersion {
		notesFixture(t, root, v)
	}
	commitFixture(t, root)
	c := newCommand(root)
	c.now = func() time.Time { return time.Date(2026, 2, 2, 0, 15, 0, 0, time.FixedZone("east", 14*60*60)) }
	return c
}

func fragmentFixture(t *testing.T, c command, name, level, summary string) {
	t.Helper()
	data, err := encodeJSON(change{Level: level, Summary: summary})
	if err != nil {
		t.Fatal(err)
	}
	put(t, c.root, "changes/"+name+".json", string(data))
}

func versionFixture(t *testing.T, s string) version {
	t.Helper()
	v, err := parseVersion(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func stateFixture(t *testing.T, c command) state {
	t.Helper()
	s, err := readState(c.root)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".git" {
			return filepath.SkipDir
		}
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		} else if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += ":" + string(data)
		}
		result[filepath.ToSlash(rel)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertUnchanged(t *testing.T, c command, before map[string]string) {
	t.Helper()
	if after := snapshot(t, c.root); !reflect.DeepEqual(before, after) {
		t.Fatalf("worktree changed on failure/read-only operation\nbefore: %#v\nafter: %#v", before, after)
	}
	if _, err := os.Lstat(filepath.Join(c.root, ".git", "plane-cli-release.lock")); !os.IsNotExist(err) {
		t.Fatalf("release lock not cleared: %v", err)
	}
}

func runFixture(c command, args ...string) (string, error) {
	var output, stderr bytes.Buffer
	err := c.run(args, &output, &stderr)
	return output.String(), err
}

func TestCanonicalSemVerAndNumericOrdering(t *testing.T) {
	ordered := []string{"1.0.0-alpha.2", "1.0.0-alpha.10", "1.0.0-beta.0", "1.0.0-beta.10", "1.0.0-rc.0", "1.0.0-rc.10", "1.0.0", "1.0.2", "1.0.10", "1.9.0", "1.10.0", "2.0.0", "999999999999999999999999999999.0.0"}
	for i, name := range ordered {
		v := versionFixture(t, name)
		if v.String() != name || compareVersion(v, v) != 0 {
			t.Fatalf("version did not round trip: %s", name)
		}
		if i != 0 {
			previous := versionFixture(t, ordered[i-1])
			if compareVersion(previous, v) >= 0 || compareVersion(v, previous) <= 0 {
				t.Fatalf("wrong numerical ordering: %s / %s", previous.String(), name)
			}
		}
	}
	for _, invalid := range []string{"v1.0.0", "1.0", "01.0.0", "1.00.0", "1.0.00", "1.0.0-alpha.01", "1.0.0-alpha", "1.0.0-preview.1", "1.0.0-1", "1.0.0-alpha.1.extra", "1.0.0+build", "1.0.0-rc.1+build", "1.0.0\n", " 1.0.0"} {
		if _, err := parseVersion(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	if got := increment("999999999999999999999999999999"); got != "1000000000000000000000000000000" {
		t.Fatal(got)
	}
}

func TestPlanPrecedenceAndReadOnlyOrdering(t *testing.T) {
	for _, tc := range []struct {
		levels                 []string
		wantLevel, wantVersion string
	}{
		{[]string{"patch", "patch"}, "patch", "1.2.4"},
		{[]string{"patch", "minor", "patch"}, "minor", "1.3.0"},
		{[]string{"minor", "major", "patch"}, "major", "2.0.0"},
	} {
		t.Run(tc.wantLevel, func(t *testing.T) {
			c := repoFixture(t, "1.2.3")
			for i, level := range tc.levels {
				fragmentFixture(t, c, fmt.Sprintf("%02d", len(tc.levels)-i), level, level+" note")
			}
			before := snapshot(t, c.root)
			output, err := runFixture(c, "version-plan")
			if err != nil {
				t.Fatal(err)
			}
			var p plan
			if err := json.Unmarshal([]byte(output), &p); err != nil {
				t.Fatal(err)
			}
			if p.RequiredBump != tc.wantLevel || p.MinimumVersion != tc.wantVersion || len(p.Changes) != len(tc.levels) {
				t.Fatalf("unexpected consumer plan: %s", output)
			}
			for i, note := range p.Changes {
				if note.Level != tc.levels[len(tc.levels)-1-i] {
					t.Fatalf("notes not ordered by fragment path: %s", output)
				}
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestPreviewPolicyAndPromotion(t *testing.T) {
	for _, tc := range []struct {
		current, target string
		valid           bool
	}{
		{"1.0.0-alpha.9", "1.0.0-alpha.10", true},
		{"1.0.0-alpha.10", "1.0.0-beta.0", true},
		{"1.0.0-beta.10", "1.0.0-rc.0", true},
		{"1.0.0-rc.10", "1.0.0", true},
		{"1.0.0-beta.0", "1.0.0-alpha.999", false},
		{"1.0.0-alpha.10", "1.0.0-alpha.2", false},
		{"1.0.0-rc.1", "1.0.0-rc.1", false},
		{initialVersion, "1.1.0-alpha.0", false},
		{initialVersion, "2.0.0", false},
	} {
		t.Run(tc.current+"_to_"+tc.target, func(t *testing.T) {
			c := repoFixture(t, tc.current)
			fragmentFixture(t, c, "impact", "major", "Preview breaking change")
			s := stateFixture(t, c)
			if err := s.validateTarget(versionFixture(t, tc.target)); (err == nil) != tc.valid {
				t.Fatalf("target validity %v: %v", tc.valid, err)
			}
			if s.makePlan().minimumCore.core() != "1.0.0" {
				t.Fatal("preview changed first stable target")
			}
		})
	}
}

func TestStableBaselineAccumulatesPreviewImpacts(t *testing.T) {
	c := repoFixture(t, "1.1.0-beta.10")
	notesFixture(t, c.root, "1.0.0")
	notesFixture(t, c.root, "1.1.0-alpha.0", change{Level: "minor", Summary: "Add feature"})
	fragmentFixture(t, c, "patch", "patch", "Fix preview")
	s := stateFixture(t, c)
	p := s.makePlan()
	if p.RequiredBump != "minor" || p.MinimumVersion != "1.1.0-beta.11" {
		t.Fatalf("preview impacts not accumulated: %+v", p)
	}
	for _, target := range []string{"1.1.0-rc.0", "1.1.0"} {
		if err := s.validateTarget(versionFixture(t, target)); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"1.0.1", "1.1.0-alpha.99"} {
		if err := s.validateTarget(versionFixture(t, target)); err == nil {
			t.Fatal("allowed reduction", target)
		}
	}
	fragmentFixture(t, c, "breaking", "major", "Break consumer contract")
	s = stateFixture(t, c)
	if s.makePlan().minimumCore.core() != "2.0.0" {
		t.Fatal("new major impact did not increase required core")
	}
	if err := s.validateTarget(versionFixture(t, "1.1.0-rc.0")); err == nil {
		t.Fatal("prerelease suffix bypassed major bump")
	}
	if err := s.validateTarget(versionFixture(t, "2.0.0-alpha.0")); err != nil {
		t.Fatal(err)
	}
}

func TestUnchangedPreviewPromotionLineage(t *testing.T) {
	c := repoFixture(t, "1.0.0")
	fragmentFixture(t, c, "feature", "minor", "Add feature")
	commitFixture(t, c.root)
	if err := c.prepare(versionFixture(t, "1.1.0-alpha.0")); err != nil {
		t.Fatal(err)
	}
	s := stateFixture(t, c)
	if notes := s.history[s.current.String()]; len(notes.Changes) != 1 || notes.PromotedFrom != "" {
		t.Fatalf("fragment preparation must retain delta notes: %+v", notes)
	}
	for _, target := range []string{"1.1.0-beta.0", "1.1.0-rc.0", "1.1.0"} {
		commitFixture(t, c.root)
		s = stateFixture(t, c)
		previous := s.current.String()
		p := s.makePlan()
		if p.RequiredBump != "minor" || p.minimumCore.core() != "1.1.0" || len(p.Changes) != 0 {
			t.Fatalf("original preview impact lost: %+v", p)
		}
		if err := c.prepare(versionFixture(t, target)); err != nil {
			t.Fatal(err)
		}
		s = stateFixture(t, c)
		notes := s.history[target]
		if notes.PromotedFrom != previous || notes.Changes == nil || len(notes.Changes) != 0 || len(s.fragments) != 0 {
			t.Fatalf("promotion must reference exact prior preview without invented changes: %+v", notes)
		}
		data, err := os.ReadFile(filepath.Join(c.root, "releases", target+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(`"changes": []`)) {
			t.Fatalf("promotion changes must encode as an empty array: %s", data)
		}
	}
	if p := s.makePlan(); p.RequiredBump != "none" || p.MinimumVersion != "1.1.0" {
		t.Fatalf("stable promotion did not reset baseline: %+v", p)
	}
}

func TestInvalidPromotionLineage(t *testing.T) {
	for _, tc := range []struct {
		name, source, target string
		changes              []change
		missing              bool
	}{
		{name: "missing-source", source: "1.0.0-alpha.1", target: "1.0.0-beta.0", missing: true},
		{name: "malformed-source", source: "invalid", target: "1.0.0-beta.0", missing: true},
		{name: "different-core", source: "1.0.0-alpha.1", target: "1.1.0-beta.0"},
		{name: "same-channel", source: "1.0.0-beta.0", target: "1.0.0-beta.1"},
		{name: "reverse-channel", source: "1.0.0-rc.0", target: "1.0.0-beta.0"},
		{name: "stable-source", source: "1.0.0", target: "1.1.0"},
		{name: "self-reference", source: "1.0.0-beta.0", target: "1.0.0-beta.0", missing: true},
		{name: "mixed-changes", source: "1.0.0-alpha.1", target: "1.0.0-beta.0", changes: []change{{Level: "patch", Summary: "Mixed delta"}}},
		{name: "empty-without-lineage", target: "1.0.0-beta.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := repoFixture(t, tc.target)
			if tc.source != "" && !tc.missing {
				notesFixture(t, c.root, tc.source)
			}
			data, err := encodeJSON(releaseNotes{
				Version: tc.target, Date: "2026-01-01",
				Changes: tc.changes, PromotedFrom: tc.source,
			})
			if err != nil {
				t.Fatal(err)
			}
			put(t, c.root, "releases/"+tc.target+".json", string(data))
			if _, err := readState(c.root); err == nil {
				t.Fatal("invalid promotion lineage accepted")
			}
		})
	}
}

func TestInvalidNoChangePreparationNeverMutates(t *testing.T) {
	for _, tc := range []struct {
		current, target string
	}{
		{initialVersion, "1.0.0-beta.0"},
		{"1.0.0", "1.0.1"},
		{"1.0.0-alpha.1", "1.0.0-alpha.2"},
		{"1.0.0-beta.0", "1.0.0-alpha.9"},
		{"1.0.0-rc.0", "1.1.0"},
	} {
		t.Run(tc.current+"_to_"+tc.target, func(t *testing.T) {
			c := repoFixture(t, tc.current)
			before := snapshot(t, c.root)
			if err := c.prepare(versionFixture(t, tc.target)); err == nil {
				t.Fatal("invalid no-change preparation succeeded")
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestPrepareConsumesInOrderWithUTCDateAndNoGitWrites(t *testing.T) {
	c := repoFixture(t, initialVersion)
	fragmentFixture(t, c, "z-last", "patch", "Last note")
	fragmentFixture(t, c, "a-first", "minor", "First note")
	head := commitFixture(t, c.root)
	if _, err := runFixture(c, "release-prepare", "--version", "1.0.0-alpha.1"); err != nil {
		t.Fatal(err)
	}
	s := stateFixture(t, c)
	notes := s.history["1.0.0-alpha.1"]
	if s.current.String() != "1.0.0-alpha.1" || len(s.fragments) != 0 || notes.Date != "2026-02-01" {
		t.Fatalf("incorrect prepared state: %+v / %+v", s, notes)
	}
	if !reflect.DeepEqual(notes.Changes, []change{{Level: "minor", Summary: "First note"}, {Level: "patch", Summary: "Last note"}}) {
		t.Fatalf("incorrect notes: %+v", notes)
	}
	if gitTest(t, c.root, "rev-parse", "HEAD") != head || gitTest(t, c.root, "tag", "--list") != "" {
		t.Fatal("preparation wrote Git history or a tag")
	}
	commitFixture(t, c.root)
	before := snapshot(t, c.root)
	if _, err := runFixture(c, "release-prepare", "--version", "1.0.0-alpha.1"); err == nil {
		t.Fatal("repeated preparation succeeded")
	}
	assertUnchanged(t, c, before)
}

func TestPreparationValidationNeverMutates(t *testing.T) {
	for _, scenario := range []string{"insufficient", "prerelease-insufficient", "tag", "untracked", "tracked", "invalid-fragment", "future-history", "missing-notes"} {
		t.Run(scenario, func(t *testing.T) {
			c := repoFixture(t, "1.2.3")
			fragmentFixture(t, c, "impact", "minor", "New consumer feature")
			commitFixture(t, c.root)
			target := "1.3.0"
			switch scenario {
			case "insufficient":
				target = "1.2.4"
			case "prerelease-insufficient":
				target = "1.2.4-rc.99"
			case "tag":
				gitTest(t, c.root, "tag", "v"+target)
			case "untracked":
				put(t, c.root, "unrelated.txt", "untracked")
			case "tracked":
				put(t, c.root, "VERSION", "1.2.3")
			case "invalid-fragment":
				put(t, c.root, "changes/impact.json", `{"level":"minor","summary":""}`)
				commitFixture(t, c.root)
			case "future-history":
				notesFixture(t, c.root, "2.0.0")
				commitFixture(t, c.root)
			case "missing-notes":
				if err := os.Remove(filepath.Join(c.root, "releases/1.2.3.json")); err != nil {
					t.Fatal(err)
				}
				commitFixture(t, c.root)
			}
			before := snapshot(t, c.root)
			if err := c.prepare(versionFixture(t, target)); err == nil {
				t.Fatal("invalid preparation succeeded")
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestRollbackRestoresConsumedFragmentsAndVersionIsLast(t *testing.T) {
	for _, failure := range []string{"second-fragment", "version-rename"} {
		t.Run(failure, func(t *testing.T) {
			c := repoFixture(t, initialVersion)
			fragmentFixture(t, c, "a", "patch", "First")
			fragmentFixture(t, c, "b", "minor", "Second")
			commitFixture(t, c.root)
			before := snapshot(t, c.root)
			removed := 0
			c.remove = func(path string) error {
				if strings.Contains(path, string(filepath.Separator)+"changes"+string(filepath.Separator)) {
					if data, err := os.ReadFile(filepath.Join(c.root, "VERSION")); err != nil || string(data) != initialVersion+"\n" {
						t.Fatalf("VERSION changed before fragment consumption: %s / %v", data, err)
					}
					if _, err := os.Stat(filepath.Join(c.root, "releases/1.0.0-alpha.1.json")); err != nil {
						t.Fatal("notes were not installed before fragment consumption", err)
					}
					removed++
					if failure == "second-fragment" && removed == 2 {
						return errors.New("injected fragment removal failure")
					}
				}
				return os.Remove(path)
			}
			c.rename = func(from, to string) error {
				if removed != 2 {
					t.Fatal("VERSION replacement ran before consuming all fragments")
				}
				for _, name := range []string{"a", "b"} {
					if _, err := os.Stat(filepath.Join(c.root, "changes", name+".json")); !os.IsNotExist(err) {
						t.Fatalf("fragment remains before VERSION replacement: %s", name)
					}
				}
				if failure != "version-rename" {
					t.Fatal("unexpected VERSION replacement")
				}
				return errors.New("injected atomic VERSION replacement failure")
			}
			if err := c.prepare(versionFixture(t, "1.0.0-alpha.1")); err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("expected injected failure: %v", err)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestRollbackFailureIsExplicitAndRetainsRecoveryBytes(t *testing.T) {
	c := repoFixture(t, initialVersion)
	fragmentFixture(t, c, "a", "patch", "Keep recoverable")
	commitFixture(t, c.root)
	c.rename = func(string, string) error { return errors.New("injected rename failure") }
	c.remove = func(path string) error {
		if strings.Contains(path, string(filepath.Separator)+"releases"+string(filepath.Separator)) {
			return errors.New("injected rollback removal failure")
		}
		return os.Remove(path)
	}
	err := c.prepare(versionFixture(t, "1.0.0-alpha.1"))
	if err == nil || !strings.Contains(err.Error(), "rollback failed") || !strings.Contains(err.Error(), "recovery backups retained at") {
		t.Fatalf("rollback failure was hidden: %v", err)
	}
	stages, err := filepath.Glob(filepath.Join(c.root, ".plane-cli-release-stage-*"))
	if err != nil || len(stages) != 1 {
		t.Fatalf("recovery stage missing: %v / %v", stages, err)
	}
	original, err := os.ReadFile(filepath.Join(c.root, "changes/a.json"))
	if err != nil {
		t.Fatal("fragment not restored", err)
	}
	backup, err := os.ReadFile(filepath.Join(stages[0], "fragment-0.json"))
	if err != nil || !bytes.Equal(original, backup) {
		t.Fatal("recovery bytes differ", err)
	}
	if _, err := os.Lstat(filepath.Join(c.root, ".git/plane-cli-release.lock")); !os.IsNotExist(err) {
		t.Fatal("lock retained after rollback failure", err)
	}
}

func TestRejectManagedSymlinksAndNonregularPaths(t *testing.T) {
	for _, path := range []string{"VERSION", "changes", "releases", "changes/a.json", "releases/1.0.0-alpha.1.json"} {
		t.Run(path, func(t *testing.T) {
			c := repoFixture(t, "1.0.0-alpha.1")
			fragmentFixture(t, c, "a", "patch", "Pending")
			full := filepath.Join(c.root, path)
			if err := os.Rename(full, full+".real"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(full)+".real", full); err != nil {
				t.Fatal(err)
			}
			commitFixture(t, c.root)
			before := snapshot(t, c.root)
			if _, err := readState(c.root); err == nil {
				t.Fatal("accepted managed symlink")
			}
			if err := c.prepare(versionFixture(t, "1.0.0-alpha.2")); err == nil {
				t.Fatal("prepared through managed symlink")
			}
			assertUnchanged(t, c, before)
		})
	}
	c := repoFixture(t, initialVersion)
	if err := os.MkdirAll(filepath.Join(c.root, "changes/not-a-file.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(c.root); err == nil {
		t.Fatal("accepted directory as fragment")
	}
}

func TestChangeValidationAndExclusiveLock(t *testing.T) {
	c := repoFixture(t, initialVersion)
	for _, note := range []change{{"fix", "Wrong level"}, {"patch", ""}, {"patch", " \t "}, {"minor", "one\ntwo"}, {"major", "one\rtwo"}, {"patch", "one\u2028two"}} {
		before := snapshot(t, c.root)
		if _, err := c.createChange(note); err == nil {
			t.Fatalf("accepted invalid note: %+v", note)
		}
		assertUnchanged(t, c, before)
	}
	locked := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- c.withLock(func(string) error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	before := snapshot(t, c.root)
	_, changeErr := c.createChange(change{Level: "patch", Summary: "Concurrent writer"})
	prepareErr := c.prepare(versionFixture(t, "1.0.0-alpha.1"))
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if changeErr == nil || prepareErr == nil || !strings.Contains(changeErr.Error(), "acquire release lock") || !strings.Contains(prepareErr.Error(), "acquire release lock") {
		t.Fatalf("concurrent writer bypassed lock: %v / %v", changeErr, prepareErr)
	}
	assertUnchanged(t, c, before)
}

func TestConcurrentFragmentNamesAreExclusiveAndComplete(t *testing.T) {
	c := repoFixture(t, initialVersion)
	const writers = 24
	type result struct {
		path string
		err  error
	}
	results := make(chan result, writers)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := range writers {
		workers.Go(func() {
			<-start
			path, err := c.createChange(change{Level: "patch", Summary: fmt.Sprintf("Writer %d", i)})
			results <- result{path: path, err: err}
		})
	}
	close(start)
	workers.Wait()
	close(results)
	paths := make(map[string]bool)
	for result := range results {
		if result.err != nil {
			if !strings.Contains(result.err.Error(), "acquire release lock") {
				t.Fatal(result.err)
			}
			continue
		}
		if paths[result.path] {
			t.Fatal("fragment name collision", result.path)
		}
		paths[result.path] = true
	}
	s := stateFixture(t, c)
	if len(paths) == 0 || len(s.fragments) != len(paths) {
		t.Fatalf("partial or lost concurrent fragments: paths=%d fragments=%d", len(paths), len(s.fragments))
	}
	for _, f := range s.fragments {
		if !paths[f.path] || !strings.HasPrefix(f.note.Summary, "Writer ") {
			t.Fatalf("invalid concurrent fragment: %+v", f)
		}
	}
}
