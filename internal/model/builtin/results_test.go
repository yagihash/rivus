package builtin_test

import (
	"go/types"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/yagihash/rivus/internal/model/builtin"
)

// `to: result` is the first result only (result0); every result is `to: results`. A function that returns
// `(value, error)` and has a row `{from: args, to: result}` does not carry its arguments into the error, and
// `err.Error()` of it reaches no sink (docs/MODELING.md, "result and results"). This test lists every row of the
// Go rule sets that says `to: result` for a function with more than one result, from the signature of the function
// (the standard library, read by go/packages). A row that is meant (the error does not hold the arguments: read the
// function and say why) is in testdata/result-first-only.txt, one symbol per line, a reason after `#`. The list is
// fixed in both directions: a row that is not meant fails until it says `results`, a line of the file that has no row
// any more (or a function that no longer has more than one result) fails until it is removed.
//
// Dart has one result per function (a Record is one value), and Rust has one result per call (`Result<T, E>` and a
// tuple, `unzip`, `into_parts`, are each one value): the rows of those sets carry no signature, and `to: result` is
// the whole value there.
func TestGoRowsThatSayResultAreForOneResult(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^(?:\((\*?)([^)]+)\)\.([A-Za-z0-9_]+)|([^.()]+(?:/[^.()]+)*)\.([A-Za-z0-9_]+))$`)
	type want struct{ sym, pkg, typ, name string }
	var wants []want
	seen := map[string]bool{}
	pkgs := map[string]bool{}
	for _, rs := range sets {
		if !strings.HasPrefix(rs.Name, "rivus/go/") {
			continue
		}
		for _, p := range rs.Propagators {
			first := false
			for _, f := range p.Flows {
				first = first || f.To == "result"
			}
			m := re.FindStringSubmatch(p.Symbol)
			if !first || m == nil || strings.ContainsAny(p.Symbol, ":#") || seen[p.Symbol] {
				continue
			}
			seen[p.Symbol] = true
			var w want
			if m[2] != "" {
				i := strings.LastIndex(m[2], ".")
				if i < 0 {
					continue
				}
				w = want{p.Symbol, m[2][:i], m[2][i+1:], m[3]}
			} else {
				w = want{p.Symbol, m[4], "", m[5]}
			}
			wants = append(wants, w)
			pkgs[w.pkg] = true
		}
	}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	// The types of the packages themselves, from the export data the go command keeps in its cache: no
	// dependency is type-checked from source.
	loaded, err := packages.Load(&packages.Config{Mode: packages.NeedTypes | packages.NeedName | packages.NeedExportFile}, paths...)
	if err != nil {
		t.Skipf("cannot load the packages: %v", err)
	}
	byPath := map[string]*types.Package{}
	for _, p := range loaded {
		if p.Types != nil {
			byPath[p.PkgPath] = p.Types
		}
	}
	multi := map[string]int{}
	for _, w := range wants {
		pkg := byPath[w.pkg]
		if pkg == nil {
			t.Errorf("%s: package %s could not be loaded", w.sym, w.pkg)
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
		if n := fn.Type().(*types.Signature).Results().Len(); n > 1 {
			multi[w.sym] = n
		}
	}

	raw, err := os.ReadFile("testdata/result-first-only.txt")
	if err != nil {
		t.Fatal(err)
	}
	meant := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line != "" {
			meant[line] = true
		}
	}
	var syms []string
	for s := range multi {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	for _, s := range syms {
		if !meant[s] {
			t.Errorf("%s returns %d results and a row says `to: result` (the first result only): write `to: results`, or, if the other results never hold the arguments (the error does not quote them), add it to testdata/result-first-only.txt with the reason", s, multi[s])
		}
	}
	for s := range meant {
		if multi[s] == 0 {
			t.Errorf("%s is in testdata/result-first-only.txt but has no `to: result` row for a function with more than one result any more: remove it", s)
		}
	}
}
