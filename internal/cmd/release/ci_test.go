package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRequiresNewTrackedValidFragment(t *testing.T) {
	for _, scenario := range []string{"source-only", "untracked-fragment", "modified-fragment", "new-fragment", "empty", "invalid-new-fragment"} {
		t.Run(scenario, func(t *testing.T) {
			c := repoFixture(t, initialVersion)
			fragmentFixture(t, c, "existing", "patch", "Existing work item")
			base := commitFixture(t, c.root)
			if scenario != "empty" {
				put(t, c.root, "source.go", "package fixture\n")
			}
			switch scenario {
			case "untracked-fragment":
				commitFixture(t, c.root)
				fragmentFixture(t, c, "new", "minor", "Untracked impact")
			case "modified-fragment":
				fragmentFixture(t, c, "existing", "major", "Modified existing impact")
				commitFixture(t, c.root)
			case "new-fragment":
				fragmentFixture(t, c, "new", "minor", "Reviewed new work item")
				commitFixture(t, c.root)
			case "invalid-new-fragment":
				put(t, c.root, "changes/new.json", `{"level":"patch","summary":""}`)
				commitFixture(t, c.root)
			case "source-only":
				commitFixture(t, c.root)
			}
			before := snapshot(t, c.root)
			_, err := runFixture(c, "version-check", "--base", base)
			wantValid := scenario == "empty" || scenario == "new-fragment"
			if (err == nil) != wantValid {
				t.Fatalf("valid=%v: %v", wantValid, err)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestCIOrdinaryChangesKeepReleaseInputsImmutable(t *testing.T) {
	for _, scenario := range []string{"new-fragment", "delete-major", "downgrade-major", "rewrite-history", "remove-history", "add-history", "change-version", "remove-version"} {
		t.Run(scenario, func(t *testing.T) {
			c := repoFixture(t, "1.2.3")
			fragmentFixture(t, c, "breaking", "major", "Break reviewed interface")
			notesFixture(t, c.root, "1.2.2")
			base := commitFixture(t, c.root)
			fragmentFixture(t, c, "new", "patch", "New reviewed fix")
			wantError := ""
			switch scenario {
			case "delete-major":
				if err := os.Remove(filepath.Join(c.root, "changes/breaking.json")); err != nil {
					t.Fatal(err)
				}
				wantError = "existing release fragment"
			case "downgrade-major":
				fragmentFixture(t, c, "breaking", "patch", "Downgrade reviewed interface impact")
				wantError = "existing release fragment"
			case "rewrite-history":
				notesFixture(t, c.root, "1.2.2", change{Level: "patch", Summary: "Rewrite historical release"})
				wantError = "release history"
			case "remove-history":
				if err := os.Remove(filepath.Join(c.root, "releases/1.2.2.json")); err != nil {
					t.Fatal(err)
				}
				wantError = "release history"
			case "add-history":
				notesFixture(t, c.root, "1.2.1")
				wantError = "release history"
			case "change-version":
				put(t, c.root, "VERSION", "1.2.4\n")
				notesFixture(t, c.root, "1.2.4")
				wantError = "VERSION"
			case "remove-version":
				if err := os.Remove(filepath.Join(c.root, "VERSION")); err != nil {
					t.Fatal(err)
				}
				wantError = "VERSION"
			}
			commitFixture(t, c.root)
			before := snapshot(t, c.root)
			_, err := runFixture(c, "version-check", "--base", base)
			if scenario == "new-fragment" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("expected %q rejection alongside valid patch fragment: %v", wantError, err)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestCIOrdinaryChangesAllowBootstrapVersionAddition(t *testing.T) {
	c := repoFixture(t, initialVersion)
	if err := os.Remove(filepath.Join(c.root, "VERSION")); err != nil {
		t.Fatal(err)
	}
	base := commitFixture(t, c.root)
	put(t, c.root, "VERSION", initialVersion+"\n")
	fragmentFixture(t, c, "bootstrap", "patch", "Introduce release tracking")
	commitFixture(t, c.root)
	before := snapshot(t, c.root)
	if _, err := runFixture(c, "version-check", "--base", base); err != nil {
		t.Fatal("initial VERSION addition must remain valid", err)
	}
	assertUnchanged(t, c, before)
}

func TestCIReleasePreparationVerifiesActualConsumption(t *testing.T) {
	for _, scenario := range []string{"valid", "no-mode", "unrelated-source", "new-fragment", "altered-notes", "reordered-notes", "modified-history", "no-consumption"} {
		t.Run(scenario, func(t *testing.T) {
			c := repoFixture(t, "1.0.0-alpha.1")
			fragmentFixture(t, c, "a", "minor", "Add reviewed feature")
			fragmentFixture(t, c, "b", "patch", "Fix reviewed behavior")
			base := commitFixture(t, c.root)
			if scenario != "no-consumption" {
				if err := c.prepare(versionFixture(t, "1.0.0-beta.0")); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "unrelated-source":
				put(t, c.root, "unrelated.go", "package unrelated\n")
			case "new-fragment":
				fragmentFixture(t, c, "recursive", "patch", "Release consumption")
			case "altered-notes":
				notesFixture(t, c.root, "1.0.0-beta.0", change{Level: "minor", Summary: "Unreviewed feature"}, change{Level: "patch", Summary: "Fix reviewed behavior"})
			case "reordered-notes":
				notesFixture(t, c.root, "1.0.0-beta.0", change{Level: "patch", Summary: "Fix reviewed behavior"}, change{Level: "minor", Summary: "Add reviewed feature"})
			case "modified-history":
				notesFixture(t, c.root, "1.0.0-alpha.1", change{Level: "patch", Summary: "Rewrite historical release"})
			}
			if scenario != "no-consumption" {
				commitFixture(t, c.root)
			}
			args := []string{"version-check", "--base", base}
			if scenario != "no-mode" {
				args = append(args, "--release-preparation")
			}
			before := snapshot(t, c.root)
			_, err := runFixture(c, args...)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("release-preparation valid=%v: %v", scenario == "valid", err)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestCIReleasePreparationVerifiesPromotionLineage(t *testing.T) {
	for _, scenario := range []string{"valid", "no-mode", "wrong-source", "ordinary-notes", "new-fragment", "modified-history", "unrelated-source", "untracked-notes"} {
		t.Run(scenario, func(t *testing.T) {
			c := repoFixture(t, "1.0.0-beta.1")
			notesFixture(t, c.root, "1.0.0-alpha.1")
			base := commitFixture(t, c.root)
			if err := c.prepare(versionFixture(t, "1.0.0-rc.0")); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "wrong-source":
				data, err := encodeJSON(releaseNotes{Version: "1.0.0-rc.0", Date: "2026-02-01", PromotedFrom: "1.0.0-alpha.1"})
				if err != nil {
					t.Fatal(err)
				}
				put(t, c.root, "releases/1.0.0-rc.0.json", string(data))
			case "ordinary-notes":
				notesFixture(t, c.root, "1.0.0-rc.0", change{Level: "patch", Summary: "Invent unreviewed promotion changes"})
			case "new-fragment":
				fragmentFixture(t, c, "recursive", "patch", "Promotion is not fragment consumption")
			case "modified-history":
				notesFixture(t, c.root, "1.0.0-beta.1", change{Level: "patch", Summary: "Rewrite promotion source"})
			case "unrelated-source":
				put(t, c.root, "unrelated.go", "package unrelated\n")
			}
			if scenario != "untracked-notes" {
				commitFixture(t, c.root)
			}
			args := []string{"version-check", "--base", base}
			if scenario != "no-mode" {
				args = append(args, "--release-preparation")
			}
			before := snapshot(t, c.root)
			_, err := runFixture(c, args...)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("promotion valid=%v: %v", scenario == "valid", err)
			}
			if scenario == "wrong-source" && !strings.Contains(err.Error(), "exact previous version 1.0.0-beta.1") {
				t.Fatalf("promotion must identify the actual base version: %v", err)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestCIReleasePreparationRejectsInsufficientCore(t *testing.T) {
	c := repoFixture(t, "1.2.3")
	fragmentFixture(t, c, "breaking", "major", "Break public interface")
	base := commitFixture(t, c.root)
	put(t, c.root, "VERSION", "1.3.0-rc.0\n")
	notesFixture(t, c.root, "1.3.0-rc.0", change{Level: "major", Summary: "Break public interface"})
	if err := os.Remove(filepath.Join(c.root, "changes/breaking.json")); err != nil {
		t.Fatal(err)
	}
	commitFixture(t, c.root)
	before := snapshot(t, c.root)
	if _, err := runFixture(c, "version-check", "--base", base, "--release-preparation"); err == nil || !strings.Contains(err.Error(), "2.0.0") {
		t.Fatalf("CI accepted insufficient core: %v", err)
	}
	assertUnchanged(t, c, before)
}

func TestVersionCheckInitialMetadataTagAndDirtyMismatch(t *testing.T) {
	c := repoFixture(t, initialVersion)
	if _, err := runFixture(c, "version-check"); err != nil {
		t.Fatal("initial preview should not require invented release notes", err)
	}
	if _, err := runFixture(c, "version-check", "--release-preparation"); err == nil {
		t.Fatal("mode alone bypassed base check")
	}
	metadata := filepath.Join(t.TempDir(), "metadata.json")
	put(t, filepath.Dir(metadata), filepath.Base(metadata), `{"skill_version":"1.0.0-alpha.0","cli_version":"1.0.0-alpha.0"}`)
	if _, err := runFixture(c, "version-check", "--skill-metadata", metadata); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{"skill_version":"1.0.0-alpha.0","cli_version":"1.0.0"}`, `{"skill_version":"1.0.0-alpha.0"}`, `{not JSON}`} {
		put(t, filepath.Dir(metadata), filepath.Base(metadata), data)
		if _, err := runFixture(c, "version-check", "--skill-metadata", metadata); err == nil {
			t.Fatal("accepted mismatching/malformed packaged skill metadata", data)
		}
	}
	gitTest(t, c.root, "tag", "v"+initialVersion)
	if _, err := runFixture(c, "version-check", "--tag", "v"+initialVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := runFixture(c, "version-check", "--tag", "v1.0.0"); err == nil {
		t.Fatal("accepted tag/version mismatch")
	}
	put(t, c.root, "untracked.txt", "untracked")
	if _, err := runFixture(c, "version-check", "--tag", "v"+initialVersion); err == nil {
		t.Fatal("tag identity accepted a dirty tree")
	}
	commitFixture(t, c.root)
	if _, err := runFixture(c, "version-check", "--tag", "v"+initialVersion); err == nil || !strings.Contains(err.Error(), "does not identify HEAD") {
		t.Fatalf("accepted tag identifying a different commit: %v", err)
	}
}

func TestAllHistoryAndFragmentInputsAreValidated(t *testing.T) {
	for _, tc := range []struct {
		path, data string
	}{
		{"VERSION", "v1.0.0-alpha.0\n"},
		{"VERSION", "1.0.0-alpha.0\n\n"},
		{"changes/a.json", `{"level":"patch","summary":"line\nline"}`},
		{"changes/a.json", `{"level":"patch","summary":"Valid","unexpected":true}`},
		{"changes/a.json", `{"level":"patch","summary":"Valid"} {}`},
		{"releases/1.0.0-alpha.0.json", `{"version":"1.0.0-alpha.0","date":"2026-02-30","changes":[{"level":"patch","summary":"Invalid date"}]}`},
		{"releases/1.0.0-alpha.0.json", `{"version":"1.0.0-alpha.0","date":"2026-01-01","changes":[]}`},
		{"releases/duplicate.json", `{"version":"1.0.0-alpha.0","date":"2026-01-01","changes":[{"level":"patch","summary":"Duplicate identity"}]}`},
		{"releases/v1.0.0-alpha.0.json", `{"version":"v1.0.0-alpha.0","date":"2026-01-01","changes":[{"level":"patch","summary":"Invalid historical version"}]}`},
		{"releases/1.0.0-alpha.1.json", `{"version":"1.0.0-alpha.1","date":"2026-01-01","changes":[{"level":"patch","summary":"Future release"}]}`},
	} {
		t.Run(tc.path+tc.data, func(t *testing.T) {
			c := repoFixture(t, initialVersion)
			put(t, c.root, tc.path, tc.data)
			before := snapshot(t, c.root)
			if _, err := runFixture(c, "version-plan"); err == nil {
				t.Fatalf("accepted invalid managed data: %s", tc.data)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestEnvironmentFallbackAndExplicitFlagOverride(t *testing.T) {
	c := repoFixture(t, initialVersion)
	t.Setenv("RELEASE_LEVEL", "major")
	t.Setenv("RELEASE_SUMMARY", "Environment note with 'quotes' and $shell; text")
	if _, err := runFixture(c, "change", "--level", "patch"); err != nil {
		t.Fatal(err)
	}
	s := stateFixture(t, c)
	if len(s.fragments) != 1 || s.fragments[0].note.Level != "patch" || s.fragments[0].note.Summary != os.Getenv("RELEASE_SUMMARY") {
		t.Fatalf("flags/environment changed consumer text: %+v", s.fragments)
	}
	commitFixture(t, c.root)
	t.Setenv("RELEASE_VERSION", "1.0.0-alpha.1")
	if _, err := runFixture(c, "release-prepare"); err != nil {
		t.Fatal(err)
	}
	commitFixture(t, c.root)
	t.Setenv("RELEASE_BASE", "HEAD")
	t.Setenv("RELEASE_PREPARATION", "false")
	if _, err := runFixture(c, "version-check"); err != nil {
		t.Fatal(err)
	}
	if _, err := runFixture(c, "version-check", "--release-preparation"); err == nil {
		t.Fatal("explicit mode did not override environment")
	}
}

func TestChangeSupportsLinkedWorktreeGitDirectory(t *testing.T) {
	c := repoFixture(t, initialVersion)
	linked := filepath.Join(t.TempDir(), "linked")
	gitTest(t, c.root, "worktree", "add", "--detach", "-q", linked, "HEAD")
	other := newCommand(linked)
	path, err := other.createChange(change{Level: "patch", Summary: "Linked worktree release note"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(linked, path)); err != nil {
		t.Fatal(err)
	}
	dir, err := other.gitDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "plane-cli-release.lock")); !os.IsNotExist(err) {
		t.Fatal("linked worktree release lock retained", err)
	}
}

func TestPrepareRequiresTrackedInputsButAllowsIgnoredBuildOutputs(t *testing.T) {
	for _, scenario := range []string{"ignored-fragment", "ignored-build-output"} {
		t.Run(scenario, func(t *testing.T) {
			c := repoFixture(t, initialVersion)
			put(t, c.root, ".gitignore", "changes/ignored.json\nbin/\n")
			fragmentFixture(t, c, "reviewed", "patch", "Reviewed change")
			commitFixture(t, c.root)
			if scenario == "ignored-fragment" {
				fragmentFixture(t, c, "ignored", "major", "Unreviewed ignored change")
			} else {
				put(t, c.root, "bin/plane-cli", "ignored build output")
			}
			if status := gitTest(t, c.root, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
				t.Fatal("fixture must be clean by ordinary Git status", status)
			}
			before := snapshot(t, c.root)
			err := c.prepare(versionFixture(t, "1.0.0-alpha.1"))
			if scenario == "ignored-fragment" {
				if err == nil || !strings.Contains(err.Error(), "all release inputs must be tracked") {
					t.Fatalf("accepted ignored release input: %v", err)
				}
				assertUnchanged(t, c, before)
			} else {
				if err != nil {
					t.Fatal("ignored build output incorrectly blocks preparation", err)
				}
				data, err := os.ReadFile(filepath.Join(c.root, "bin/plane-cli"))
				if err != nil || string(data) != "ignored build output" {
					t.Fatal("preparation changed ignored build output", err)
				}
			}
		})
	}
}
