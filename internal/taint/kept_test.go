package taint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"
)

// The result of an analysis holds kept(), not the whole state (the results of a large component
// are all held until the round is committed). So what commit and keepEarlier read of the state
// of the result must be among the fields kept() copies, and nothing else may read the state
// of a result (fnResult.s).
func TestResultKeepsWhatCommitReads(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "engine.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	copied := map[string]bool{}
	read := map[string]bool{}
	var sReaders []string // functions that read the .s of a result
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fd.Name.Name
		ast.Inspect(fd, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.KeyValueExpr:
				if name == "kept" {
					if id, ok := x.Key.(*ast.Ident); ok {
						copied[id.Name] = true
					}
				}
			case *ast.SelectorExpr:
				id, ok := x.X.(*ast.Ident)
				if !ok {
					return true
				}
				if id.Name == "r" && x.Sel.Name == "s" && name != "compute" {
					sReaders = append(sReaders, name)
				}
				if x.Sel.Name != "kept" && (name == "commit" && id.Name == "s") || (name == "keepEarlier" && id.Name == "s") {
					read[x.Sel.Name] = true
				}
			}
			return true
		})
	}
	if len(copied) == 0 || len(read) == 0 {
		t.Fatal("the functions were not found")
	}
	for _, g := range sReaders {
		if g != "commit" {
			t.Errorf("%s reads the state of a result: only commit may", g)
		}
	}
	var missing []string
	for field := range read {
		if !copied[field] {
			missing = append(missing, field)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("commit or keepEarlier reads fields of the state that kept() does not copy: %v", missing)
	}
}
