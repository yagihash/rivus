package model

import (
	"strings"
	"testing"
)

func parseRules(t *testing.T, src string) *Rules {
	t.Helper()
	rs, err := Parse(strings.NewReader(src), "test")
	if err != nil {
		t.Fatal(err)
	}
	return Merge(rs)
}

func TestWithNamedForm(t *testing.T) {
	rs := parseRules(t, `
ruleset: t
propagators:
  - symbol: a.Each
    calls: [{fn: arg0, with: [{param: 1, from: recv}]}]
  - symbol: a.Both
    calls: [{fn: arg0, with: [{param: 0, from: arg1}, {param: 1, from: none}]}]
  - symbol: a.Old
    calls: [{fn: arg0, with: [arg1, recv]}]
`)
	var got [][]string
	for _, p := range rs.Sets[0].Propagators {
		got = append(got, p.Calls[0].With)
	}
	want := [][]string{{"", "recv"}, {"arg1", ""}, {"arg1", "recv"}}
	for i := range want {
		if strings.Join(got[i], ",") != strings.Join(want[i], ",") || len(got[i]) != len(want[i]) {
			t.Errorf("row %d: with = %q, want %q", i, got[i], want[i])
		}
	}
	for _, bad := range []string{
		`with: [{param: 0, from: recv}, arg1]`,
		`with: [{param: 0}]`,
		`with: [{param: 0, from: recv}, {param: 0, from: arg1}]`,
		`with: [{param: 0, from: recv, extra: 1}]`,
		`with: [{param: -1, from: recv}]`,
		`with: recv`,
		// from is a position or none: a list or a mapping, and an empty value, were read as none (the parameter left empty, with no word)
		`with: [{param: 0, from: [recv]}]`,
		`with: [{param: 0, from: {a: b}}]`,
		`with: [{param: 0, from: }]`,
		`with: [{param: 0, from: ""}]`,
		// a parameter given twice is found also when the first gave none (it left "" in the list)
		`with: [{param: 0, from: none}, {param: 0, from: recv}]`,
		`with: [{param: 0, from: none}, {param: 0, from: none}]`,
	} {
		_, err := Parse(strings.NewReader("ruleset: t\npropagators:\n  - symbol: a.F\n    calls: [{fn: arg0, "+bad+"}]\n"), "test")
		if err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}

func TestCallWarnings(t *testing.T) {
	rs := parseRules(t, `
ruleset: t
propagators:
  - symbol: sort.Slice
    calls: [{fn: arg1, params: 2, with: [arg0]}]
  - symbol: strings.Map
    calls: [{fn: arg0, with: [arg1]}]
  - symbol: strings.Map
    calls: [{fn: arg0, params: 1, with: [arg1, arg1]}]
  - symbol: a.Handle
    calls: [{fn: arg0, params: 2, with: [recv]}]
  - symbol: a.Handle2
    calls: [{fn: arg0, params: 2, with: [{param: 1, from: recv}]}]
  - symbol: a.Handle3
    calls: [{fn: arg0, params: 2, with: [{param: 1, from: recv}, {param: 2, from: arg1}]}]
  - symbol: a.Unknown
    calls: [{fn: arg0, with: [recv]}]
`)
	got := rs.CallWarnings()
	if len(got) != 4 {
		t.Fatalf("warnings = %q, want 4 (sort.Slice, the second strings.Map, a.Handle, a.Handle3)", got)
	}
	for i, sym := range []string{"sort.Slice", "strings.Map", "a.Handle", "a.Handle3"} {
		if !strings.Contains(got[i], sym+" ") {
			t.Errorf("warning %d = %q, want it to name %s", i, got[i], sym)
		}
	}
}

// "params not written" and `params: 0` are different: a row that does not say params is not checked, and a row
// that says 0 (a callback with no parameters, `(*sync.Once).Do`) is warned of for any `with`.
func TestCallWarningsParamsZero(t *testing.T) {
	rs := parseRules(t, `
ruleset: t
propagators:
  - symbol: a.NotWritten
    calls: [{fn: arg0, with: [recv]}]
  - symbol: a.Zero
    calls: [{fn: arg0, params: 0, with: [recv]}]
  - symbol: a.ZeroNoWith
    calls: [{fn: arg0, params: 0}]
`)
	if rs.Sets[0].Propagators[0].Calls[0].Params != nil {
		t.Errorf("a row without params has Params = %d, want nil", *rs.Sets[0].Propagators[0].Calls[0].Params)
	}
	if p := rs.Sets[0].Propagators[1].Calls[0].Params; p == nil || *p != 0 {
		t.Errorf("`params: 0` is read as %v, want a written 0", p)
	}
	got := rs.CallWarnings()
	if len(got) != 1 || !strings.Contains(got[0], "a.Zero ") || !strings.Contains(got[0], "has 1 entries but the callback has 0 parameters") {
		t.Fatalf("warnings = %q, want one, for a.Zero (1 entry, 0 parameters)", got)
	}
}

// A negative `params` is an error at the load, like the other mistakes of the format.
func TestCallParamsNegativeIsAnError(t *testing.T) {
	_, err := Parse(strings.NewReader("ruleset: t\npropagators:\n  - symbol: a.F\n    calls: [{fn: arg0, params: -1, with: [recv]}]\n"), "test")
	if err == nil || !strings.Contains(err.Error(), "params") {
		t.Fatalf("err = %v, want an error about params", err)
	}
}
