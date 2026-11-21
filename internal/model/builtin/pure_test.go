package builtin_test

import (
	"go/types"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// The criterion for `pure: true` (docs/MODELING.md, "When a function may be pure"): the result is a
// number, a bool, nothing, an error, or does not depend on the arguments. A function of the standard
// library that is `pure` and returns a string, bytes, a slice, a map, a struct, a pointer or an
// interface is listed here, and every one of them was read and found not to carry its arguments
// (it returns a clock, a handle, a fixed value, or a value made by the runtime). The list is fixed:
// a function that is added to `pure` with such a result fails this test until it is read and added
// to the file; one that was removed (a flow was written) fails until it is removed from the file.
//
// The file is testdata/pure-carriers.txt: one symbol per line, a comment after `#`.
func TestPureFunctionsThatReturnDataAreReviewed(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	rules := model.Merge(sets...)
	re := regexp.MustCompile(`^(?:\((\*?)([^)]+)\)\.([A-Za-z0-9_]+)|([^.()]+(?:/[^.()]+)*)\.([A-Za-z0-9_]+))$`)
	type want struct{ sym, pkg, typ, name string }
	var wants []want
	pkgs := map[string]bool{}
	for sym, ps := range rules.Propagators {
		pure := false
		for _, p := range ps {
			pure = pure || p.Pure
		}
		m := re.FindStringSubmatch(sym)
		if !pure || m == nil || strings.ContainsAny(sym, ":#") {
			continue
		}
		var w want
		if m[2] != "" {
			i := strings.LastIndex(m[2], ".")
			if i < 0 {
				continue
			}
			w = want{sym, m[2][:i], m[2][i+1:], m[3]}
		} else {
			w = want{sym, m[4], "", m[5]}
		}
		if first, _, _ := strings.Cut(w.pkg, "/"); strings.ContainsAny(first, ".:#") {
			continue
		}
		wants = append(wants, w)
		pkgs[w.pkg] = true
	}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	loaded, err := packages.Load(&packages.Config{Mode: packages.NeedTypes | packages.NeedName | packages.NeedImports | packages.NeedDeps}, paths...)
	if err != nil {
		t.Skipf("cannot load the standard library: %v", err)
	}
	byPath := map[string]*types.Package{}
	for _, p := range loaded {
		if p.Types != nil {
			byPath[p.PkgPath] = p.Types
		}
	}
	var carriers []string
	for _, w := range wants {
		pkg := byPath[w.pkg]
		if pkg == nil {
			continue
		}
		var fn *types.Func
		if w.typ == "" {
			fn, _ = pkg.Scope().Lookup(w.name).(*types.Func)
		} else if tn, ok := pkg.Scope().Lookup(w.typ).(*types.TypeName); ok {
			start := tn.Type()
			if !types.IsInterface(start) {
				start = types.NewPointer(start)
			}
			f, _, _ := types.LookupFieldOrMethod(start, true, pkg, w.name)
			fn, _ = f.(*types.Func)
		}
		if fn == nil {
			continue // TestBuiltinStdlibSymbolsExist says it
		}
		res := fn.Type().(*types.Signature).Results()
		for i := 0; i < res.Len(); i++ {
			if carriesData(res.At(i).Type()) {
				carriers = append(carriers, w.sym)
				break
			}
		}
	}
	sort.Strings(carriers)

	raw, err := os.ReadFile("testdata/pure-carriers.txt")
	if err != nil {
		t.Fatal(err)
	}
	reviewed := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line != "" {
			reviewed[line] = true
		}
	}
	got := map[string]bool{}
	for _, c := range carriers {
		got[c] = true
		if !reviewed[c] {
			t.Errorf("%s is pure and returns data: read whether the result can hold an argument (docs/MODELING.md, \"When a function may be pure\"); write a flow if it can, add it to testdata/pure-carriers.txt if not", c)
		}
	}
	for r := range reviewed {
		if !got[r] {
			t.Errorf("%s is in testdata/pure-carriers.txt but is not a pure function that returns data any more: remove it", r)
		}
	}
}

// carriesData: a value of the type can hold a string, bytes or a structure that came from an argument.
// Numbers, bools and errors do not count (an error is told by the uncertainty of the call; the criterion is
// written for the data a function makes).
func carriesData(t types.Type) bool {
	if n, ok := t.(*types.Named); ok && n.Obj().Pkg() == nil && n.Obj().Name() == "error" {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Interface:
		return !isErrorType(t)
	case *types.Struct, *types.Slice, *types.Map, *types.Pointer, *types.Chan, *types.Array, *types.Signature:
		return true
	case *types.Tuple:
		return true
	}
	return false
}

func isErrorType(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}
