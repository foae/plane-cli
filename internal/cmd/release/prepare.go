package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func writeExclusive(path string, data []byte, mode os.FileMode) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = joinError(err, closeErr)
		}
		if err != nil {
			if removeErr := os.Remove(path); removeErr != nil {
				err = joinError(err, fmt.Errorf("remove incomplete output %s: %w", path, removeErr))
			}
		}
	}()
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

func (c command) createChange(note change) (path string, err error) {
	if err := validateChange(note); err != nil {
		return "", err
	}
	data, err := encodeJSON(note)
	if err != nil {
		return "", err
	}
	err = c.withLock(func(_ string) error {
		if _, err := readState(c.root); err != nil {
			return err
		}
		dir := filepath.Join(c.root, "changes")
		exists, err := directory(dir, true)
		if err != nil {
			return err
		}
		if !exists {
			if err := os.Mkdir(dir, 0755); err != nil {
				return err
			}
		}
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			if !exists {
				err = joinError(err, os.Remove(dir))
			}
			return err
		}
		path = filepath.Join("changes", hex.EncodeToString(random[:])+".json")
		if err := writeExclusive(filepath.Join(c.root, path), data, 0644); err != nil {
			if !exists {
				err = joinError(err, os.Remove(dir))
			}
			return err
		}
		return nil
	})
	return path, err
}

func (c command) prepare(target version) error {
	return c.withLock(func(_ string) error {
		if err := c.requireClean(); err != nil {
			return err
		}
		s, err := readState(c.root)
		if err != nil {
			return err
		}
		if err := s.validateTarget(target); err != nil {
			return err
		}
		if err := c.requireTracked(s); err != nil {
			return err
		}
		if err := c.rejectExistingTag(target); err != nil {
			return err
		}
		return c.installRelease(s, target)
	})
}

// All release outputs and fragment backups exist before the first managed-path
// mutation. VERSION is the final atomic replacement. This handles ordinary I/O
// failures, not process termination or power-loss recovery.
func (c command) installRelease(s state, target version) (err error) {
	notes := releaseNotes{Version: target.String(), Date: c.now().UTC().Format("2006-01-02"), Changes: s.makePlan().Changes}
	if len(s.fragments) == 0 {
		notes.PromotedFrom = s.current.String()
	}
	data, err := encodeJSON(notes)
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(c.root, ".plane-cli-release-stage-")
	if err != nil {
		return err
	}
	preserveStage := false
	defer func() {
		if !preserveStage {
			if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
				err = joinError(err, fmt.Errorf("release staging cleanup failed: %w", cleanupErr))
			}
		}
	}()
	versionPath := filepath.Join(c.root, "VERSION")
	versionInfo, err := os.Lstat(versionPath)
	if err != nil {
		return err
	}
	if err := writeExclusive(filepath.Join(stage, "VERSION"), []byte(target.String()+"\n"), versionInfo.Mode().Perm()); err != nil {
		return err
	}
	if err := writeExclusive(filepath.Join(stage, "notes.json"), data, 0644); err != nil {
		return err
	}
	backups := make([]string, len(s.fragments))
	for i, f := range s.fragments {
		info, err := os.Lstat(filepath.Join(c.root, f.path))
		if err != nil {
			return err
		}
		backups[i] = filepath.Join(stage, fmt.Sprintf("fragment-%d.json", i))
		if err := writeExclusive(backups[i], f.data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	// No path below this point is changed until all validation and staging pass.
	releasesDir := filepath.Join(c.root, "releases")
	exists, err := directory(releasesDir, true)
	if err != nil {
		return err
	}
	createdDir := false
	noteInstalled := false
	consumed := 0
	notePath := filepath.Join(releasesDir, target.String()+".json")
	rollback := func(cause error) error {
		var rollbackErr error
		for i := range consumed {
			if err := os.Link(backups[i], filepath.Join(c.root, s.fragments[i].path)); err != nil {
				rollbackErr = joinError(rollbackErr, fmt.Errorf("restore %s: %w", s.fragments[i].path, err))
			}
		}
		if noteInstalled {
			if err := c.remove(notePath); err != nil {
				rollbackErr = joinError(rollbackErr, fmt.Errorf("remove notes: %w", err))
			}
		}
		if createdDir {
			if err := os.Remove(releasesDir); err != nil {
				rollbackErr = joinError(rollbackErr, fmt.Errorf("remove new releases directory: %w", err))
			}
		}
		if rollbackErr != nil {
			preserveStage = true
			return fmt.Errorf("%w; rollback failed: %w; recovery backups retained at %s", cause, rollbackErr, stage)
		}
		return cause
	}
	if !exists {
		if err := os.Mkdir(releasesDir, 0755); err != nil {
			return err
		}
		createdDir = true
	}
	// Link rather than Rename refuses an existing destination without replacing it.
	if err := os.Link(filepath.Join(stage, "notes.json"), notePath); err != nil {
		return rollback(err)
	}
	noteInstalled = true
	for _, f := range s.fragments {
		if err := c.remove(filepath.Join(c.root, f.path)); err != nil {
			return rollback(err)
		}
		consumed++
	}
	if err := c.rename(filepath.Join(stage, "VERSION"), versionPath); err != nil {
		return rollback(err)
	}
	return nil
}
