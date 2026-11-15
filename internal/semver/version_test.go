package semver

import "testing"

func TestSemVer(t *testing.T) {
	ordered := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.9.0", "1.10.0", "2.0.0", "999999999999999999999999999999999999999.0.0"}
	for i, raw := range ordered {
		v, err := Parse(raw)
		if err != nil || Compare(v, v) != 0 {
			t.Fatalf("parse %q: %+v %v", raw, v, err)
		}
		if i > 0 {
			previous, err := Parse(ordered[i-1])
			if err != nil || Compare(previous, v) >= 0 || Compare(v, previous) <= 0 {
				t.Fatalf("ordering %q < %q", ordered[i-1], raw)
			}
		}
	}
	for _, raw := range []string{"", "v1.0.0", "1", "1.0", "1.0.0.0", "01.0.0", "1.01.0", "1.0.01", "1.0.0-", "1.0.0+", "1.0.0-a..b", "1.0.0-01", "1.0.0-alpha.01", "1.0.0+one+two", "1.0.0+bad_thing", "1.0.0-β", " 1.0.0", "1.0.0\n"} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("accepted invalid %q", raw)
		}
	}
	base, err := Parse("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"1.0.0+build.01", "1.0.0+different-build"} {
		v, err := Parse(raw)
		if err != nil || Compare(base, v) != 0 {
			t.Fatalf("build metadata changed precedence: %q %v", raw, err)
		}
	}
}
