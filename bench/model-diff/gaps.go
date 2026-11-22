package main

import (
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
)

// coverageGaps lists the exported functions and methods of a package of the standard library that take a string, bytes or an `any`
// (what a query, a command, a path or a value can be given in) and that no built-in model file has a row for. The probes of model-diff
// start from the rows, so a function that has no row is never found by them (a missing sink or flow is a silent miss): this is the
// other direction, the list of what has no row. It says nothing about the flows of a function that has a row.
func coverageGaps(pkgPath string, modelFiles []string) ([]string, error) {
	pkg, err := importer.ForCompiler(token.NewFileSet(), "source", nil).Import(pkgPath)
	if err != nil {
		return nil, err
	}
	var models strings.Builder
	for _, f := range modelFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		models.Write(b)
	}
	text := models.String()
	takesData := func(sig *types.Signature) (string, bool) {
		var ps []string
		ok := false
		for i := 0; i < sig.Params().Len(); i++ {
			t := sig.Params().At(i).Type().String()
			ps = append(ps, t)
			switch t {
			case "string", "[]string", "[]byte", "any", "[]any", "io.Reader", "io.Writer", "[]interface{}":
				ok = true
			}
		}
		return strings.Join(ps, ", "), ok
	}
	var out []string
	check := func(sym string, sig *types.Signature) {
		if ps, ok := takesData(sig); ok && !strings.Contains(text, "symbol: "+sym+"\n") {
			out = append(out, fmt.Sprintf("%s(%s)", sym, ps))
		}
	}
	for _, n := range pkg.Scope().Names() {
		o := pkg.Scope().Lookup(n)
		if !o.Exported() {
			continue
		}
		switch o := o.(type) {
		case *types.Func:
			check(pkgPath+"."+n, o.Type().(*types.Signature))
		case *types.TypeName:
			ms := types.NewMethodSet(types.NewPointer(o.Type()))
			for i := 0; i < ms.Len(); i++ {
				if f, ok := ms.At(i).Obj().(*types.Func); ok && f.Exported() {
					check(fmt.Sprintf("(*%s.%s).%s", pkgPath, n, f.Name()), f.Type().(*types.Signature))
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}
