package taint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// parseNonTest parses the non-test Go files of this package.
func parseNonTest(t *testing.T) map[string]*ast.File {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*ast.File{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = f
	}
	return out
}

// kindConstants are the Kind constants (string literals) of the package: name to value.
func kindConstants(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range parseNonTest(t) {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if !strings.HasPrefix(n.Name, "Kind") || i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						v, _ := strconv.Unquote(lit.Value)
						out[n.Name] = v
					}
				}
			}
		}
	}
	return out
}

// Every Kind constant has a row in Kinds, and every row is a constant: the table is the one list.
func TestKindsTableCoversConstants(t *testing.T) {
	consts := kindConstants(t)
	if len(consts) < 20 {
		t.Fatalf("the Kind constants were not read: %v", consts)
	}
	inTable := map[string]bool{}
	for _, k := range Kinds {
		if inTable[k.Kind] {
			t.Errorf("kind %q is twice in Kinds", k.Kind)
		}
		inTable[k.Kind] = true
		switch k.Attributable {
		case AttributableTarget, AttributableRivus, AttributableMixed:
		default:
			t.Errorf("kind %q: attributable %q is not one of the three", k.Kind, k.Attributable)
		}
		if strings.TrimSpace(k.Doc) == "" {
			t.Errorf("kind %q has no description", k.Kind)
		}
	}
	var missing []string
	for name, v := range consts {
		if !inTable[v] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("Kind constants that are not in Kinds (kinds.go): %v", missing)
	}
	vals := map[string]bool{}
	for _, v := range consts {
		vals[v] = true
	}
	for k := range inTable {
		if !vals[k] {
			t.Errorf("Kinds has %q, which no Kind constant has", k)
		}
	}
}

// The attributable_to that an uncertainty gets is the one of the table.
func TestUncertaintyAttributableIsFromTable(t *testing.T) {
	for _, k := range Kinds {
		if got := newUncertainty(k.Kind, "", "").AttributableTo; got != k.Attributable {
			t.Errorf("kind %q: newUncertainty gives %q, the table says %q", k.Kind, got, k.Attributable)
		}
	}
}

// attributable_to is not written by hand where an uncertainty is made: a kind is given by its constant, and the value comes
// from the table (attributable(kind)). The only exception is the one that the table says in its row: a dynamic_dispatch
// that has no candidate is the rivus' (calls.go).
func TestAttributableNotAssignedByHand(t *testing.T) {
	isTableCall := func(e ast.Expr) bool {
		c, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := c.Fun.(*ast.Ident)
		return ok && id.Name == "attributable"
	}
	for name, f := range parseNonTest(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.KeyValueExpr:
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "AttributableTo" && !isTableCall(x.Value) {
					t.Errorf("%s: AttributableTo is written by hand; use attributable(kind) (the table, kinds.go)", name)
				}
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					sel, ok := l.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "AttributableTo" {
						continue
					}
					if name == "calls.go" && i < len(x.Rhs) {
						if id, ok := x.Rhs[i].(*ast.Ident); ok && id.Name == "AttributableRivus" {
							continue
						}
					}
					t.Errorf("%s: AttributableTo is assigned by hand (only the dynamic_dispatch without a candidate in calls.go may)", name)
				}
			case *ast.CallExpr:
				// a kind that is given as a string literal is not in the table by name
				var fn string
				switch f := x.Fun.(type) {
				case *ast.Ident:
					fn = f.Name
				case *ast.SelectorExpr:
					fn = f.Sel.Name
				}
				if fn == "newUncertainty" || fn == "NewUncertainty" {
					if len(x.Args) > 0 {
						if lit, ok := x.Args[0].(*ast.BasicLit); ok {
							t.Errorf("%s: %s is given the literal kind %s; use a Kind constant", name, fn, lit.Value)
						}
					}
				}
			}
			return true
		})
	}
}

// KindsTable has the three columns and a row for each kind of Kinds.
func TestKindsTableRows(t *testing.T) {
	tab := KindsTable()
	for _, k := range Kinds {
		if !strings.Contains(tab, "| `"+k.Kind+"` | "+k.Attributable+" | "+k.Doc+" |") {
			t.Errorf("KindsTable has no row for %s", k.Kind)
		}
	}
	if len(Kinds) == 0 {
		t.Error("Kinds is empty")
	}
}
