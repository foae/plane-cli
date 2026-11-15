package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const initialVersion = "1.0.0-alpha.0"

type change struct {
	Level   string `json:"level"`
	Summary string `json:"summary"`
}

type fragment struct {
	path string
	data []byte
	note change
}

type releaseNotes struct {
	Version      string   `json:"version"`
	Date         string   `json:"date"`
	Changes      []change `json:"changes"`
	PromotedFrom string   `json:"promoted_from,omitempty"`
}

type state struct {
	current     version
	versionData []byte
	fragments   []fragment
	history     map[string]releaseNotes
}

type plan struct {
	Version        string   `json:"version"`
	RequiredBump   string   `json:"required_bump"`
	MinimumVersion string   `json:"minimum_version"`
	Changes        []change `json:"changes"`
	minimumCore    version
}

func validateChange(c change) error {
	if levelRank(c.Level) == 0 {
		return fmt.Errorf("invalid change level %q (want patch, minor or major)", c.Level)
	}
	if !utf8.ValidString(c.Summary) || strings.TrimSpace(c.Summary) == "" || strings.ContainsAny(c.Summary, "\r\n\u0085\u2028\u2029") {
		return fmt.Errorf("summary must be nonempty, valid UTF-8 and a single line")
	}
	return nil
}

func decodeJSON(data []byte, target any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON input must be valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}

func encodeJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func regularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file, not a symlink or special file", path)
	}
	return os.ReadFile(path)
}

func directory(path string, optional bool) (bool, error) {
	info, err := os.Lstat(path)
	if optional && os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%s must be a directory, not a symlink or special file", path)
	}
	return true, nil
}

func parseVersionFile(data []byte) (version, error) {
	s := strings.TrimSuffix(string(data), "\n")
	v, err := parseVersion(s)
	if err != nil {
		return version{}, fmt.Errorf("VERSION: %w", err)
	}
	return v, nil
}

func readState(root string) (state, error) {
	s := state{history: make(map[string]releaseNotes)}
	var err error
	s.versionData, err = regularFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return s, err
	}
	s.current, err = parseVersionFile(s.versionData)
	if err != nil {
		return s, err
	}
	for _, dir := range []string{"changes", "releases"} {
		full := filepath.Join(root, dir)
		exists, err := directory(full, true)
		if err != nil {
			return s, err
		}
		if !exists {
			continue
		}
		entries, err := os.ReadDir(full)
		if err != nil {
			return s, err
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			data, err := regularFile(filepath.Join(root, path))
			if err != nil {
				return s, err
			}
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			if err := s.addFile(filepath.ToSlash(path), data); err != nil {
				return s, err
			}
		}
	}
	return s, s.validate()
}

