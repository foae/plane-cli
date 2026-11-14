// Command checkdocs validates repository-local links and schema references offline.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/foae/plane-cli/internal/cli"
	"github.com/foae/plane-cli/internal/spec"
)

var (
	inlineLink    = regexp.MustCompile(`!?\[[^\]\n]*\]\(<?([^\s)>]+)>?(?:\s+"[^"]*")?\)`)
	referenceLink = regexp.MustCompile(`(?m)^\s{0,3}\[[^\]]+\]:\s*<?([^\s>]+)>?`)
	htmlLink      = regexp.MustCompile(`(?:href|src)=["']([^"']+)["']`)
	explicitID    = regexp.MustCompile(`\bid=["']([^"']+)["']`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "checkdocs:", err)
		os.Exit(1)
	}
}

func withoutFences(data string) string {
	lines := strings.Split(data, "\n")
	fence := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if fence != "" {
			lines[i] = ""
			if strings.HasPrefix(trim, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trim, "```") {
			fence = "```"
			lines[i] = ""
		} else if strings.HasPrefix(trim, "~~~") {
			fence = "~~~"
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

func slug(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func anchors(data string) map[string]bool {
	found := map[string]bool{}
	counts := map[string]int{}
	lines := strings.Split(withoutFences(data), "\n")
	add := func(heading string) {
		id := slug(heading)
		n := counts[id]
		counts[id]++
		if n != 0 {
			id = fmt.Sprintf("%s-%d", id, n)
		}
		found[id] = true
	}
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			n := len(trim) - len(strings.TrimLeft(trim, "#"))
			if n <= 6 && len(trim) > n && trim[n] == ' ' {
				add(strings.TrimSpace(strings.TrimRight(trim[n:], "#")))
			}
		} else if i > 0 && trim != "" && (strings.Trim(trim, "=") == "" || strings.Trim(trim, "-") == "") {
			add(lines[i-1])
		}
	}
	for _, match := range explicitID.FindAllStringSubmatch(data, -1) {
		found[match[1]] = true
	}
	return found
}

func run() error {
	var docs []string
	if err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".md") {
			docs = append(docs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	var failures []error
	anchorCache := map[string]map[string]bool{}
	links := 0
	for _, path := range docs {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body := withoutFences(string(data))
		for _, pattern := range []*regexp.Regexp{inlineLink, referenceLink, htmlLink} {
			for _, match := range pattern.FindAllStringSubmatch(body, -1) {
				u, err := url.Parse(match[1])
				if err != nil {
					failures = append(failures, fmt.Errorf("%s: malformed link %q", path, match[1]))
					continue
				}
				if u.IsAbs() || u.Host != "" {
					continue
				}
				links++
				target := path
				if u.Path != "" {
					if strings.HasPrefix(u.Path, "/") {
						target = filepath.Clean(strings.TrimPrefix(u.Path, "/"))
					} else {
						target = filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(u.Path)))
					}
				}
				if target == ".." || strings.HasPrefix(target, ".."+string(filepath.Separator)) {
					failures = append(failures, fmt.Errorf("%s: link leaves repository: %s", path, match[1]))
					continue
				}
				info, err := os.Stat(target)
				if err != nil {
					failures = append(failures, fmt.Errorf("%s: missing target %s", path, match[1]))
					continue
				}
				if u.Fragment == "" {
					continue
				}
				if info.IsDir() {
					target = filepath.Join(target, "README.md")
				}
				if strings.HasSuffix(target, ".json") {
					if err := jsonFragment(target, u.Fragment); err != nil {
						failures = append(failures, fmt.Errorf("%s: %w", path, err))
					}
					continue
				}
				if !strings.HasSuffix(target, ".md") {
					failures = append(failures, fmt.Errorf("%s: cannot validate fragment on %s", path, target))
					continue
				}
				if anchorCache[target] == nil {
					content, err := os.ReadFile(target)
					if err != nil {
						failures = append(failures, err)
						continue
					}
					anchorCache[target] = anchors(string(content))
				}
				if !anchorCache[target][u.Fragment] {
					failures = append(failures, fmt.Errorf("%s: missing fragment %s", path, match[1]))
				}
			}
		}
	}
	// Compile every schema shipped in the catalog against only its explicit resources.
	data, err := os.ReadFile("api/catalog.json")
	if err != nil {
		return err
	}
	var catalog struct {
		Commands []struct {
			Versions map[string]struct {
				Schema json.RawMessage `json:"schema"`
			} `json:"versions"`
		} `json:"commands"`
		Operations []struct {
			Request  json.RawMessage `json:"request_schema"`
			Response json.RawMessage `json:"response_schema"`
		} `json:"operations"`
		Resources map[string]json.RawMessage `json:"resources"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return err
	}
	var schemas []json.RawMessage
	for _, command := range catalog.Commands {
		for _, variant := range command.Versions {
			schemas = append(schemas, variant.Schema)
		}
	}
	for _, operation := range catalog.Operations {
		schemas = append(schemas, operation.Request, operation.Response)
	}
	for _, resource := range catalog.Resources {
		schemas = append(schemas, resource)
	}
	for _, schema := range schemas {
		if err := spec.Compile(schema, catalog.Resources); err != nil {
			failures = append(failures, fmt.Errorf("embedded schema: %w", err))
		}
	}
	if err := usageExamples(); err != nil {
		failures = append(failures, err)
	}
	if len(failures) != 0 {
		return errors.Join(failures...)
	}
	fmt.Printf("local_docs=%d local_links=%d offline_schemas=%d\n", len(docs), links, len(schemas))
	return nil
}

func jsonFragment(path, fragment string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if !strings.HasPrefix(fragment, "/") {
		return fmt.Errorf("unsupported JSON fragment %s#%s", path, fragment)
	}
	for _, token := range strings.Split(strings.TrimPrefix(fragment, "/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch node := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = node[token]
			if !ok {
				return fmt.Errorf("missing JSON fragment %s#%s", path, fragment)
			}
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return fmt.Errorf("missing JSON fragment %s#%s", path, fragment)
			}
			value = node[index]
		default:
			return fmt.Errorf("invalid JSON fragment %s#%s", path, fragment)
		}
	}
	return nil
}

// Only current local usage examples execute; future API examples never dispatch.
func usageExamples() error {
	data, err := os.ReadFile("docs/usage.md")
	if err != nil {
		return err
	}
	catalog, err := spec.Load()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "plane-docs-config-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	// Local now includes configuration writes, not just offline discovery.
	// Isolate every CLI override and restore the caller's environment afterward.
	for _, name := range []string{"CONFIG_DIR", "CONNECTION", "BASE_URL", "API_VERSION", "API_KEY", "ACCESS_TOKEN", "AUTH_MODE", "CREDENTIAL_BACKEND", "TIMEOUT", "REQUEST_TIMEOUT", "CA_FILE", "WORKSPACE", "PROJECT"} {
		key := "PLANE_CLI_" + name
		value, present := os.LookupEnv(key)
		defer func() {
			if present {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		}()
		if err := os.Unsetenv(key); err != nil {
			return err
		}
	}
	if err := os.Setenv("PLANE_CLI_CONFIG_DIR", dir); err != nil {
		return err
	}
	count := 0
	manual := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			manual = line == "```sh manual"
			continue
		}
		if manual {
			continue
		}
		if !strings.HasPrefix(line, "plane-cli ") {
			continue
		}
		words := strings.Fields(line)
		args := words[1:]
		safe := len(args) == 1 && (args[0] == "--help" || args[0] == "--version")
		for _, command := range catalog.Commands {
			if !command.Local || len(args) < len(command.Path) {
				continue
			}
			match := true
			for i, token := range command.Path {
				if args[i] != token {
					match = false
				}
			}
			if match {
				safe = true
			}
		}
		if !safe {
			return fmt.Errorf("docs/usage.md: example is not an implemented local command: %s", line)
		}
		var diagnostics bytes.Buffer
		if code := cli.Run(context.Background(), args, cli.Streams{In: strings.NewReader(""), Out: io.Discard, Err: &diagnostics}, cli.Build{}); code != 0 {
			return fmt.Errorf("docs/usage.md: local example failed with exit %d: %s", code, line)
		}
		count++
	}
	if count == 0 {
		return errors.New("docs/usage.md: no runnable local examples")
	}
	return nil
}
