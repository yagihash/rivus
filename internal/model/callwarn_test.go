package model

import (
	"go/importer"
	"go/token"
	"go/types"
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
    calls: [{fn: arg1, with: [arg0]}]
  - symbol: strings.Map
    calls: [{fn: arg0, with: [arg1]}]
  - symbol: strings.Map
    calls: [{fn: arg0, with: [arg1, arg1]}]
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

// The table of known callbacks is the signatures of the standard library.
func TestCallbackParamsKnown(t *testing.T) {
	imp := importer.ForCompiler(token.NewFileSet(), "source", nil)
	for key, n := range knownCallbackParams {
		slash := strings.LastIndex(key, "/")
		sym, fn := key[:slash], key[slash+1:]
		var sig *types.Signature
		if strings.HasPrefix(sym, "(*") {
			pkgName, rest, _ := strings.Cut(strings.TrimPrefix(sym, "(*"), ".")
			typ, method, _ := strings.Cut(rest, ").")
			pkg, err := imp.Import(pkgName)
			if err != nil {
				t.Fatal(err)
			}
			named := pkg.Scope().Lookup(typ).Type()
			obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(named), true, pkg, method)
			sig = obj.(*types.Func).Type().(*types.Signature)
		} else {
			pkgName, name, _ := strings.Cut(sym, ".")
			pkg, err := imp.Import(pkgName)
			if err != nil {
				t.Fatal(err)
			}
			sig = pkg.Scope().Lookup(name).Type().(*types.Signature)
		}
		idx := int(fn[len(fn)-1] - '0')
		cb, ok := sig.Params().At(idx).Type().Underlying().(*types.Signature)
		if !ok {
			t.Errorf("%s: parameter %d is not a function", key, idx)
			continue
		}
		if cb.Params().Len() != n {
			t.Errorf("%s: the callback has %d parameters, the table says %d", key, cb.Params().Len(), n)
		}
	}
}
