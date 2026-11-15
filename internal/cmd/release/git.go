package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (c command) git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = c.root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return data, nil
}

func (c command) gitDirectory() (string, error) {
	root, err := c.git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	actual, err := filepath.EvalSymlinks(c.root)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(root)) != actual {
		return "", fmt.Errorf("run release tooling from the Git worktree root")
	}
	dir, err := c.git("rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(dir)), nil
}

func (c command) withLock(fn func(string) error) (err error) {
	dir, err := c.gitDirectory()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "plane-cli-release.lock")
	if err := os.Mkdir(path, 0700); err != nil {
		return fmt.Errorf("acquire release lock %s: %w; another release process may be running; remove this lock only after confirming no release process remains", path, err)
	}
	defer func() {
		if removeErr := os.Remove(path); removeErr != nil {
			err = joinError(err, fmt.Errorf("release lock cleanup failed: %w", removeErr))
		}
	}()
	return fn(dir)
}

func joinError(original, additional error) error {
	if additional == nil {
		return original
	}
	if original == nil {
		return additional
	}
	return fmt.Errorf("%w; %w", original, additional)
}

func (c command) requireClean() error {
	if _, err := c.gitDirectory(); err != nil {
		return err
	}
	status, err := c.git("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return fmt.Errorf("release operation requires a clean Git worktree, including untracked files")
	}
	return nil
}

func (c command) requireTagAtHead(tag string) error {
	head, err := c.git("rev-parse", "--verify", "HEAD")
	if err != nil {
		return err
	}
	target, err := c.git("rev-parse", "--verify", "refs/tags/"+tag+"^{commit}")
	if err != nil {
		return err
	}
	if !bytes.Equal(head, target) {
		return fmt.Errorf("tag %s does not identify HEAD", tag)
	}
	return nil
}

func (c command) rejectExistingTag(target version) error {
	data, err := c.git("tag", "--list", "v"+target.String(), target.String())
	if err != nil {
		return err
	}
	if len(data) != 0 {
		return fmt.Errorf("tag already exists for version %s", target.String())
	}
	return nil
}

func (c command) requireTracked(s state) error {
	args := []string{"ls-files", "--error-unmatch", "--", ":(literal)VERSION"}
	for _, f := range s.fragments {
		args = append(args, ":(literal)"+f.path)
	}
	for name := range s.history {
		args = append(args, ":(literal)releases/"+name+".json")
	}
	if _, err := c.git(args...); err != nil {
		return fmt.Errorf("all release inputs must be tracked, including ignored managed files: %w", err)
	}
	return nil
}

type gitChange struct {
	status string
	path   string
}

func (c command) changedPaths(base string) ([]gitChange, error) {
	data, err := c.git("diff", "--name-status", "-z", "--no-renames", base, "--")
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(data, []byte{0})
	var changes []gitChange
	for i := 0; i+1 < len(parts); i += 2 {
		changes = append(changes, gitChange{status: string(parts[i]), path: string(parts[i+1])})
	}
	untracked, err := c.git("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range bytes.Split(untracked, []byte{0}) {
		if len(path) != 0 {
			changes = append(changes, gitChange{status: "?", path: string(path)})
		}
	}
	return changes, nil
}

func fragmentPath(path string) bool {
	return strings.HasPrefix(path, "changes/") && strings.HasSuffix(path, ".json") && !strings.Contains(strings.TrimPrefix(path, "changes/"), "/")
}

