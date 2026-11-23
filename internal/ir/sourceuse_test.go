package ir_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
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
	"internal/frontend/accept/compare.go":   true, // compares the programs that two extractors made
	"internal/irspill/source.go":            true, // takes the functions of a program into the file
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

// The workers of the analysis (internal/taint) never ask a Source for a function. A function that cannot be read is a
// panic with a *ReadError (ir.Must, ir.Walk), which the command catches (runCommand) when it comes from the goroutine
// of the command; a panic in another goroutine is not caught and ends the process. The IR is read before the analysis
// starts (the sites, the calls of function values, the build of the graph), and the graph holds what the analysis needs.
// A reader in the packages that run in the workers would break that: it has to catch the panic itself and return it.
func TestWorkersDoNotReadTheSource(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]bool{"Must": true, "Get": true, "Walk": true, "WalkProject": true, "Source": true, "MemSource": true, "ConcatSource": true, "HeadOf": true}
	for _, dir := range []string{"internal/taint", "internal/sg"} {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, filepath.Join(root, dir), func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pkgs {
			for name, f := range p.Files {
				ast.Inspect(f, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok || !reads[sel.Sel.Name] {
						return true
					}
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == "ir" {
						// sg builds the graph from the Source, on the goroutine of the command (build.go): the one place
						if dir == "internal/sg" && filepath.Base(name) == "build.go" {
							return true
						}
						t.Errorf("%s reads the Source in a package that runs in the workers: ir.%s", fset.Position(sel.Pos()), sel.Sel.Name)
					}
					return true
				})
			}
		}
	}
}
