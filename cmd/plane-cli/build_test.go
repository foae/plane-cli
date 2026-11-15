package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildIdentity(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567"
	module := func(version string, settings ...debug.BuildSetting) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: version}, Settings: settings}
	}
	modified := debug.BuildSetting{Key: "vcs.modified", Value: "true"}
	clean := debug.BuildSetting{Key: "vcs.modified", Value: "false"}
	vcs := debug.BuildSetting{Key: "vcs.revision", Value: hash}
	replaced := module("v1.2.3")
	replaced.Main.Replace = &debug.Module{Path: "../local"}
	tests := []struct {
		name, version, commit   string
		info                    *debug.BuildInfo
		wantVersion, wantCommit string
	}{
		{"missing metadata", "dev", "unknown", nil, "dev", "unknown"},
		{"installed tag", "dev", "unknown", module("v1.2.3"), "v1.2.3", "unknown"},
		{"installed prerelease", "dev", "unknown", module("v1.2.3-rc.1"), "v1.2.3-rc.1", "unknown"},
		{"installed pseudo-version", "dev", "unknown", module("v0.0.0-20261115120000-0123456789ab"), "dev", "unknown"},
		{"installed prerelease pseudo-version", "dev", "unknown", module("v1.2.3-rc.1.0.20261115120000-0123456789ab"), "dev", "unknown"},
		{"noncanonical module", "dev", "unknown", module("1.2.3"), "dev", "unknown"},
		{"devel", "dev", "unknown", module("(devel)", clean, vcs), "dev", hash},
		{"local tagged checkout", "dev", "unknown", module("v1.2.3", clean, vcs), "dev", hash},
		{"replaced", "dev", "unknown", replaced, "dev", "unknown"},
		{"modified installed", "dev", "unknown", module("v1.2.3", modified, vcs), "dev", hash},
		{"modified injected", "v1.2.3", hash, module("(devel)", modified), "dev", hash},
		{"injected", "v2.3.4-rc.2", "release-commit", module("v1.2.3", clean, vcs), "v2.3.4-rc.2", "release-commit"},
		{"injected no metadata", "v2.3.4", hash, nil, "v2.3.4", hash},
		{"invalid injection", "v01.2.3", "unknown", nil, "dev", "unknown"},
		{"unsafe vcs", "dev", "unknown", module("(devel)", debug.BuildSetting{Key: "vcs.revision", Value: "bad\nrevision"}), "dev", "unknown"},
		{"empty injected", "", "", module("(devel)", vcs), "dev", hash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, c := buildIdentity(tt.version, tt.commit, tt.info)
			if v != tt.wantVersion || c != tt.wantCommit {
				t.Fatalf("buildIdentity = (%q, %q), want (%q, %q)", v, c, tt.wantVersion, tt.wantCommit)
			}
		})
	}
}

func TestReleaseVersion(t *testing.T) {
	for _, value := range []string{"0.0.0", "1.2.3", "1.2.3-alpha.1", "1.2.3-rc-1"} {
		if !releaseVersion(value) {
			t.Errorf("rejected canonical version %q", value)
		}
	}
	for _, value := range []string{"", "v1.2.3", "1.2", "01.2.3", "1.02.3", "1.2.03", "1.2.3-", "1.2.3-01", "1.2.3-a..b", "1.2.3+meta", "1.2.3-a+meta", "1.2.3-a/b"} {
		if releaseVersion(value) {
			t.Errorf("accepted noncanonical version %q", value)
		}
	}
}
