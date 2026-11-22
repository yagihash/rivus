package model

import (
	"sort"
	"strings"
	"testing"
)

func TestMatchMajor(t *testing.T) {
	for _, c := range []struct {
		pattern, sym string
		want         bool
	}{
		{"x.org/m/v*/pkg.F", "x.org/m/v2/pkg.F", true},
		{"x.org/m/v*/pkg.F", "x.org/m/v93/pkg.F", true},
		{"x.org/m/v*/pkg.F", "x.org/m/v10/pkg.F", true},
		{"(*x.org/m/v*/pkg.T).M", "(*x.org/m/v3/pkg.T).M", true},
		{"x.org/m/v*.F", "x.org/m/v5.F", true},              // the root package of the module
		{"x.org/m/v*/pkg.F", "x.org/m/vendor/pkg.F", false}, // not v and digits
		{"x.org/m/v*/pkg.F", "x.org/m/v/pkg.F", false},      // no digits
		{"x.org/m/v*/pkg.F", "x.org/m/v2x/pkg.F", false},    // digits and more
		{"x.org/m/v*/pkg.F", "x.org/m/v2/sub/pkg.F", false}, // one element only
		{"x.org/m/v*/pkg.F", "x.org/m/pkg.F", false},        // no major in the path
		{"x.org/m/v*/pkg.F", "x.org/m/v1/pkg.F", false},     // v0 and v1 have no major element in a path
		{"x.org/m/v*/pkg.F", "x.org/m/v0/pkg.F", false},
		{"x.org/m/v*/pkg.F", "x.org/m/v02/pkg.F", false},
		{"x.org/m/v*/pkg.F", "x.org/m/v2/pkg.G", false},
		{"x.org/m/v*/pkg.F", "y.org/m/v2/pkg.F", false},
		{"x.org/m/pkg.F", "x.org/m/pkg.F", true}, // without /v*: the symbol itself, as before
		{"x.org/m/pkg.F", "x.org/m/v2/pkg.F", false},
	} {
		if got := MatchMajor(c.pattern, c.sym); got != c.want {
			t.Errorf("MatchMajor(%q, %q) = %v, want %v", c.pattern, c.sym, got, c.want)
		}
	}
}

func TestCheckMajorWildcard(t *testing.T) {
	for _, ok := range []string{"x.org/m/v*/pkg.F", "(*x.org/m/v*/pkg.T).M", "x.org/m/v*.F", "x.org/m/v*", "x.org/m/pkg.F", "(*x.org/m/pkg.T).M", "strings.Join"} {
		if err := CheckMajorWildcard(ok, "t"); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for bad, why := range map[string]string{
		"x.org/m/v*/sub/v*/pkg.F": "more than once",
		"x.org/v*x/pkg.F":         "whole path element",
		"x.org/m/v*2/pkg.F":       "whole path element",
		"/v*/pkg.F":               "after the path of the module",
		"x.org/m//v*/pkg.F":       "after the path of the module",
	} {
		err := CheckMajorWildcard(bad, "t")
		if err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%q: error %v, want one with %q", bad, err, why)
		}
	}
}

