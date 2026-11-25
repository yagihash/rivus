// syntax_calls writes the calls that the Go source writes, one JSON object per line, for bench/go-callcheck/callcheck.py.
//
//	go run ./bench/go-callcheck/syntax DIR      (DIR: a Go module; the packages of ./..., without tests, as `rivus debug extract` reads them)
//
// A call is a *ast.CallExpr that is neither a conversion (types.Info.Types[fun].IsType()) nor a constant (len of an array,
// unsafe.Sizeof). Its position is the byte offset of the "(" that opens the argument list (CallExpr.Lparen), except the call
// that a `go` or `defer` statement makes: go/ssa gives it the position of the keyword, and so does the extractor, so that
// is the position here. A record has:
//
//	file, line       the file (relative to DIR, with "/") and the line of the position
//	offset           the position, 0-based, in bytes
//	start            the offset of the first token of the call (the callee part is start .. offset)
//	kind             "call", "go", "defer", or "builtin" (a call of a builtin function; "name" is the builtin)
//	name             the builtin ("append", "panic", "unsafe.String", ...), or ""
//	owner, owner_line  the outermost declaration that holds the call (a function, a method, a package variable); a closure is part of it
//	dead             "" or the reason that go/ssa does not keep the call: "unreachable" (go/cfg: the statement cannot be reached
//	                 from the entry of its function) or "discarded_closure" (the call is in a function literal whose value is
//	                 assigned to the blank identifier, so nothing can call it)
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/packages"
)

type rec struct {
	File      string `json:"file"`
	Offset    int    `json:"offset"`
	Start     int    `json:"start"`
	Line      int    `json:"line"`
	Kind      string `json:"kind"`
	Name      string `json:"name,omitempty"`
	Owner     string `json:"owner"`
	OwnerLine int    `json:"owner_line"`
	Dead      string `json:"dead,omitempty"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: syntax DIR")
		os.Exit(2)
	}
	dir, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal(err)
	}
	cfgp := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps, Dir: dir}
	pkgs, err := packages.Load(cfgp, "./...")
	if err != nil {
		fatal(err)
	}
	var out []rec
	failed := false
	for _, p := range pkgs {
		for _, e := range p.Errors {
			fmt.Fprintln(os.Stderr, "syntax_calls:", p.PkgPath+":", e)
			failed = true
		}
		for _, f := range p.Syntax {
			out = append(out, fileCalls(p, f, dir)...)
		}
	}
	if failed {
		os.Exit(1)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Offset < out[j].Offset
	})
	enc := json.NewEncoder(os.Stdout)
	for _, r := range out {
		enc.Encode(r)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "syntax_calls:", err)
	os.Exit(1)
}

// unreachable is the set of statements (and conditions) that go/cfg puts in a block that cannot be reached, for every
// function and function literal of the file. A call is not reachable when one of its ancestors is in it.
func unreachable(p *packages.Package, f *ast.File) map[ast.Node]bool {
	dead := map[ast.Node]bool{}
	mayReturn := func(c *ast.CallExpr) bool {
		if id, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
			if b, ok := p.TypesInfo.Uses[id].(*types.Builtin); ok && b.Name() == "panic" {
				return false
			}
		}
		return true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		if body != nil {
			for _, b := range cfg.New(body, mayReturn).Blocks {
				if !b.Live {
					for _, node := range b.Nodes {
						dead[node] = true
					}
				}
			}
		}
		return true
	})
	return dead
}

// discarded is the set of the function literals whose value is assigned to the blank identifier (`_ = func() { .. }`,
// `var _ = func() { .. }`): no one can call them, and go/ssa does not keep their bodies.
func discarded(f *ast.File) map[*ast.FuncLit]bool {
	set := map[*ast.FuncLit]bool{}
	blank := func(e ast.Expr) bool { id, ok := e.(*ast.Ident); return ok && id.Name == "_" }
	lit := func(e ast.Expr) {
		if l, ok := ast.Unparen(e).(*ast.FuncLit); ok {
			set[l] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) == len(n.Rhs) {
				for i := range n.Lhs {
					if blank(n.Lhs[i]) {
						lit(n.Rhs[i])
					}
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) == len(n.Values) {
				for i := range n.Names {
					if blank(n.Names[i]) {
						lit(n.Values[i])
					}
				}
			}
		}
		return true
	})
	return set
}

func fileCalls(p *packages.Package, f *ast.File, dir string) []rec {
	tf := p.Fset.File(f.Pos())
	rel, err := filepath.Rel(dir, tf.Name())
	if err != nil {
		rel = tf.Name()
	}
	rel = filepath.ToSlash(rel)
	dead := unreachable(p, f)
	disc := discarded(f)
	var out []rec
	var stack []ast.Node
	owner, ownerLine := "", 0
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if len(stack) == 2 { // a declaration of the file
			owner, ownerLine = describe(p.Fset, n)
		}
		if vs, ok := n.(*ast.ValueSpec); ok && len(stack) == 3 { // one declaration of a `var ( .. )` group
			owner, ownerLine = "var "+vs.Names[0].Name, p.Fset.Position(vs.Pos()).Line
		}
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := ast.Unparen(ce.Fun)
		tv := p.TypesInfo.Types[fun]
		if tv.IsType() {
			return true
		}
		if v := p.TypesInfo.Types[ce]; v.Value != nil {
			return true
		}
		r := rec{File: rel, Kind: "call", Owner: owner, OwnerLine: ownerLine}
		pos := ce.Lparen
		if tv.IsBuiltin() {
			r.Kind = "builtin"
			switch fn := fun.(type) {
			case *ast.Ident:
				r.Name = fn.Name
			case *ast.SelectorExpr:
				r.Name = "unsafe." + fn.Sel.Name
			}
		}
		if len(stack) >= 2 {
			switch parent := stack[len(stack)-2].(type) {
			case *ast.GoStmt:
				r.Kind, pos = "go", parent.Go
			case *ast.DeferStmt:
				r.Kind, pos = "defer", parent.Defer
			}
		}
		r.Offset = p.Fset.Position(pos).Offset
		r.Line = p.Fset.Position(pos).Line
		r.Start = p.Fset.Position(ce.Pos()).Offset
		if r.Kind == "go" || r.Kind == "defer" {
			r.Start = r.Offset
		}
		for _, a := range stack {
			if dead[a] && r.Dead == "" {
				r.Dead = "unreachable"
			}
			if l, ok := a.(*ast.FuncLit); ok && disc[l] {
				r.Dead = "discarded_closure"
			}
		}
		out = append(out, r)
		return true
	})
	return out
}

func describe(fset *token.FileSet, n ast.Node) (string, int) {
	line := fset.Position(n.Pos()).Line
	switch d := n.(type) {
	case *ast.FuncDecl:
		name := d.Name.Name
		if d.Recv != nil && len(d.Recv.List) > 0 {
			name = recvName(d.Recv.List[0].Type) + "." + name
		}
		return name, fset.Position(d.Pos()).Line
	case *ast.GenDecl:
		return "var", line
	}
	return "?", line
}

func recvName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return recvName(e.X)
	case *ast.IndexExpr:
		return recvName(e.X)
	case *ast.IndexListExpr:
		return recvName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return "?"
}
