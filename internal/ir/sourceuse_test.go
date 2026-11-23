package ir_test

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The code that reads the functions of a program asks an ir.Source for them. Program.Funcs is touched only by what
// makes the program or writes it out (the files listed here), and by the package ir itself (the Source of a program
// in memory is made of it). A file that is not here and reads Program.Funcs is a reader that goes around the Source: it
// would keep working with the functions in memory, and break when they are not.
var makesProgram = map[string]bool{
	"cmd/rivus/lang.go":                     true, // joins the programs of the languages
	"cmd/rivus/bodyreplaced.go":             true, // drops the functions that the rules stand for
	"cmd/rivus/extract.go":                  true, // rivus debug extract: writes what the extractor made
	"internal/frontend/goir/goir.go":        true, // makes the program from the Go packages
	"internal/frontend/goir/streamtexts.go": true, // writes the texts of the functions into the program
}

func TestProgramFuncsIsReadThroughSource(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:  root,
	}
	pkgs, err := packages.Load(cfg, "github.com/yagihash/rivus/cmd/...", "github.com/yagihash/rivus/internal/...")
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, p := range pkgs {
		if strings.HasSuffix(p.PkgPath, "/internal/ir") {
			continue
		}
		for i, f := range p.Syntax {
			rel, err := filepath.Rel(cfg.Dir, p.CompiledGoFiles[i])
			if err != nil || strings.HasSuffix(rel, "_test.go") {
				continue
			}
			rel = filepath.ToSlash(rel)
			if makesProgram[rel] {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Funcs" {
					return true
				}
				tv, ok := p.TypesInfo.Types[sel.X]
				if !ok {
					return true
				}
				typ := tv.Type
				if ptr, ok := typ.(*types.Pointer); ok {
					typ = ptr.Elem()
				}
				if named, ok := typ.(*types.Named); ok && named.Obj().Name() == "Program" && named.Obj().Pkg() != nil &&
					strings.HasSuffix(named.Obj().Pkg().Path(), "/internal/ir") {
					found = append(found, rel+": "+p.Fset.Position(sel.Pos()).String())
				}
				return true
			})
		}
	}
	sort.Strings(found)
	for _, s := range found {
		t.Errorf("reads ir.Program.Funcs, not an ir.Source: %s", s)
	}
}
