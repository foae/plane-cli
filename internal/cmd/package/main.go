// Command package builds local release artifacts; it never publishes them.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

func main() {
	out := flag.String("output", "dist/snapshot", "new output directory")
	release := flag.Bool("release", false, "require clean exact release tag")
	flag.Parse()
	if err := run(*out, *release); err != nil {
		fmt.Fprintln(os.Stderr, "package:", err)
		os.Exit(1)
	}
}

func command(env []string, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Env = append(os.Environ(), env...)
	b, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, b)
	}
	return b, nil
}

func run(out string, release bool) error {
	out = filepath.Clean(out)
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(raw))
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(alpha|beta|rc)\.(0|[1-9][0-9]*))?$`).MatchString(version) {
		return fmt.Errorf("invalid VERSION")
	}
	if _, err := command(nil, "go", "run", "./internal/cmd/release", "version-check"); err != nil {
		return err
	}
	revision, err := command(nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(revision))
	status, err := command(nil, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	dirty := len(bytes.TrimSpace(status)) != 0
	stamp, err := command(nil, "git", "show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return err
	}
	date, err := time.Parse(time.RFC3339, strings.TrimSpace(string(stamp)))
	if err != nil {
		return err
	}
	identity := "dev"
	if release {
		if _, err := command(nil, "go", "run", "./internal/cmd/release", "version-check", "--tag", "v"+version); err != nil {
			return err
		}
		if !strings.Contains(version, "-") {
			if _, err := command(nil, "go", "run", "./internal/cmd/checkapi", "--release"); err != nil {
				return err
			}
		}
		identity = "v" + version
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return fmt.Errorf("output must not exist: %s", out)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(out), ".package-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	notices, err := os.ReadFile("THIRD_PARTY_NOTICES.txt")
	if err != nil {
		return err
	}
	guidance := []byte("Verify SHA256SUMS before extracting. Copy plane-cli to a directory on PATH; no sudo or installer is required. Run plane-cli version and plane-cli --help. Snapshot builds report dev and are not published releases. Live Plane edition compatibility is unverified.\n")
	sums := map[string]string{}
	targets := []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}
	for _, target := range targets {
		parts := strings.Split(target, "/")
		binary := filepath.Join(stage, "plane-cli")
		fmt.Println("Packaging", target)
		flags := "-X main.version=" + identity + " -X main.commit=" + commit
		if _, err := command([]string{"CGO_ENABLED=0", "GOOS=" + parts[0], "GOARCH=" + parts[1]}, "go", "build", "-trimpath", "-buildvcs=false", "-ldflags", flags, "-o", binary, "./cmd/plane-cli"); err != nil {
			return err
		}
		name := "plane-cli_" + version + "_" + parts[0] + "_" + parts[1] + ".tar.gz"
		sum, err := archive(filepath.Join(stage, name), date, []entry{{name: "plane-cli", source: binary, mode: 0755}, {name: "LICENSE", data: license, mode: 0644}, {name: "THIRD_PARTY_NOTICES.txt", data: notices, mode: 0644}, {name: "INSTALL.txt", data: guidance, mode: 0644}})
		if err != nil {
			return err
		}
		sums[name] = sum
		if err := os.Remove(binary); err != nil {
			return err
		}
	}
	cliVersion := strings.TrimPrefix(identity, "v")
	metadata := map[string]any{"skill_version": version, "cli_version": cliVersion, "supported_cli_range": "=" + cliVersion, "source_commit": commit, "snapshot": !release}
	meta, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	entries := []entry{{name: "plane-cli/LICENSE", data: license, mode: 0644}, {name: "plane-cli/skill-metadata.json", data: append(meta, '\n'), mode: 0644}}
	err = filepath.WalkDir("skills/plane-cli", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("nonregular skill file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data = bytes.ReplaceAll(data, []byte("@VERSION@"), []byte(version))
		data = bytes.ReplaceAll(data, []byte("@COMMIT@"), []byte(commit))
		data = bytes.ReplaceAll(data, []byte("@CLI_VERSION@"), []byte(cliVersion))
		relative, err := filepath.Rel("skills", path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{name: filepath.ToSlash(relative), data: data, mode: 0644})
		return nil
	})
	if err != nil {
		return err
	}
	name := "plane-cli-skill_" + version + ".tar.gz"
	sums[name], err = archive(filepath.Join(stage, name), date, entries)
	if err != nil {
		return err
	}
	formula, err := os.ReadFile("Formula/plane-cli.rb")
	if err != nil {
		return err
	}
	formula = bytes.ReplaceAll(formula, []byte("@VERSION@"), []byte(version))
	for _, target := range targets {
		parts := strings.Split(target, "/")
		name := "plane-cli_" + version + "_" + parts[0] + "_" + parts[1] + ".tar.gz"
		formula = bytes.ReplaceAll(formula, []byte("@SHA_"+strings.ToUpper(parts[0]+"_"+parts[1])+"@"), []byte(sums[name]))
	}
	if err := os.WriteFile(filepath.Join(stage, "plane-cli.rb"), formula, 0644); err != nil {
		return err
	}
	hashes := map[string]string{}
	for _, path := range []string{"api/catalog.json", "api/provenance.json", "go.mod", "go.sum", "tools/versions.mk"} {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(b)
		hashes[path] = hex.EncodeToString(digest[:])
	}
	releaseMeta := map[string]any{"version": version, "cli_version": cliVersion, "commit": commit, "source_dirty": dirty, "date": date.UTC().Format(time.RFC3339), "snapshot": !release, "go_version": runtime.Version(), "targets": targets, "input_sha256": hashes, "archive_sha256": sums, "compatibility": "Documentation-backed fake-server acceptance; live editions and native keyrings unverified."}
	b, err := json.MarshalIndent(releaseMeta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "release-metadata.json"), append(b, '\n'), 0644); err != nil {
		return err
	}
	checksums := make(map[string]string, len(sums)+2)
	for name, sum := range sums {
		checksums[name] = sum
	}
	for _, name := range []string{"plane-cli.rb", "release-metadata.json"} {
		data, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		checksums[name] = hex.EncodeToString(digest[:])
	}
	var manifest strings.Builder
	names := make([]string, 0, len(checksums))
	for name := range checksums {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&manifest, "%s  %s\n", checksums[name], name)
	}
	if err := os.WriteFile(filepath.Join(stage, "SHA256SUMS"), []byte(manifest.String()), 0644); err != nil {
		return err
	}
	if err := os.Rename(stage, out); err != nil {
		return err
	}
	fmt.Println("Artifacts staged in", out)
	return nil
}

type entry struct {
	name   string
	data   []byte
	source string
	mode   int64
}

func archive(path string, date time.Time, entries []entry) (sum string, result error) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); result == nil {
			result = err
		}
	}()
	digest := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(file, digest))
	tw := tar.NewWriter(gz)
	for _, item := range entries {
		if err := archiveEntry(tw, item, date); err != nil {
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func archiveEntry(tw *tar.Writer, item entry, date time.Time) (result error) {
	var body io.Reader = bytes.NewReader(item.data)
	size := int64(len(item.data))
	if item.source != "" {
		file, err := os.Open(item.source)
		if err != nil {
			return err
		}
		defer func() {
			if err := file.Close(); result == nil {
				result = err
			}
		}()
		info, err := file.Stat()
		if err != nil {
			return err
		}
		size, body = info.Size(), file
	}
	if err := tw.WriteHeader(&tar.Header{Name: item.name, Mode: item.mode, Size: size, ModTime: date.UTC(), Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
		return err
	}
	_, err := io.Copy(tw, body)
	return err
}
