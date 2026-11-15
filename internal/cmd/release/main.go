// Command release manages local release inputs. It never commits, tags or publishes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type command struct {
	root string
	now  func() time.Time
	// Fault injection is limited to the two filesystem operations whose failure
	// can occur after fragments have been consumed.
	rename func(string, string) error
	remove func(string) error
}

func main() {
	root, err := os.Getwd()
	if err == nil {
		c := newCommand(root)
		err = c.run(os.Args[1:], os.Stdout, os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func newCommand(root string) command {
	return command{root: root, now: time.Now, rename: os.Rename, remove: os.Remove}
}

func (c command) run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: release change|version-plan|release-prepare|version-check [flags]")
	}
	name := args[0]
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var level, summary, requested, base, tag, skillMetadata string
	var preparation bool
	switch name {
	case "change":
		fs.StringVar(&level, "level", os.Getenv("RELEASE_LEVEL"), "release impact: patch, minor or major")
		fs.StringVar(&summary, "summary", os.Getenv("RELEASE_SUMMARY"), "nonempty single-line release note")
	case "version-plan":
	case "release-prepare":
		fs.StringVar(&requested, "version", os.Getenv("RELEASE_VERSION"), "canonical target version, without v prefix")
	case "version-check":
		fs.StringVar(&base, "base", os.Getenv("RELEASE_BASE"), "base Git commit for CI change-set validation")
		defaultPreparation := false
		if value := os.Getenv("RELEASE_PREPARATION"); value != "" {
			var err error
			defaultPreparation, err = strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("RELEASE_PREPARATION: %w", err)
			}
		}
		fs.BoolVar(&preparation, "release-preparation", defaultPreparation, "verify only a complete release-input consumption change set (requires --base)")
		fs.StringVar(&tag, "tag", "", "require this exact v-prefixed release tag and a clean worktree")
		fs.StringVar(&skillMetadata, "skill-metadata", "", "optional JSON file containing matching skill_version and cli_version")
	default:
		return fmt.Errorf("unknown release command %q", name)
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("%s does not accept positional arguments", name)
	}
	switch name {
	case "change":
		path, err := c.createChange(change{Level: level, Summary: summary})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, filepath.ToSlash(path))
		return err
	case "release-prepare":
		v, err := parseVersion(requested)
		if err != nil {
			return err
		}
		if err := c.prepare(v); err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, "prepared", v.String())
		return err
	}
	s, err := readState(c.root)
	if err != nil {
		return err
	}
	if name == "version-plan" {
		data, err := encodeJSON(s.makePlan())
		if err != nil {
			return err
		}
		_, err = stdout.Write(data)
		return err
	}
	if preparation && base == "" {
		return fmt.Errorf("--release-preparation requires --base")
	}
	if tag != "" {
		if tag != "v"+s.current.String() {
			return fmt.Errorf("tag %q does not match VERSION (want v%s)", tag, s.current.String())
		}
		if err := c.requireClean(); err != nil {
			return err
		}
		if err := c.requireTagAtHead(tag); err != nil {
			return err
		}
	}
	if skillMetadata != "" {
		if err := checkSkillMetadata(c.root, skillMetadata, s.current.String()); err != nil {
			return err
		}
	}
	if base != "" {
		if err := c.checkBase(base, preparation, s); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(stdout, "version inputs valid:", s.current.String())
	return err
}

func checkSkillMetadata(root, path, expected string) error {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	// The explicit packaging input is not a guessed source-template location.
	data, err := regularFile(path)
	if err != nil {
		return err
	}
	var metadata struct {
		SkillVersion string `json:"skill_version"`
		CLIVersion   string `json:"cli_version"`
	}
	if err := decodeJSON(data, &metadata); err != nil {
		return fmt.Errorf("skill metadata: %w", err)
	}
	if metadata.SkillVersion != expected || metadata.CLIVersion != expected {
		return fmt.Errorf("skill metadata versions must both equal VERSION %s", expected)
	}
	return nil
}
