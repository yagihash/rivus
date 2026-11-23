package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// rvobsSource is the package that the instrumented copy gets (<module>/rvobs). Enter is called first in every
// function; it reads the two innermost frames of the stack: the function that was entered and its caller.
const rvobsSource = `// Package rvobs records the calls that happen (written by bench/observed-edges; not part of the code that is analysed).
package rvobs

import (
	"encoding/json"
	"os"
	"runtime"
	"sync"
)

type key struct {
	From, To, File string
	Line         int
}

var (
	mu    sync.Mutex
	edges = map[key]int{}
)

// Enter records the edge (the caller, the line it called from) -> (the function that calls Enter).
func Enter() {
	var pc [3]uintptr
	n := runtime.Callers(2, pc[:])
	fr := runtime.CallersFrames(pc[:n])
	callee, ok := fr.Next()
	if !ok {
		return
	}
	caller, ok := fr.Next()
	if !ok {
		return
	}
	mu.Lock()
	edges[key{caller.Function, callee.Function, caller.File, caller.Line}]++
	mu.Unlock()
}

// Dump writes the edges to the file named by RVOBS_OUT (nothing when it is not set).
func Dump() {
	out := os.Getenv("RVOBS_OUT")
	if out == "" {
		return
	}
	type row struct {
		From  string ` + "`json:\"from\"`" + `
		To    string ` + "`json:\"to\"`" + `
		File  string ` + "`json:\"file\"`" + `
		Line  int    ` + "`json:\"line\"`" + `
		Count int    ` + "`json:\"count\"`" + `
	}
	mu.Lock()
	defer mu.Unlock()
	var rows []row
	for k, c := range edges {
		rows = append(rows, row{k.From, k.To, k.File, k.Line, c})
	}
	b, err := json.Marshal(map[string]any{"edges": rows})
	if err == nil {
		err = os.WriteFile(out, b, 0o644)
	}
	if err != nil {
		os.Stderr.WriteString("rvobs: " + err.Error() + "\n")
		os.Exit(1)
	}
}
`

const enterText = " rvobs.Enter(); "

// InstrumentSource returns the source with a call of rvobs.Enter at the start of every function and function literal,
// and the import of the package. The text is inserted on the lines that are there (after each "{" of a body, and after
// "package x"), so that no line number changes: the positions of the graph that was made from the original are the same.
func InstrumentSource(name string, src []byte, rvobsPath string) ([]byte, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, 0, err
	}
	var offs []int
	ast.Inspect(f, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		if body != nil {
			offs = append(offs, fset.Position(body.Lbrace).Offset+1)
		}
		return true
	})
	if len(offs) == 0 {
		return src, 0, nil
	}
	n := len(offs)
	pkgEnd := fset.Position(f.Name.End()).Offset
	offs = append(offs, pkgEnd)
	sort.Sort(sort.Reverse(sort.IntSlice(offs)))
	out := append([]byte(nil), src...)
	for _, o := range offs {
		ins := enterText
		if o == pkgEnd {
			ins = fmt.Sprintf("; import rvobs %q; ", rvobsPath)
		}
		out = append(out[:o], append([]byte(ins), out[o:]...)...)
	}
	return out, n, nil
}

// InstrumentTree instruments every .go file under dir (not the tests) and returns the number of functions.
func InstrumentTree(dir, rvobsPath string) (int, error) {
	total := 0
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out, n, err := InstrumentSource(p, src, rvobsPath)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if n == 0 {
			return nil
		}
		total += n
		return os.WriteFile(p, out, 0o644)
	})
	return total, err
}

// InsertDump puts "rvobs.Dump()" on the line of the first call of marker in the file (the harness of the cells ends its
// run there), and the import. The harness file is in the copy only.
func InsertDump(file, marker, rvobsPath string) error {
	src, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	i := strings.Index(string(src), marker)
	if i < 0 {
		return fmt.Errorf("%s: %q not found (the place where rvobs.Dump goes)", file, marker)
	}
	end := i + strings.IndexByte(string(src[i:]), '\n')
	if end < i {
		return fmt.Errorf("%s: no line end after %q", file, marker)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.PackageClauseOnly)
	if err != nil {
		return err
	}
	pkgEnd := fset.Position(f.Name.End()).Offset
	out := append([]byte(nil), src[:pkgEnd]...)
	out = append(out, []byte(fmt.Sprintf("; import rvobs %q", rvobsPath))...)
	out = append(out, src[pkgEnd:end]...)
	out = append(out, []byte("; rvobs.Dump()")...)
	out = append(out, src[end:]...)
	return os.WriteFile(file, out, 0o644)
}
