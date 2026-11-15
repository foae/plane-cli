package main

import (
	"runtime/debug"
	"strings"
)

// buildIdentity uses only embedded metadata. Release builds inject version and
// commit through -X main.version and -X main.commit; local builds stay dev.
func buildIdentity(injectedVersion, injectedCommit string, info *debug.BuildInfo) (string, string) {
	v, c := "dev", "unknown"
	if releaseVersion(strings.TrimPrefix(injectedVersion, "v")) {
		v = injectedVersion
	}
	if injectedCommit != "" && injectedCommit != "unknown" {
		c = injectedCommit
	}
	if info == nil {
		return v, c
	}
	modified := false
	localBuild := false
	for _, setting := range info.Settings {
		localBuild = localBuild || setting.Key == "vcs" || strings.HasPrefix(setting.Key, "vcs.")
		switch setting.Key {
		case "vcs.modified":
			modified = modified || setting.Value != "false"
		case "vcs.revision":
			if c == "unknown" && revision(setting.Value) {
				c = setting.Value
			}
		}
	}
	if modified {
		return "dev", c
	}
	// Local Go builds also derive Main.Version from tags; only module installs
	// without VCS metadata may use the fallback instead of validated injection.
	if v == "dev" && !localBuild && info.Main.Replace == nil && strings.HasPrefix(info.Main.Version, "v") && releaseVersion(strings.TrimPrefix(info.Main.Version, "v")) && !pseudoVersion(info.Main.Version) {
		v = info.Main.Version
	}
	return v, c
}

func pseudoVersion(value string) bool {
	// Go pseudo-versions end in a timestamp and twelve-character revision.
	_, tail, ok := strings.Cut(value, "-")
	if !ok {
		return false
	}
	last := strings.LastIndexByte(tail, '-')
	if last < 0 || len(tail[last+1:]) != 12 || !revision(tail[last+1:]) {
		return false
	}
	stamp := tail[:last]
	if dot := strings.LastIndexByte(stamp, '.'); dot >= 0 {
		stamp = stamp[dot+1:]
	}
	return len(stamp) == 14 && decimal(stamp)
}

// releaseVersion deliberately excludes build metadata and noncanonical numbers.
func releaseVersion(value string) bool {
	core, pre, hasPre := strings.Cut(value, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !decimal(part) || len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	if !hasPre {
		return true
	}
	for _, part := range strings.Split(pre, ".") {
		if part == "" || decimal(part) && len(part) > 1 && part[0] == '0' {
			return false
		}
		for _, ch := range part {
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && ch != '-' {
				return false
			}
		}
	}
	return true
}

func decimal(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func revision(value string) bool {
	if len(value) < 7 || len(value) > 64 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}