func (c command) checkBase(base string, preparation bool, current state) error {
	if _, err := c.gitDirectory(); err != nil {
		return err
	}
	resolved, err := c.git("rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		return err
	}
	sha := strings.TrimSpace(string(resolved))
	changes, err := c.changedPaths(sha)
	if err != nil {
		return err
	}
	if preparation {
		return c.checkPreparation(sha, changes, current)
	}
	if len(changes) == 0 {
		return nil
	}
	fragments := make(map[string]bool, len(current.fragments))
	for _, f := range current.fragments {
		fragments[f.path] = true
	}
	addedFragment := false
	for _, change := range changes {
		switch {
		case change.path == "VERSION" && change.status != "A":
			return fmt.Errorf("VERSION changes require release-preparation mode")
		case change.path == "releases" || strings.HasPrefix(change.path, "releases/"):
			return fmt.Errorf("release history changes require release-preparation mode: %s", change.path)
		case fragmentPath(change.path):
			if change.status != "A" && change.status != "?" {
				return fmt.Errorf("existing release fragment must not be modified or deleted: %s", change.path)
			}
			if change.status == "A" && fragments[change.path] {
				addedFragment = true
			}
		}
	}
	if addedFragment {
		return nil
	}
	return fmt.Errorf("change set requires a new, valid, tracked changes/*.json fragment")
}

func (c command) stateAt(base string) (state, error) {
	s := state{history: make(map[string]releaseNotes)}
	tree, err := c.git("ls-tree", "-r", "-z", base, "--", "VERSION", "changes", "releases")
	if err != nil {
		return s, err
	}
	for _, entry := range bytes.Split(tree, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		metadata, path, ok := bytes.Cut(entry, []byte{'\t'})
		if !ok {
			return s, fmt.Errorf("invalid Git tree entry")
		}
		fields := bytes.Fields(metadata)
		if len(fields) != 3 || (string(fields[0]) != "100644" && string(fields[0]) != "100755") {
			return s, fmt.Errorf("base managed path %s is not a regular file", path)
		}
		if string(path) != "VERSION" && !strings.HasSuffix(string(path), ".json") {
			continue
		}
		if string(path) != "VERSION" && !fragmentPath(string(path)) && (strings.Count(string(path), "/") != 1 || !strings.HasPrefix(string(path), "releases/")) {
			return s, fmt.Errorf("unexpected nested managed path %s", path)
		}
		data, err := c.git("cat-file", "blob", string(fields[2]))
		if err != nil {
			return s, err
		}
		if string(path) == "VERSION" {
			s.versionData = data
		} else if err := s.addFile(string(path), data); err != nil {
			return s, err
		}
	}
	if s.versionData == nil {
		return s, fmt.Errorf("base commit has no VERSION file")
	}
	s.current, err = parseVersionFile(s.versionData)
	if err != nil {
		return s, err
	}
	return s, s.validate()
}

func (c command) checkPreparation(base string, changes []gitChange, current state) error {
	previous, err := c.stateAt(base)
	if err != nil {
		return fmt.Errorf("release-preparation base: %w", err)
	}
	if err := previous.validateTarget(current.current); err != nil {
		return err
	}
	if len(current.fragments) != 0 {
		return fmt.Errorf("release-preparation must consume the complete base fragment set without adding fragments")
	}
	notePath := "releases/" + current.current.String() + ".json"
	expected := map[string]string{"VERSION": "M", notePath: "A"}
	for _, f := range previous.fragments {
		expected[f.path] = "D"
	}
	if len(changes) != len(expected) {
		return fmt.Errorf("release-preparation change set must contain only VERSION, new history and consumed fragments")
	}
	for _, change := range changes {
		if expected[change.path] != change.status {
			return fmt.Errorf("release-preparation contains unrelated or invalid change %s %s", change.status, change.path)
		}
		delete(expected, change.path)
	}
	if len(expected) != 0 {
		return fmt.Errorf("release-preparation is missing required input consumption")
	}
	notes, exists := current.history[current.current.String()]
	if len(previous.fragments) == 0 {
		if !exists || notes.PromotedFrom != previous.current.String() || len(notes.Changes) != 0 {
			return fmt.Errorf("release-preparation promotion must reference the exact previous version %s without changes", previous.current)
		}
		return nil
	}
	if !exists || notes.PromotedFrom != "" || len(notes.Changes) != len(previous.fragments) {
		return fmt.Errorf("release-preparation notes do not match consumed fragments")
	}
	for i, f := range previous.fragments {
		if notes.Changes[i] != f.note {
			return fmt.Errorf("release-preparation notes differ from consumed fragment %s or its deterministic order", f.path)
		}
	}
	return nil
}
