package gossa

import (
	"fmt"
	"go/ast"
	"sort"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// addUnread records the packages that were loaded and not built. go/ssa builds nothing for a package
// that does not type-check, and nothing for a package that imports one (go/packages marks it IllTyped
// too): they stay in Initial, and what they hold is not in the analysis. A package with a type error
// in one line takes every package that depends on it with it, so each is told, with the number of its
// functions and the package whose error it is (the result is not the analysis of the code, but of what
// built).
func (p *Program) addUnread(pkgs []*packages.Package, built []*ssa.Package) {
	var out []LoadError
	for i, pkg := range pkgs {
		if i < len(built) && built[i] != nil {
			continue
		}
		n := declaredFuncs(pkg)
		cause := "the package does not type-check: " + firstError(pkg)
		if len(pkg.Errors) == 0 {
			if root := rootCause(pkg, map[*packages.Package]bool{}); root != nil {
				cause = fmt.Sprintf("it depends on %s, which does not type-check: %s", root.PkgPath, firstError(root))
			} else {
				cause = "go/packages marked it as not type-checked"
			}
		}
		out = append(out, LoadError{
			Package:   pkg.PkgPath,
			Message:   fmt.Sprintf("not analysed: %s; its %d function(s) are not in the result", cause, n),
			Unread:    true,
			Functions: n,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	p.LoadErrors = append(p.LoadErrors, out...)
}

// rootCause finds a package with an error of its own among the packages that pkg imports (the first
// in the order of the import paths).
func rootCause(pkg *packages.Package, seen map[*packages.Package]bool) *packages.Package {
	if seen[pkg] {
		return nil
	}
	seen[pkg] = true
	paths := make([]string, 0, len(pkg.Imports))
	for path := range pkg.Imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		imp := pkg.Imports[path]
		if !imp.IllTyped {
			continue
		}
		if len(imp.Errors) > 0 {
			return imp
		}
		if r := rootCause(imp, seen); r != nil {
			return r
		}
	}
	return nil
}

func firstError(pkg *packages.Package) string {
	if len(pkg.Errors) == 0 {
		return "no error was reported"
	}
	return pkg.Errors[0].Msg
}

// declaredFuncs is the number of functions and methods with a body in the files that were parsed.
func declaredFuncs(pkg *packages.Package) int {
	n := 0
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				n++
			}
		}
	}
	return n
}
