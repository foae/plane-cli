package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These are consumer tests of the actual shell boundary, not publication mocks
// inside the implementation. Only gh and the unrelated Go version checker are
// replaced; Git uses a local annotated tag and a local bare origin.
func TestPublishBoundary(t *testing.T) {
	script, err := os.ReadFile("../../../scripts/publish-release.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"wrong-commit", "snapshot", "extra-asset", "different-bytes", "resume", "traversal", "wrong-input", "remote-tag", "dirty", "preview"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			root, artifacts, remote, bin := filepath.Join(base, "source"), filepath.Join(base, "assets"), filepath.Join(base, "remote-assets"), filepath.Join(base, "bin")
			for _, dir := range []string{root, artifacts, remote, bin} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, text string, mode os.FileMode) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(text), mode); err != nil {
					t.Fatal(err)
				}
			}
			git := func(args ...string) string {
				t.Helper()
				c := exec.Command("git", args...)
				c.Dir = root
				c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
				out, err := c.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			version := "1.2.3"
			if scenario == "preview" {
				version += "-rc.1"
			}
			write(filepath.Join(root, "VERSION"), version+"\n", 0644)
			inputs := []string{"api/catalog.json", "api/provenance.json", "go.mod", "go.sum", "tools/versions.mk"}
			inputHashes := map[string]string{}
			hash := func(data string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(data))) }
			for _, name := range inputs {
				write(filepath.Join(root, name), name, 0644)
				inputHashes[name] = hash(name)
			}
			git("init", "-q")
			git("config", "user.email", "fixture@example.invalid")
			git("config", "user.name", "Fixture")
			git("add", ".")
			git("commit", "-qm", "fixture")
			commit := git("rev-parse", "HEAD")
			git("tag", "-a", "v"+version, "-m", "release")
			origin := filepath.Join(base, "origin.git")
			git("clone", "--bare", root, origin)
			git("remote", "add", "origin", origin)
			targets := []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}
			names := []string{}
			archiveHashes := map[string]string{}
			for _, target := range targets {
				names = append(names, "plane-cli_"+version+"_"+strings.ReplaceAll(target, "/", "_")+".tar.gz")
			}
			names = append(names, "plane-cli-skill_"+version+".tar.gz")
			for _, name := range names {
				write(filepath.Join(artifacts, name), name, 0644)
				archiveHashes[name] = hash(name)
			}
			metadata := map[string]any{"version": version, "cli_version": version, "commit": commit, "source_dirty": false, "snapshot": false, "targets": targets, "input_sha256": inputHashes, "archive_sha256": archiveHashes}
			switch scenario {
			case "wrong-commit":
				metadata["commit"] = strings.Repeat("0", 40)
			case "snapshot":
				metadata["snapshot"] = true
			case "wrong-input":
				inputHashes[inputs[0]] = strings.Repeat("0", 64)
			case "remote-tag":
				git("tag", "-f", "-a", "v"+version, "-m", "changed")
			case "dirty":
				write(filepath.Join(root, "dirty"), "dirty", 0644)
			}
			data, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(artifacts, "release-metadata.json"), string(data), 0644)
			write(filepath.Join(artifacts, "plane-cli.rb"), "formula", 0644)
			names = append(names, "plane-cli.rb", "release-metadata.json")
			manifest := ""
			for _, name := range names {
				b, err := os.ReadFile(filepath.Join(artifacts, name))
				if err != nil {
					t.Fatal(err)
				}
				manifest += hash(string(b)) + "  " + name + "\n"
			}
			if scenario == "traversal" {
				manifest += strings.Repeat("0", 64) + "  ../outside\n"
			}
			write(filepath.Join(artifacts, "SHA256SUMS"), manifest, 0644)
			names = append(names, "SHA256SUMS")
			// The first asset already exists: valid resume must compare it, not upload it.
			write(filepath.Join(remote, names[0]), names[0], 0644)
			if scenario == "extra-asset" {
				write(filepath.Join(remote, "unexpected.txt"), "extra", 0644)
			}
			if scenario == "different-bytes" {
				write(filepath.Join(remote, names[0]), "different", 0644)
			}
			log := filepath.Join(base, "mutations")
			write(filepath.Join(bin, "go"), "#!/bin/sh\nexit 0\n", 0755)
			write(filepath.Join(bin, "gh"), `#!/usr/bin/env python3
import os, pathlib, shutil, sys
args = sys.argv[1:]
remote = pathlib.Path(os.environ['TEST_REMOTE'])
if args[0] == 'api':
    print('1')
elif args[:2] == ['release', 'view']:
    if '.assets[].name' in args:
        print('\n'.join(sorted(p.name for p in remote.iterdir())))
    else:
        print('true')
elif args[:2] == ['release', 'download']:
    name = args[args.index('--pattern') + 1]
    shutil.copyfile(remote / name, pathlib.Path(args[args.index('--dir') + 1]) / name)
elif args[:2] == ['release', 'upload']:
    with open(os.environ['TEST_LOG'], 'a') as log: log.write(' '.join(args) + '\n')
    shutil.copyfile(args[3], remote / pathlib.Path(args[3]).name)
elif args[:2] == ['release', 'edit']:
    with open(os.environ['TEST_LOG'], 'a') as log: log.write(' '.join(args) + '\n')
else:
    raise SystemExit('unexpected gh invocation: ' + repr(args))
`, 0755)
			scriptPath := filepath.Join(base, "publish-release.sh")
			write(scriptPath, string(script), 0755)
			cmd := exec.Command("sh", scriptPath, artifacts, "publish")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TEST_REMOTE="+remote, "TEST_LOG="+log, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
			output, runErr := cmd.CombinedOutput()
			mutations, _ := os.ReadFile(log)
			valid := scenario == "resume" || scenario == "preview"
			if !valid {
				if runErr == nil {
					t.Fatalf("accepted %s: %s", scenario, output)
				}
				if len(mutations) != 0 {
					t.Fatalf("mutated before rejecting %s: %s", scenario, mutations)
				}
				reasons := map[string]string{"wrong-commit": "metadata identity mismatch", "snapshot": "metadata is not a clean release", "extra-asset": "unexpected remote asset", "different-bytes": "remote asset differs", "traversal": "invalid checksum manifest entry", "wrong-input": "metadata input hashes mismatch", "remote-tag": "remote annotated tag differs", "dirty": "checkout is dirty"}
				if !strings.Contains(string(output), reasons[scenario]) {
					t.Fatalf("wrong rejection for %s: %s", scenario, output)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("valid resume: %v: %s", runErr, output)
			}
			lines := strings.Split(strings.TrimSpace(string(mutations)), "\n")
			if len(lines) != 8 {
				t.Fatalf("want seven uploads and one edit, got %s", mutations)
			}
			if strings.Contains(string(mutations), "upload v"+version+" "+names[0]+" ") {
				t.Fatalf("reuploaded existing asset: %s", mutations)
			}
			last := lines[len(lines)-1]
			if !strings.Contains(last, "--draft=false") || strings.Contains(last, "--latest=true") {
				t.Fatalf("unsafe edit: %s", last)
			}
			if (scenario == "preview") != strings.Contains(last, "--latest=false") {
				t.Fatalf("incorrect latest policy: %s", last)
			}
			for _, name := range names {
				got, err := os.ReadFile(filepath.Join(remote, name))
				if err != nil {
					t.Fatal(err)
				}
				want, err := os.ReadFile(filepath.Join(artifacts, name))
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != string(want) {
					t.Fatalf("remote differs: %s", name)
				}
			}
		})
	}
}
