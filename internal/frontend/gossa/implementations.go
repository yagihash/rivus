package gossa

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// Implementations finds the methods that implement an interface method.
//
// The symbol has the form "(pkg/path.Interface).Method". The search covers
// the packages that were loaded from source: dependencies are boundaries
// (ADR-015) and bring their own models. The result lists the symbols of the
// concrete methods, such as "(*example.com/m.Handler).ServeHTTP".
func (p *Program) Implementations(symbol string) ([]string, error) {
	typeStr, method, ok := splitMethodSymbol(symbol)
	if !ok {
		return nil, fmt.Errorf("%q is not the symbol of an interface method, which looks like \"(pkg.Interface).Method\"", symbol)
	}
	var named types.Type
	if dot := strings.LastIndex(typeStr, "."); dot < 0 {
		// A type of the universe, such as error, has no package.
		obj, ok := types.Universe.Lookup(typeStr).(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("%q: %s is not a type", symbol, typeStr)
		}
		named = obj.Type()
	} else {
		pkgPath, name := typeStr[:dot], typeStr[dot+1:]
		pkg := p.SSA.ImportedPackage(pkgPath)
		if pkg == nil {
			return nil, fmt.Errorf("%q: the package %s is not loaded", symbol, pkgPath)
		}
		mem, ok := pkg.Members[name].(*ssa.Type)
		if !ok {
			return nil, fmt.Errorf("%q: %s.%s is not a type", symbol, pkgPath, name)
		}
		named = mem.Type()
	}
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("%q: %s is not an interface", symbol, typeStr)
	}
	var target *types.Func
	for i := 0; i < iface.NumMethods(); i++ {
		if m := iface.Method(i); m.Name() == method {
			target = m
		}
	}
	if target == nil {
		return nil, fmt.Errorf("%q: the interface has no method %s", symbol, method)
	}

	found := map[string]bool{}
	for _, op := range p.OwnPkgs {
		names := make([]string, 0, len(op.Members))
		for n := range op.Members {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			tm, ok := op.Members[n].(*ssa.Type)
			if !ok {
				continue
			}
			t := tm.Type()
			if types.IsInterface(t) {
				continue
			}
			if named, ok := t.(*types.Named); ok && named.TypeParams().Len() > 0 {
				continue // a generic type has no single method set
			}
			for _, cand := range []types.Type{t, types.NewPointer(t)} {
				if !types.Implements(cand, iface) {
					continue
				}
				sel := types.NewMethodSet(cand).Lookup(target.Pkg(), method)
				if sel == nil {
					continue
				}
				if fn, ok := sel.Obj().(*types.Func); ok {
					found[fn.FullName()] = true
				}
			}
		}
	}
	out := make([]string, 0, len(found))
	for s := range found {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

// splitMethodSymbol splits "(pkg.Type).Method" into "pkg.Type" and "Method".
func splitMethodSymbol(symbol string) (typeStr, method string, ok bool) {
	if !strings.HasPrefix(symbol, "(") || strings.HasPrefix(symbol, "(*") {
		return "", "", false
	}
	end := strings.Index(symbol, ").")
	if end < 0 {
		return "", "", false
	}
	return symbol[1:end], symbol[end+2:], true
}
