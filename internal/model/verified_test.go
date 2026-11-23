package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func parseVerified(t *testing.T, body string) (*RuleSet, error) {
	t.Helper()
	return Parse(strings.NewReader("ruleset: t@1\nverified_versions:\n"+body), "t.yaml")
}

func TestVerifiedVersionsAreRead(t *testing.T) {
	rs, err := parseVerified(t, `  - package: shelf_router
    versions: ">=1.1.0 <2.0.0"
    checked_with: 1.1.5
    checked: 2026-11-22
  - package: github.com/x/y/v2
    versions: v2.1.0
    checked_with: v2.1.0
    checked: 2026-11-01
  - package: serde
    versions: ^1.0.100
    checked: 2026-11-01
  - package: http
    versions: "~1.2"
    checked_with: 1.2.9
    checked: 2026-11-01
  - package: a
    versions: ^0.2.3
    checked_with: 0.2.9
    checked: 2026-11-01
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.VerifiedVersions) != 5 || rs.VerifiedVersions[0].CheckedWith != "1.1.5" || rs.VerifiedVersions[0].Checked != "2026-11-22" {
		t.Fatalf("%+v", rs.VerifiedVersions)
	}
	// a record is not a declaration, and it changes no row
	if r := Merge(rs); len(r.Verified) != 5 || r.Verified[0].Package != "a" || r.Verified[0].Source != "t.yaml" || r.HasDeclared() {
		t.Fatalf("%+v", r.Verified)
	}
}

func TestVerifiedVersionsAreChecked(t *testing.T) {
	for name, c := range map[string]struct{ body, want string }{
		"no package":     {"  - versions: ^1.0.0\n    checked: 2026-11-01\n", "package is required"},
		"a pattern":      {"  - package: a/...\n    versions: ^1.0.0\n    checked: 2026-11-01\n", "not a pattern"},
		"empty versions": {"  - package: a\n    checked: 2026-11-01\n", "range is empty"},
		"bad versions":   {"  - package: a\n    versions: latest\n    checked: 2026-11-01\n", "not a version"},
		"no day":         {"  - package: a\n    versions: ^1.0.0\n", "checked is required"},
		"bad day":        {"  - package: a\n    versions: ^1.0.0\n    checked: yesterday\n", "not a day"},
		"future":         {"  - package: a\n    versions: ^1.0.0\n    checked: 2999-01-01\n", "future"},
		"with a range":   {"  - package: a\n    versions: ^1.0.0\n    checked_with: ^1.0.0\n    checked: 2026-11-01\n", "not one version"},
		"outside":        {"  - package: a\n    versions: \">=1.0.0 <2.0.0\"\n    checked_with: 2.0.0\n    checked: 2026-11-01\n", "outside"},
		"outside caret":  {"  - package: a\n    versions: ^0.2.3\n    checked_with: 0.3.0\n    checked: 2026-11-01\n", "outside"},
		"twice":          {"  - package: a\n    versions: ^1.0.0\n    checked: 2026-11-01\n  - package: a\n    versions: ^2.0.0\n    checked: 2026-11-01\n", "recorded twice"},
		"unknown key":    {"  - package: a\n    versions: ^1.0.0\n    checked: 2026-11-01\n    note: x\n", "field note not found"},
	} {
		if _, err := parseVerified(t, c.body); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", name, c.want, err)
		}
	}
	// no record at all is fine, an empty list too, and the package need not be one the rows name
	if _, err := Parse(strings.NewReader("ruleset: t@1\nverified_versions: []\n"), "t.yaml"); err != nil {
		t.Error(err)
	}
	if _, err := parseVerified(t, "  - package: no_row_names_this\n    versions: 1.0.0\n    checked: 2026-11-01\n"); err != nil {
		t.Error(err)
	}
}

// A package at several versions is one entry when any of them is outside the ranges: Used is the first one outside, AllUsed lists
// them; a package at one version has no all_used (the JSON of Go is what it was).
func TestUnverifiedVersionsOfSeveral(t *testing.T) {
	rs, err := Parse(strings.NewReader("ruleset: t@1\nverified_versions:\n  - package: ab\n    versions: \">=1.0.0 <2.0.0\"\n    checked: 2026-11-23\n"), "t.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(rs)
	got := r.UnverifiedVersionsOf(map[string][]string{"ab": {"1.2.0", "0.9.0", "0.8.0"}}, "dart", "lock", SamePackage)
	want := []UnverifiedVersion{{Package: "ab", Used: "0.9.0", AllUsed: []string{"1.2.0", "0.9.0", "0.8.0"}, Verified: []string{">=1.0.0 <2.0.0"}, Source: "lock"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("several: %v, want %v", got, want)
	}
	if got := r.UnverifiedVersionsOf(map[string][]string{"ab": {"1.2.0", "1.9.9"}}, "dart", "lock", SamePackage); len(got) != 0 {
		t.Errorf("all in the range: %v", got)
	}
	if got := r.UnverifiedVersionsOf(map[string][]string{"a_b": {"3.0.0"}}, "dart", "lock", SamePackage); len(got) != 0 {
		t.Errorf("names are as written: %v", got)
	}
	b, _ := json.Marshal(r.UnverifiedVersions(map[string]string{"ab": "3.0.0"}, "go.mod"))
	if strings.Contains(string(b), "all_used") {
		t.Errorf("one version has no all_used: %s", b)
	}
}

// Rust writes - and _ in a crate name as the same: the record and the lock file may spell it either way.
func TestSameCrate(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"actix-web", "actix-web", true}, {"google-cloud-storage", "google_cloud_storage", true}, {"a_b", "a-b", true}, {"ab", "a-b", false}} {
		if got := SameCrate(c.a, c.b); got != c.want {
			t.Errorf("SameCrate(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
