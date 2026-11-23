package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// A row of calls of a package says `checked_by: cell` (docs/MODELING.md, section 3): no harness runs the package (it is the
// code of a third party), and a cell of the zoo holds the row instead. The cell is a file of a `cells` tree of the zoo that
// calls the function of the row, and that has a mark that says what the analysis finds there: @sink(<id>) (not `none`) or
// @miss(<kind>). The marks are checked by the zoo tests of the language. This test checks that the cell is there, so that
// a row that says it is held is held by something.

// isStdlibSymbol says whether the symbol is a function of the standard library of its language (the only rows that a
// harness can call).
func isStdlibSymbol(sym string) bool {
	switch {
	case strings.HasPrefix(sym, "dart:"), strings.HasPrefix(sym, "std::"), strings.HasPrefix(sym, "core::"), strings.HasPrefix(sym, "alloc::"):
		return true
	case strings.HasPrefix(sym, "package:"), strings.Contains(sym, "::") && !strings.Contains(sym, " "):
		return false
	}
	// Go: `strings.Map`, `(*net/http.Client).Do`; a package of a third party has a dot in the first element of its path
	s := strings.TrimLeft(sym, "(*")
	first, rest, hasSlash := strings.Cut(s, "/")
	_ = rest
	return !hasSlash || !strings.Contains(first, ".")
}

// hasCellMark says whether the text of a cell has a mark that holds a flow: @sink(<id>) with an id that is not none, or @miss(.
func hasCellMark(text string) bool {
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, "@miss(") {
			return true
		}
		rest := l
		for {
			i := strings.Index(rest, "@sink(")
			if i < 0 {
				break
			}
			rest = rest[i+len("@sink("):]
			id, _, _ := strings.Cut(rest, ")")
			if strings.TrimSpace(id) != "none" {
				return true
			}
		}
	}
	return false
}

// cellRowProblems is what is wrong with the declared rows (symbol -> the file of the model that has it): the rows that no
// cell with a mark calls. callers is symbol -> the files that call it; held says whether a file is a cell with a mark.
func cellRowProblems(declared map[string]string, callers map[string][]string, held func(file string) bool) []string {
	var bad []string
	for sym, file := range declared {
		ok := false
		for _, f := range callers[sym] {
			if held(f) {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, sym+" ("+file+"): says checked_by: cell, and no cell of the zoo that has a @sink or @miss mark calls it. Add the cell (bench/zoo-<language>/cells), or take checked_by out if a harness can call it")
		}
	}
	sort.Strings(bad)
	return bad
}

// irCallers adds, for each function that a call in a cell names, the files of the cells that call it.
func irCallers(callers map[string][]string, dir string, p *ir.Program) {
	for _, f := range p.Funcs {
		if f.Dep {
			continue
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall || in.Callee == nil || in.Callee.Symbol == "" {
					continue
				}
				file := in.Pos.File
				if file == "" {
					file = f.File
				}
				if file == "" {
					continue
				}
				if !filepath.IsAbs(file) {
					file = filepath.Join(dir, file)
				}
				if !strings.Contains(filepath.ToSlash(file), "/cells/") {
					continue
				}
				callers[in.Callee.Symbol] = append(callers[in.Callee.Symbol], file)
			}
		}
	}
}

func TestCallsRowsAreHeldByCells(t *testing.T) {
	t.Parallel()
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]map[string]string{}
	for _, rs := range sets {
		file := strings.TrimPrefix(rs.Origin, "builtin:")
		lang := strings.SplitN(file, "-", 2)[0]
		for _, p := range rs.Propagators {
			if !p.CallsCheckedByCell() {
				continue
			}
			if isStdlibSymbol(p.Symbol) {
				t.Errorf("%s (%s): checked_by: cell is for a package; a row of the standard library is held by a harness", p.Symbol, file)
				continue
			}
			if declared[lang] == nil {
				declared[lang] = map[string]string{}
			}
			declared[lang][p.Symbol] = file
		}
	}
	for _, lang := range []string{"go", "dart", "rust"} {
		rows := declared[lang]
		if len(rows) == 0 {
			continue
		}
		lang := lang
		t.Run(lang, func(t *testing.T) {
			callers := map[string][]string{}
			visit := func(dir string, p *ir.Program) { irCallers(callers, dir, p) }
			switch lang {
			case "go":
				goZooIR(t, visit)
			case "dart":
				dartZooIR(t, visit)
			case "rust":
				rustZooIR(t, visit)
			}
			held := func(file string) bool {
				b, err := os.ReadFile(file)
				return err == nil && hasCellMark(string(b))
			}
			for _, msg := range cellRowProblems(rows, callers, held) {
				t.Error(msg)
			}
		})
	}
}

func TestCellRowProblems(t *testing.T) {
	t.Parallel()
	declared := map[string]string{"example.com/lib.Subscribe": "go-lib.yaml"}
	held := func(f string) bool { return f == "/z/cells/a.go" }
	if got := cellRowProblems(declared, map[string][]string{"example.com/lib.Subscribe": {"/z/cells/a.go"}}, held); len(got) != 0 {
		t.Errorf("a cell with a mark calls it: %v", got)
	}
	if got := cellRowProblems(declared, map[string][]string{"example.com/lib.Subscribe": {"/z/cells/b.go"}}, held); len(got) != 1 {
		t.Errorf("a cell without a mark: %v", got)
	}
	if got := cellRowProblems(declared, nil, held); len(got) != 1 {
		t.Errorf("no cell: %v", got)
	}
}

func TestHasCellMarkAndStdlib(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		"x() // @sink(a-vuln)":                    true,
		"x() // @sink(none) @nosink(b)":           false,
		"x() // @source(a) @miss(unmodeled) @ref": true,
		"x()": false,
	} {
		if hasCellMark(text) != want {
			t.Errorf("hasCellMark(%q) = %v", text, !want)
		}
	}
	for sym, want := range map[string]bool{
		"strings.Map": true, "(*net/http.Client).Do": true, "sort.Slice": true, "example.com/lib.Subscribe": false,
		"(*github.com/a/b.T).M": false, "dart:core#Iterable.map": true, "package:shelf_router#Router.get": false,
		"std::option::Option::map": true, "tonic::Request": false,
	} {
		if isStdlibSymbol(sym) != want {
			t.Errorf("isStdlibSymbol(%q) = %v", sym, !want)
		}
	}
}