func TestMajorWildcardInRules(t *testing.T) {
	good := `ruleset: t@1
verified_versions:
  - package: x.org/m/v*
    versions: ">=93.0.0 <94.0.0"
    checked_with: v93.0.0
    checked: 2026-11-23
sinks:
  - symbol: x.org/m/v*/pkg.F
    args: [0]
    category: command_injection
sources:
  - kind: field
    field: x.org/m/v*/pkg.T.Body
    labels: [user_input]
  - kind: call_result
    symbol: (*x.org/m/v*/pkg.T).Get
    labels: [user_input]
`
	if _, err := Parse(strings.NewReader(good), "t.yaml"); err != nil {
		t.Fatalf("good: %v", err)
	}
	for name, body := range map[string]string{
		"sink twice":   "sinks:\n  - symbol: x.org/m/v*/pkg/v*.F\n    args: [0]\n    category: c\n",
		"sink partial": "sinks:\n  - symbol: x.org/v*x/pkg.F\n    args: [0]\n    category: c\n",
		"source field": "sources:\n  - kind: field\n    field: x.org/v*x/pkg.T.B\n    labels: [l]\n",
		"verified":     "verified_versions:\n  - package: x.org/m/v*/pkg/v*\n    versions: \">=1.0.0\"\n    checked: 2026-11-23\n",
	} {
		if _, err := Parse(strings.NewReader("ruleset: t@1\n"+body), "t.yaml"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestExpandMajors(t *testing.T) {
	rs, err := Parse(strings.NewReader(`ruleset: t@1
sinks:
  - symbol: x.org/m/v*/pkg.F
    args: [0]
    category: command_injection
  - symbol: x.org/m/pkg.F
    args: [0]
    category: command_injection
sources:
  - kind: call_result
    symbol: (*x.org/m/v*/pkg.T).Get
    labels: [user_input]
  - kind: field
    field: x.org/m/v*/pkg.T.Body
    labels: [user_input]
  - kind: param_type
    type: "*x.org/m/v*/pkg.Ctx"
    labels: [user_input]
propagators:
  - symbol: x.org/m/v*/pkg.P
    flows: [{from: arg0, to: result}]
`), "t.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(rs)
	r.ExpandMajors([]string{"x.org/m/v2/pkg", "x.org/m/v3/pkg/sub", "x.org/m/vendor/pkg", "x.org/m/v1/pkg", "x.org/mm/v4/pkg", "strings"})
	keys := func(m map[string][]Sink) string {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return strings.Join(ks, " ")
	}
	if got, want := keys(r.Sinks), "x.org/m/pkg.F x.org/m/v*/pkg.F x.org/m/v2/pkg.F x.org/m/v3/pkg.F"; got != want {
		t.Errorf("sinks: %s, want %s (v2 and v3 have the program, vendor and v1 are not majors, mm is another module)", got, want)
	}
	if len(r.Sinks["x.org/m/v2/pkg.F"]) != 1 || r.Sinks["x.org/m/v2/pkg.F"][0].Symbol != "x.org/m/v2/pkg.F" {
		t.Errorf("the row of v2: %+v", r.Sinks["x.org/m/v2/pkg.F"])
	}
	if len(r.CallSources["(*x.org/m/v3/pkg.T).Get"]) != 1 || len(r.CallSources["(*x.org/m/v2/pkg.T).Get"]) != 1 {
		t.Errorf("call sources: %v", r.CallSources)
	}
	if len(r.Fields["x.org/m/v2/pkg.T.Body"]) != 1 || len(r.Fields["x.org/m/v3/pkg.T.Body"]) != 1 {
		t.Errorf("fields: %v", r.Fields)
	}
	types := map[string]bool{}
	for _, s := range r.ParamTypes {
		types[s.Type] = true
	}
	if !types["*x.org/m/v2/pkg.Ctx"] || !types["*x.org/m/v3/pkg.Ctx"] || !types["*x.org/m/v*/pkg.Ctx"] || len(types) != 3 {
		t.Errorf("param types: %v", types)
	}
	if !r.HasModel("x.org/m/v2/pkg.P") || !r.HasModel("x.org/m/v3/pkg.P") || r.HasModel("x.org/m/v4/pkg.P") {
		t.Errorf("propagators: %v", r.Propagators)
	}
	// the sets are as they were, and a second call adds nothing
	if rs.Sinks[0].Symbol != "x.org/m/v*/pkg.F" {
		t.Errorf("the set was changed: %s", rs.Sinks[0].Symbol)
	}
	r.ExpandMajors([]string{"x.org/m/v2/pkg"})
	if len(r.Sinks["x.org/m/v2/pkg.F"]) != 1 {
		t.Errorf("a second call adds the rows again")
	}
}

func TestUnverifiedVersions(t *testing.T) {
	rs, err := Parse(strings.NewReader(`ruleset: t@1
verified_versions:
  - package: x.org/m/v*
    versions: ">=93.0.0 <94.0.0"
    checked: 2026-11-23
  - package: x.org/exact
    versions: "^1.2.0"
    checked: 2026-11-23
`), "t.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(rs)
	got := r.UnverifiedVersions(map[string]string{
		"x.org/m/v93":   "v93.0.1",             // in the range
		"x.org/m/v84":   "v84.2.0",             // another major: out
		"x.org/exact":   "v1.9.0+incompatible", // in
		"x.org/unknown": "v0.1.0",              // no record: not listed
		"x.org/m/vault": "v1.0.0",              // not a major: no record
	}, "go.mod")
	if len(got) != 1 || got[0].Package != "x.org/m/v84" || got[0].Used != "v84.2.0" || got[0].Source != "go.mod" || len(got[0].Verified) != 1 || got[0].Verified[0] != ">=93.0.0 <94.0.0" {
		t.Errorf("unverified: %+v", got)
	}
	out := r.UnverifiedVersions(map[string]string{"x.org/exact": "v2.0.0", "x.org/m/v2": "v2.0.0"}, "go.mod")
	if len(out) != 2 || out[0].Package != "x.org/exact" || out[1].Package != "x.org/m/v2" {
		t.Errorf("sorted by package: %+v", out)
	}
	if (VerifiedVersion{Versions: "^1.2.0"}).Holds("garbage") {
		t.Errorf("a text that is not a version is not in a range")
	}
}