func (s *state) addFile(path string, data []byte) error {
	if strings.HasPrefix(path, "changes/") {
		var c change
		if err := decodeJSON(data, &c); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := validateChange(c); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		s.fragments = append(s.fragments, fragment{path: path, data: data, note: c})
		return nil
	}
	var n releaseNotes
	if err := decodeJSON(data, &n); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	v, err := parseVersion(n.Version)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if path != "releases/"+v.String()+".json" {
		return fmt.Errorf("%s: history filename must match its version %s", path, n.Version)
	}
	if _, exists := s.history[n.Version]; exists {
		return fmt.Errorf("duplicate history version %s", n.Version)
	}
	date, err := time.Parse("2006-01-02", n.Date)
	if err != nil || date.Format("2006-01-02") != n.Date {
		return fmt.Errorf("%s: date must be a UTC calendar date (YYYY-MM-DD)", path)
	}
	if n.PromotedFrom != "" && len(n.Changes) != 0 {
		return fmt.Errorf("%s: promoted release notes must not contain changes", path)
	}
	if len(n.Changes) == 0 && n.PromotedFrom == "" {
		return fmt.Errorf("%s: empty release notes require promoted_from lineage", path)
	}
	for _, c := range n.Changes {
		if err := validateChange(c); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	s.history[n.Version] = n
	return nil
}

func (s *state) validate() error {
	first := version{major: "1", minor: "0", patch: "0", stage: "alpha", sequence: "0"}
	if compareVersion(s.current, first) < 0 {
		return fmt.Errorf("VERSION must target 1.0.0 or later")
	}
	stableFound := s.current.stage == ""
	for name := range s.history {
		v, err := parseVersion(name)
		if err != nil {
			return err
		}
		if compareVersion(v, first) < 0 || compareVersion(v, s.current) > 0 {
			return fmt.Errorf("history version %s is outside initial version through VERSION", name)
		}
		notes := s.history[name]
		if notes.PromotedFrom != "" {
			source, err := parseVersion(notes.PromotedFrom)
			if err != nil || !validPromotion(source, v) {
				return fmt.Errorf("history version %s has invalid promoted_from %q: require an earlier same-core preview channel", name, notes.PromotedFrom)
			}
			if _, exists := s.history[notes.PromotedFrom]; !exists {
				return fmt.Errorf("history version %s references missing promoted_from %s", name, notes.PromotedFrom)
			}
		}
		stableFound = stableFound || v.stage == ""
	}
	if !stableFound && s.current.core() != "1.0.0" {
		return fmt.Errorf("before the first stable release VERSION must retain the 1.0.0 core target")
	}
	if s.current.String() != initialVersion {
		if _, exists := s.history[s.current.String()]; !exists {
			return fmt.Errorf("VERSION %s has no matching release notes", s.current.String())
		}
	}
	sort.Slice(s.fragments, func(i, j int) bool { return s.fragments[i].path < s.fragments[j].path })
	return nil
}

func (s state) makePlan() plan {
	p := plan{Version: s.current.String(), RequiredBump: "none", Changes: make([]change, 0, len(s.fragments))}
	for _, f := range s.fragments {
		p.RequiredBump = higherLevel(p.RequiredBump, f.note.Level)
		p.Changes = append(p.Changes, f.note)
	}
	var stable version
	stableFound := false
	for name := range s.history {
		v, _ := parseVersion(name)
		if v.stage == "" && (!stableFound || compareVersion(v, stable) > 0) {
			stable, stableFound = v, true
		}
	}
	if s.current.stage == "" {
		stable, stableFound = s.current, true
	}
	if !stableFound {
		p.minimumCore = version{major: "1", minor: "0", patch: "0"}
	} else {
		impact := p.RequiredBump
		for name, notes := range s.history {
			v, _ := parseVersion(name)
			if compareVersion(v, stable) > 0 {
				for _, c := range notes.Changes {
					impact = higherLevel(impact, c.Level)
				}
			}
		}
		p.RequiredBump = impact
		p.minimumCore = bump(stable, impact)
		if compareCore(s.current, p.minimumCore) > 0 {
			p.minimumCore = s.current
			p.minimumCore.stage, p.minimumCore.sequence = "", ""
		}
	}
	minimum := p.minimumCore
	if s.current.stage != "" && compareCore(s.current, minimum) == 0 {
		minimum.stage, minimum.sequence = s.current.stage, increment(s.current.sequence)
	}
	p.MinimumVersion = minimum.String()
	return p
}

func validPromotion(source, target version) bool {
	return source.stage != "" && source.stage != target.stage &&
		compareCore(source, target) == 0 && compareVersion(source, target) < 0
}

func (s state) canPromote(target version) bool {
	_, prepared := s.history[s.current.String()]
	return prepared && validPromotion(s.current, target)
}

func (s state) validateTarget(target version) error {
	if len(s.fragments) == 0 && !s.canPromote(target) {
		return fmt.Errorf("release preparation requires pending changes or advancement of a prepared preview to a later channel on the same core")
	}
	if compareVersion(target, s.current) <= 0 {
		return fmt.Errorf("requested version %s must be greater than VERSION %s", target.String(), s.current.String())
	}
	if _, exists := s.history[target.String()]; exists {
		return fmt.Errorf("release history already contains %s", target.String())
	}
	p := s.makePlan()
	if compareCore(target, p.minimumCore) < 0 {
		return fmt.Errorf("requested version %s does not satisfy %s impact: core must be at least %s", target.String(), p.RequiredBump, p.minimumCore.core())
	}
	stableFound := s.current.stage == ""
	for name := range s.history {
		v, _ := parseVersion(name)
		stableFound = stableFound || v.stage == ""
	}
	if !stableFound && target.core() != "1.0.0" {
		return fmt.Errorf("before the first stable release the core target must remain 1.0.0")
	}
	return nil
}
