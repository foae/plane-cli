// Package semver parses and compares Semantic Versioning 2.0.0 versions.
package semver

import (
	"errors"
	"strings"
)

// Version retains numeric components as strings to avoid integer overflow.
type Version struct {
	Major, Minor, Patch string
	Prerelease, Build   string
}

func Parse(s string) (Version, error) {
	var v Version
	core, build, hasBuild := strings.Cut(s, "+")
	if hasBuild && !identifiers(build, false) {
		return v, errors.New("invalid semantic version")
	}
	core, pre, hasPre := strings.Cut(core, "-")
	major, rest, first := strings.Cut(core, ".")
	minor, patch, second := strings.Cut(rest, ".")
	if !first || !second || !number(major) || !number(minor) || !number(patch) || (hasPre && !identifiers(pre, true)) {
		return v, errors.New("invalid semantic version")
	}
	return Version{major, minor, patch, pre, build}, nil
}

func number(s string) bool {
	return numeric(s) && (len(s) == 1 || s[0] != '0')
}

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func identifiers(s string, prerelease bool) bool {
	for {
		part, rest, more := strings.Cut(s, ".")
		if part == "" || (prerelease && numeric(part) && !number(part)) {
			return false
		}
		for i := range len(part) {
			c := part[i]
			if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '-' {
				return false
			}
		}
		if !more {
			return true
		}
		s = rest
	}
}

// CompareNumber compares canonical nonnegative decimal numbers without overflow.
func CompareNumber(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

// Compare ignores build metadata, as required by SemVer precedence.
func Compare(a, b Version) int {
	for _, pair := range [][2]string{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if c := CompareNumber(pair[0], pair[1]); c != 0 {
			return c
		}
	}
	if a.Prerelease == "" && b.Prerelease != "" {
		return 1
	}
	if a.Prerelease != "" && b.Prerelease == "" {
		return -1
	}
	x, y := a.Prerelease, b.Prerelease
	for x != "" || y != "" {
		if x == "" {
			return -1
		}
		if y == "" {
			return 1
		}
		xi, xr, _ := strings.Cut(x, ".")
		yi, yr, _ := strings.Cut(y, ".")
		xn, yn := numeric(xi), numeric(yi)
		var c int
		switch {
		case xn && yn:
			c = CompareNumber(xi, yi)
		case xn:
			c = -1
		case yn:
			c = 1
		default:
			c = strings.Compare(xi, yi)
		}
		if c != 0 {
			return c
		}
		x, y = xr, yr
	}
	return 0
}
