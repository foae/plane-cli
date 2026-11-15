package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestArtifacts consumes the actual staged archives, not rebuilt test binaries.
func TestArtifacts(t *testing.T) {
	dir := os.Getenv("PLANE_CLI_ARTIFACT_DIR")
	if dir == "" {
		t.Skip("set PLANE_CLI_ARTIFACT_DIR to exercise packaged artifacts")
	}
	root := filepath.Join("..", "..", "..")
	versionFile, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(versionFile))
	notices, err := os.ReadFile(filepath.Join(root, "THIRD_PARTY_NOTICES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Version       string            `json:"version"`
		CLIVersion    string            `json:"cli_version"`
		Commit        string            `json:"commit"`
		SourceDirty   *bool             `json:"source_dirty"`
		Snapshot      *bool             `json:"snapshot"`
		InputSHA256   map[string]string `json:"input_sha256"`
		ArchiveSHA256 map[string]string `json:"archive_sha256"`
	}
	metadataFile, err := os.ReadFile(filepath.Join(dir, "release-metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadataFile, &metadata); err != nil {
		t.Fatal(err)
	}
	head, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Version != version || metadata.Commit != strings.TrimSpace(string(head)) || metadata.SourceDirty == nil || metadata.Snapshot == nil {
		t.Fatalf("metadata identity does not match source: %s", metadataFile)
	}
	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if *metadata.SourceDirty != (len(bytes.TrimSpace(status)) != 0) {
		t.Fatal("metadata source_dirty differs from current source")
	}
	cliVersion := version
	if *metadata.Snapshot {
		cliVersion = "dev"
	} else if *metadata.SourceDirty {
		t.Fatal("release metadata reports dirty source")
	}
	if metadata.CLIVersion != cliVersion {
		t.Fatalf("metadata CLI version %q, expected %q", metadata.CLIVersion, cliVersion)
	}
	inputs := []string{"api/catalog.json", "api/provenance.json", "go.mod", "go.sum", "tools/versions.mk"}
	if len(metadata.InputSHA256) != len(inputs) {
		t.Fatalf("unexpected metadata input hashes: %v", metadata.InputSHA256)
	}
	for _, name := range inputs {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if metadata.InputSHA256[name] != hex.EncodeToString(sum[:]) {
			t.Fatalf("metadata input checksum mismatch: %s", name)
		}
	}
	targets := []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"}
	expected := map[string]bool{
		"plane-cli-skill_" + version + ".tar.gz": true,
		"plane-cli.rb":                           true,
		"release-metadata.json":                  true,
	}
	for _, target := range targets {
		expected["plane-cli_"+version+"_"+target+".tar.gz"] = true
	}
	if len(metadata.ArchiveSHA256) != 5 {
		t.Fatalf("expected only five archive hashes in metadata, got %v", metadata.ArchiveSHA256)
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("bad checksum line %q", line)
		}
		name := fields[1]
		if filepath.Base(name) != name || seen[name] != "" || !expected[name] {
			t.Fatalf("unexpected/duplicate manifest member %q", name)
		}
		seen[name] = fields[0]
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != fields[0] {
			t.Fatalf("checksum mismatch: %s", name)
		}
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		if metadata.ArchiveSHA256[name] != fields[0] {
			t.Fatalf("metadata archive checksum mismatch: %s", name)
		}
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		tr := tar.NewReader(gz)
		members := map[string][]byte{}
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if h.Typeflag != tar.TypeReg || strings.Contains(h.Name, "..") || strings.HasPrefix(h.Name, "/") {
				t.Fatalf("unsafe archive entry %s", h.Name)
			}
			if _, ok := members[h.Name]; ok {
				t.Fatalf("duplicate entry %s", h.Name)
			}
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			members[h.Name] = b
			if h.Name == "plane-cli" && h.Mode != 0755 {
				t.Fatalf("binary mode %o", h.Mode)
			}
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(name, "plane-cli-skill_") {
			for _, required := range []string{"plane-cli/LICENSE", "plane-cli/SKILL.md", "plane-cli/skill-metadata.json"} {
				if len(members[required]) == 0 {
					t.Fatalf("missing %s", required)
				}
			}
			var skill struct {
				SkillVersion      string `json:"skill_version"`
				CLIVersion        string `json:"cli_version"`
				SupportedCLIRange string `json:"supported_cli_range"`
				SourceCommit      string `json:"source_commit"`
				Snapshot          *bool  `json:"snapshot"`
			}
			if err := json.Unmarshal(members["plane-cli/skill-metadata.json"], &skill); err != nil {
				t.Fatal(err)
			}
			if skill.SkillVersion != version || skill.CLIVersion != cliVersion || skill.SupportedCLIRange != "="+cliVersion || skill.SourceCommit != metadata.Commit || skill.Snapshot == nil || *skill.Snapshot != *metadata.Snapshot {
				t.Fatalf("skill metadata identity mismatch: %s", members["plane-cli/skill-metadata.json"])
			}
			continue
		}
		for _, required := range []string{"plane-cli", "LICENSE", "INSTALL.txt"} {
			if len(members[required]) == 0 {
				t.Fatalf("missing %s", required)
			}
		}
		if !bytes.Equal(members["THIRD_PARTY_NOTICES.txt"], notices) {
			t.Fatalf("dependency license notices missing or changed in %s", name)
		}
		binary := members["plane-cli"]
		arch := "amd64"
		if strings.HasSuffix(name, "_arm64.tar.gz") {
			arch = "arm64"
		}
		osName := "linux"
		if strings.Contains(name, "_darwin_") {
			osName = "darwin"
		}
		if osName == "linux" {
			f, err := elf.NewFile(bytes.NewReader(binary))
			if err != nil {
				t.Fatal(err)
			}
			expected := elf.EM_X86_64
			if arch == "arm64" {
				expected = elf.EM_AARCH64
			}
			if f.Machine != expected {
				t.Fatalf("wrong ELF target %s", name)
			}
			for _, p := range f.Progs {
				if p.Type == elf.PT_INTERP {
					t.Fatalf("dynamic interpreter in %s", name)
				}
			}
		} else {
			f, err := macho.NewFile(bytes.NewReader(binary))
			if err != nil {
				t.Fatal(err)
			}
			expected := macho.CpuAmd64
			if arch == "arm64" {
				expected = macho.CpuArm64
			}
			if f.Cpu != expected {
				t.Fatalf("wrong Mach-O target %s", name)
			}
		}
		if osName == runtime.GOOS && arch == runtime.GOARCH {
			prefix := t.TempDir()
			path := filepath.Join(prefix, "plane-cli")
			if err := os.WriteFile(path, binary, 0755); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"version"}, {"--help"}, {"commands", "schema", "--api-version", "v1"}, {"commands", "schema", "--api-version", "v2"}} {
				cmd := exec.Command(path, args...)
				cmd.Env = append(os.Environ(), "PLANE_CLI_CONFIG_DIR="+filepath.Join(prefix, "config"))
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("%v: %v %s", args, err, out)
				}
				if args[0] != "--help" && !json.Valid(out) {
					t.Fatalf("invalid JSON for %v", args)
				}
				if args[0] == "version" {
					var result struct {
						Data struct {
							Version string `json:"version"`
							Commit  string `json:"commit"`
						} `json:"data"`
					}
					if err := json.Unmarshal(out, &result); err != nil {
						t.Fatal(err)
					}
					if strings.TrimPrefix(result.Data.Version, "v") != cliVersion || result.Data.Commit != metadata.Commit {
						t.Fatalf("packaged CLI identity differs from metadata: %s", out)
					}
				}
			}
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("expected seven manifest members, got %d", len(seen))
	}
	formulaFile, err := os.ReadFile(filepath.Join(dir, "plane-cli.rb"))
	if err != nil {
		t.Fatal(err)
	}
	formula := string(formulaFile)
	if strings.Contains(formula, "@") {
		t.Fatal("rendered formula has unresolved placeholders")
	}
	if strings.Count(formula, "      url ") != 4 || strings.Count(formula, "      sha256 ") != 4 {
		t.Fatal("rendered formula must contain exactly four target URLs/checksums")
	}
	for _, osName := range []string{"darwin", "linux"} {
		platform := "macos"
		if osName == "linux" {
			platform = "linux"
		}
		block := "  on_" + platform + " do\n"
		for _, arch := range []string{"arm64", "amd64"} {
			selector := "arm"
			if arch == "amd64" {
				selector = "intel"
			}
			name := "plane-cli_" + version + "_" + osName + "_" + arch + ".tar.gz"
			url := "https://github.com/foae/plane-cli/releases/download/v" + version + "/" + name
			block += "    on_" + selector + " do\n      url \"" + url + "\"\n      sha256 \"" + seen[name] + "\"\n    end\n"
		}
		block += "  end\n"
		if !strings.Contains(formula, block) {
			t.Fatalf("rendered formula target URLs/checksums/selectors differ for %s", osName)
		}
	}
}
