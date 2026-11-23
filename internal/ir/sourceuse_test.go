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

// The function that Each, Walk, WalkProject, WalkProjectIf and WalkMarked give is not kept after the call it was given to.
// A Source that keeps its functions out of memory decodes a function for each of these reads and lets it go; a reader that
// kept the pointer would hold the function (the memory it was spared) and would, beside a reader that asks for the same
// function by Func, have two copies of one function (valuecalls compares the functions by pointer). What is kept is the
// Ref (the ID and the number), and the function is asked for again. The test looks for the plain ways to keep it: a
// variable of the code around the call that is given the function, and an append of it to such a variable. A
// function that is passed on to a call is not followed.
func TestEachDoesNotKeepTheFunction(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	walks := map[string]bool{"Each": true, "Walk": true, "WalkProject": true, "WalkProjectIf": true, "WalkMarked": true}
	skip := map[string]bool{"internal/ir": true, "internal/irspill": true} // they make the Sources
	var found []string
	for _, dir := range []string{"cmd", "internal"} {
		filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if skip[filepath.ToSlash(filepath.Dir(rel))] {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Error(err)
				return nil
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				case *ast.Ident:
					name = fn.Name
				}
				if !walks[name] {
					return true
				}
				for _, a := range call.Args {
					if lit, ok := a.(*ast.FuncLit); ok {
						for _, s := range keepsFunction(lit) {
							found = append(found, fset.Position(s.Pos()).String())
						}
					}
				}
				return true
			})
			return nil
		})
	}
	sort.Strings(found)
	for _, s := range found {
		t.Errorf("keeps the *ir.Func that a walk gave it, after the call: %s", s)
	}
}

// keepsFunction lists where the function literal gives a function that it was given (a parameter of the type *Func or
// *ir.Func) to a variable that it did not declare, or appends it to one.
func keepsFunction(lit *ast.FuncLit) []ast.Node {
	isFunc := func(e ast.Expr) bool {
		star, ok := e.(*ast.StarExpr)
		if !ok {
			return false
		}
		switch x := star.X.(type) {
		case *ast.SelectorExpr:
			return x.Sel.Name == "Func"
		case *ast.Ident:
			return x.Name == "Func"
		}
		return false
	}
	params := map[string]bool{}
	for _, p := range lit.Type.Params.List {
		if isFunc(p.Type) {
			for _, n := range p.Names {
				if n.Name != "_" {
					params[n.Name] = true
				}
			}
		}
	}
	if len(params) == 0 {
		return nil
	}
	local := map[string]bool{}
	for _, p := range lit.Type.Params.List {
		for _, n := range p.Names {
			local[n.Name] = true
		}
	}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						local[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				local[id.Name] = true
			}
		case *ast.RangeStmt:
			if x.Tok == token.DEFINE {
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if id, ok := e.(*ast.Ident); ok {
						local[id.Name] = true
					}
				}
			}
		case *ast.FuncLit:
			for _, p := range x.Type.Params.List {
				for _, id := range p.Names {
					local[id.Name] = true
				}
			}
		}
		return true
	})
	// holds says whether the value is the function, or a value that has it in it (a struct, a pointer to one, an append)
	var holds func(e ast.Expr) bool
	holds = func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.Ident:
			return params[x.Name]
		case *ast.ParenExpr:
			return holds(x.X)
		case *ast.UnaryExpr:
			return x.Op == token.AND && holds(x.X)
		case *ast.CompositeLit:
			for _, el := range x.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					el = kv.Value
				}
				if holds(el) {
					return true
				}
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "append" {
				for _, a := range x.Args {
					if holds(a) {
						return true
					}
				}
			}
		}
		return false
	}
	root := func(e ast.Expr) string {
		for {
			switch x := e.(type) {
			case *ast.Ident:
				return x.Name
			case *ast.SelectorExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.StarExpr:
				e = x.X
			case *ast.ParenExpr:
				e = x.X
			default:
				return ""
			}
		}
	}
	var out []ast.Node
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Tok == token.DEFINE {
			return true
		}
		for i, l := range as.Lhs {
			if r := root(l); r == "" || r == "_" || local[r] {
				continue
			}
			if i < len(as.Rhs) && holds(as.Rhs[i]) {
				out = append(out, as)
			} else if len(as.Rhs) == 1 && len(as.Lhs) > 1 && holds(as.Rhs[0]) {
				out = append(out, as)
			}
		}
		return true
	})
	return out
}
