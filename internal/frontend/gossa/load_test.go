package gossa

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const testdata = "../../../testdata/go"

func TestLoad(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/hello"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", p.LoadErrors)
	}
	var direct bool
	for _, fn := range p.Funcs {
		if FuncID(fn) == "example.com/cases/cases/hello.Direct" {
			direct = true
			if !p.Own[fn] {
				t.Errorf("%s should be own", FuncID(fn))
			}
		}
		// Dependencies have no bodies.
		if strings.HasPrefix(FuncID(fn), "example.com/cases/lib.") {
			t.Errorf("dependency function %s has a body", FuncID(fn))
		}
	}
	if !direct {
		t.Fatalf("Direct not found; funcs=%d", len(p.Funcs))
	}
}

func TestSnippetAndText(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/hello"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range p.Funcs {
		if FuncID(fn) != "example.com/cases/cases/hello.Direct" {
			continue
		}
		var found bool
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Pos().IsValid() && strings.Contains(p.Snippet(in.Pos()), "lib.Source()") {
					found = true
					if got := p.Position(in.Pos()); got.Line != 6 {
						t.Errorf("line = %d, want 6", got.Line)
					}
					if got := p.ExprText(in.Pos()); !strings.Contains(got, "lib . Source ( )") {
						t.Errorf("ExprText = %q", got)
					}
				}
			}
		}
		if !found {
			t.Error("snippet for lib.Source() not found")
		}
		if bt := p.BodyText(fn); !strings.HasPrefix(bt, "func Direct ( )") {
			t.Errorf("BodyText = %q", bt)
		}
	}
}

func TestLoadErrorsAreRecorded(t *testing.T) {
	// A package that does not type-check must not fail the whole load.
	p, err := Load(testdata, []string{"./cases/hello", "./cases/doesnotexist"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) == 0 {
		t.Fatal("expected a recorded load error")
	}
}

func TestImplementations(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/impl"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Implementations("(example.com/cases/cases/impl.Store).Save")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"(*example.com/cases/cases/impl.memStore).Save",
		"(example.com/cases/cases/impl.fileStore).Save",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}

	// A type of the universe has no package.
	if _, err := p.Implementations("(error).Error"); err != nil {
		t.Errorf("(error).Error: %v", err)
	}
	for _, bad := range []string{"os.Open", "(*impl.Store).Save", "(example.com/cases/cases/impl.Store).Nope", "(example.com/cases/cases/impl.memStore).Save", "(no/such.Pkg).M"} {
		if _, err := p.Implementations(bad); err == nil {
			t.Errorf("%q should not be accepted", bad)
		}
	}
}

// A package outside a module (a file named on the command line) has no Module, as the standard library
// has none: it is the code to analyse, so its bodies are built (yagihash/rivus #51).
func TestLoadOutsideModuleWithDeps(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc Source() string { return \"x\" }\nfunc Sink(s string) {}\n\nfunc main() { Sink(Source()) }\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, deps := range []bool{false, true} {
		p, err := Load(dir, []string{"main.go"}, Options{Deps: deps})
		if err != nil {
			t.Fatal(err)
		}
		own := 0
		for fn := range p.Own {
			if len(fn.Blocks) > 0 {
				own++
			}
		}
		if own < 3 {
			t.Errorf("deps=%v: %d own functions with a body, want at least 3 (load errors: %v)", deps, own, p.LoadErrors)
		}
	}
}

// closedFuncSrc uses generic functions of the standard library that have
// anonymous functions inside (their instances make the body of the generic
// function only when Origin is asked), and types whose methods those bodies use.
const closedFuncSrc = `package main

import (
	"sync"
	"sync/atomic"
)

type T struct{ n int }

var p atomic.Pointer[T]
var pool sync.Pool

func main() {
	f := sync.OnceValues(func() (string, error) { return "x", nil })
	g := sync.OnceValue(func() int { return 1 })
	a, _ := f()
	_ = a
	_ = g()
	pool.Put(&T{})
	p.Store(&T{})
	var mu sync.RWMutex
	mu.RLock()
	mu.RUnlock()
}
`

func loadClosedFuncSrc(t *testing.T) *Program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(closedFuncSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir, []string{"main.go"}, Options{Deps: true})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func bodiedFuncs(prog *ssa.Program) map[*ssa.Function]bool {
	m := map[*ssa.Function]bool{}
	for fn := range ssautil.AllFunctions(prog) {
		if len(fn.Blocks) > 0 {
			m[fn] = true
		}
	}
	return m
}

// The set of functions that have a body is closed: asking Origin of each
// function (which makes the body of a generic function when it is asked)
// does not add one. If it did, the set that Load gives would depend on the
// order the loop sees the functions in (ADR-118, (ii)).
func TestLoadFunctionSetIsClosed(t *testing.T) {
	p := loadClosedFuncSrc(t)
	before := bodiedFuncs(p.SSA)
	for fn := range ssautil.AllFunctions(p.SSA) {
		fn.Origin()
	}
	after := bodiedFuncs(p.SSA)
	if len(after) != len(before) {
		var added []string
		for fn := range after {
			if !before[fn] {
				added = append(added, FuncID(fn))
			}
		}
		sort.Strings(added)
		if len(added) > 5 {
			added = added[:5]
		}
		t.Errorf("functions with a body: %d before Origin, %d after (for example %v)", len(before), len(after), added)
	}
}

// The same input loaded three times in one process gives the same functions.
func TestLoadFunctionSetIsStable(t *testing.T) {
	var first []string
	for i := 0; i < 3; i++ {
		p := loadClosedFuncSrc(t)
		ids := make([]string, 0, len(p.Funcs))
		for _, fn := range p.Funcs {
			ids = append(ids, string(FuncID(fn)))
		}
		sort.Strings(ids)
		if i == 0 {
			first = ids
			continue
		}
		if len(ids) != len(first) {
			t.Fatalf("load %d: %d functions, load 0: %d", i, len(ids), len(first))
		}
		for j := range ids {
			if ids[j] != first[j] {
				t.Fatalf("load %d: function %d is %s, in load 0 %s", i, j, ids[j], first[j])
			}
		}
	}
}
