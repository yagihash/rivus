package builtin_test

import (
	"go/types"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

func TestBuiltinRulesAreValid(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) == 0 {
		t.Fatal("no built-in rule sets")
	}
	r := model.Merge(sets...)
	for _, sym := range []string{"os/exec.Command", "(*database/sql.DB).Query", "(net/http.ResponseWriter).Write"} {
		if len(r.Sinks[sym]) == 0 {
			t.Errorf("built-in sink %s missing", sym)
		}
	}
	if len(r.ParamTypes) == 0 {
		t.Error("built-in param_type source missing")
	}
}

// A row of calls of the built-in models fills the parameters of its callback (a `with` that does not fit is a
// silent loss of the flow).
func TestBuiltinCallRowsFitTheirCallbacks(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range model.Merge(sets...).CallWarnings() {
		t.Error(w)
	}
}

// Every symbol of the Go standard library that a built-in rule names must be one that a call can have
// (yagihash/rivus #53): a method that the type only gets by embedding is called by the symbol of the type that
// declares it, and a field is not called.
func TestBuiltinStdlibSymbolsExist(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	rules := model.Merge(sets...)
	re := regexp.MustCompile(`^(?:\((\*?)([^)]+)\)\.([A-Za-z0-9_]+)|([^.()]+(?:/[^.()]+)*)\.([A-Za-z0-9_]+))$`)
	type want struct{ sym, pkg, typ, name string }
	var wants []want
	pkgs := map[string]bool{}
	for _, sym := range rules.Symbols() {
		m := re.FindStringSubmatch(sym)
		if m == nil || strings.ContainsAny(sym, ":#") {
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
			continue // not the standard library
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
	for _, w := range wants {
		pkg := byPath[w.pkg]
		if pkg == nil {
			continue // not in this Go version
		}
		if w.typ == "" {
			if pkg.Scope().Lookup(w.name) == nil {
				t.Errorf("%s: %s is not in %s", w.sym, w.name, w.pkg)
			}
			continue
		}
		obj := pkg.Scope().Lookup(w.typ)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			t.Errorf("%s: %s is not a type of %s", w.sym, w.typ, w.pkg)
			continue
		}
		start := tn.Type()
		if !types.IsInterface(start) {
			start = types.NewPointer(start)
		}
		found, _, _ := types.LookupFieldOrMethod(start, true, pkg, w.name)
		fn, ok := found.(*types.Func)
		if !ok {
			t.Errorf("%s: %s is not a method of %s (a field, or nothing)", w.sym, w.name, w.typ)
			continue
		}
		if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
			rt := recv.Type()
			if p, ok := rt.(*types.Pointer); ok {
				rt = p.Elem()
			}
			if n, ok := rt.(*types.Named); ok && n.Obj() != tn && !types.IsInterface(tn.Type()) {
				t.Errorf("%s: %s is declared by %s: a call has that symbol", w.sym, w.name, n.Obj().Name())
			}
			if n, ok := rt.(*types.Named); ok && n.Obj() != tn && types.IsInterface(tn.Type()) {
				t.Errorf("%s: %s is embedded from %s.%s: a call has that symbol", w.sym, w.name, n.Obj().Pkg().Path(), n.Obj().Name())
			}
		}
	}
}

// A `deferred:` entry names an issue that exists (docs/issues): the question that keeps it open is written there.
func TestDeferredRefsExist(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, rs := range sets {
		for _, d := range rs.Deferred {
			pat := filepath.Join("..", "..", "..", "docs", "issues", model.DeferredRefFile(d.Ref))
			if m, _ := filepath.Glob(pat); len(m) == 0 {
				t.Errorf("%s: deferred %s: ref %s has no file (%s)", rs.Origin, d.Symbol, d.Ref, pat)
			}
		}
	}
}
