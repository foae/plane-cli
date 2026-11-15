package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Only the release channels in the repository's release contract are accepted.
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(alpha|beta|rc)\.(0|[1-9][0-9]*))?$`)

type version struct {
	major, minor, patch string
	stage, sequence     string
}

func parseVersion(s string) (version, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return version{}, fmt.Errorf("invalid canonical release version %q (want X.Y.Z[-alpha.N|-beta.N|-rc.N])", s)
	}
	return version{m[1], m[2], m[3], m[4], m[5]}, nil
}

func (v version) String() string {
	s := v.core()
	if v.stage != "" {
		s += "-" + v.stage + "." + v.sequence
	}
	return s
}

func (v version) core() string { return v.major + "." + v.minor + "." + v.patch }

func compareNumber(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

func compareCore(a, b version) int {
	for _, pair := range [][2]string{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if c := compareNumber(pair[0], pair[1]); c != 0 {
			return c
		}
	}
	return 0
}

func compareVersion(a, b version) int {
	if c := compareCore(a, b); c != 0 {
		return c
	}
	if a.stage == "" && b.stage != "" {
		return 1
	}
	if a.stage != "" && b.stage == "" {
		return -1
	}
	if stageRank(a.stage) < stageRank(b.stage) {
		return -1
	}
	if stageRank(a.stage) > stageRank(b.stage) {
		return 1
	}
	return compareNumber(a.sequence, b.sequence)
}

func stageRank(stage string) int {
	switch stage {
	case "alpha":
		return 0
	case "beta":
		return 1
	case "rc":
		return 2
	default:
		return 3
	}
}

func increment(s string) string {
	b := []byte(s)
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != '9' {
			b[i]++
			return string(b)
		}
		b[i] = '0'
	}
	return "1" + string(b)
}

func bump(v version, level string) version {
	v.stage, v.sequence = "", ""
	switch level {
	case "major":
		v.major, v.minor, v.patch = increment(v.major), "0", "0"
	case "minor":
		v.minor, v.patch = increment(v.minor), "0"
	case "patch":
		v.patch = increment(v.patch)
	}
	return v
}

func levelRank(level string) int {
	switch level {
	case "patch":
		return 1
	case "minor":
		return 2
	case "major":
		return 3
	default:
		return 0
	}
}

func higherLevel(a, b string) string {
	if levelRank(b) > levelRank(a) {
		return b
	}
	return a
}
