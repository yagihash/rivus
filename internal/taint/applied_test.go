package taint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
)

// runGo analyses a module of one file. dep says which functions are taken as functions of a dependency.
func runGo(t *testing.T, src string, rules *model.Rules, dep func(id string) bool, workers int) *Result {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module zz\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zz.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	l := loadIR(t, dir, "./...")
	if dep != nil {
		for _, fn := range l.sg.Funcs {
			fn.Dep = dep(fn.ID)
		}
	}
	opts := DefaultOptions()
	opts.Workers = workers
	return l.engine(l.sites(rules), rules, opts).Run()
}

func emptyRules() *model.Rules {
	r := &model.Rules{}
	r.Sinks = map[string][]model.Sink{}
	return r
}

// valueCallModule is n functions that are each passed to apply, which calls its parameter.
func valueCallModule(n int) string {
	var b strings.Builder
	b.WriteString("package zz\n\nfunc apply(f func(string) string, s string) string { return f(s) }\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func t%d(s string) string { return s + \"%d\" }\nfunc c%d() string { return apply(t%d, \"x\") }\n", i, i, i, i)
	}
	return b.String()
}

// summary.value_candidates: the calls of function values by how many functions were found. The edge is at
// ir.ValueCandidateMost: 32 functions are followed, 33 are left out, and the count says which.
func TestValueCandidatesCountsTheCallsOverTheLimit(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{{ir.ValueCandidateMost, "21-32"}, {ir.ValueCandidateMost + 1, "33+"}, {5, "2-5"}, {1, "1"}} {
		res := runGo(t, valueCallModule(tc.n), emptyRules(), nil, 1)
		if got := res.ValueCandidates[tc.want]; got != 1 {
			t.Errorf("%d functions flow to the call: %v, want one call in %q", tc.n, res.ValueCandidates, tc.want)
		}
		for k, v := range res.ValueCandidates {
			if k != tc.want && v != 0 && k != "0" {
				t.Errorf("%d functions: the call is also counted in %q: %v", tc.n, k, res.ValueCandidates)
			}
		}
	}
	for _, c := range []struct {
		n    int
		want string
	}{{0, "0"}, {1, "1"}, {2, "2-5"}, {5, "2-5"}, {6, "6-20"}, {20, "6-20"}, {21, "21-32"}, {32, "21-32"}, {33, "33+"}, {300, "33+"}} {
		if got := ValueCandidateBucket(c.n); got != c.want {
			t.Errorf("ValueCandidateBucket(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

const dispatchModule = `package zz

type I interface{ M(s string) string }
type A struct{}
type B struct{}
type C struct{}

func (A) M(s string) string { return s }
func (B) M(s string) string { return s + "b" }
func (C) M(s string) string { return s + "c" }

func Call(i I, s string) string {
	r := ""
	for k := 0; k < len(s); k++ {
		r += i.M(r + s)
	}
	return r
}
func UseA() string { return Call(A{}, "x") }
func UseB() string { return Call(B{}, "x") }
func UseC() string { return Call(C{}, "x") }
`

func isImplOfBC(id string) bool {
	return strings.Contains(id, ".B).M") || strings.Contains(id, ".C).M")
}

// summary.dispatch_candidates_applied: each (call, candidate) the analysis applied, once. The call is in Call, with
// three implementations.
func TestDispatchCandidatesAppliedCountsWhatWasApplied(t *testing.T) {
	res := runGo(t, dispatchModule, emptyRules(), nil, 1)
	if res.DispatchApplied != 3 {
		t.Errorf("one call with three implementations (in a loop, so it is analysed more than once): applied %d, want 3", res.DispatchApplied)
	}
}

// A call that has a model does not apply the body of the functions of a dependency (the model says what they do): they
// are in the candidates of the IR, and are not counted.
func TestDispatchCandidatesAppliedLeavesOutWhatTheModelReplaces(t *testing.T) {
	rules := emptyRules()
	rules.Propagators = map[string][]model.Propagator{}
	for _, sym := range []string{"(zz.I).M"} {
		rules.Propagators[sym] = []model.Propagator{{Symbol: sym, Pure: true}}
	}
	res := runGo(t, dispatchModule, rules, isImplOfBC, 1)
	// the own implementation (A): 1. B and C are in a dependency, the model replaces them.
	if res.DispatchApplied != 1 {
		t.Errorf("with a model and B, C of a dependency: applied %d, want 1 (A)", res.DispatchApplied)
	}
	// without the model the same dependency functions are applied
	if without := runGo(t, dispatchModule, emptyRules(), isImplOfBC, 1); without.DispatchApplied != 3 {
		t.Errorf("without a model: applied %d, want 3", without.DispatchApplied)
	}
}

// The number does not depend on the number of workers, or on how many times a function was analysed again.
func TestDispatchCandidatesAppliedAndValueCandidatesAreTheSameEveryTime(t *testing.T) {
	src := dispatchModule + valueCallModule(40)[len("package zz\n"):]
	first := runGo(t, src, emptyRules(), nil, 1)
	for _, w := range []int{1, 2, 8} {
		for i := 0; i < 3; i++ {
			got := runGo(t, src, emptyRules(), nil, w)
			if got.DispatchApplied != first.DispatchApplied || fmt.Sprint(got.ValueCandidates) != fmt.Sprint(first.ValueCandidates) {
				t.Fatalf("workers %d, run %d: applied %d %v, first run %d %v", w, i, got.DispatchApplied, got.ValueCandidates, first.DispatchApplied, first.ValueCandidates)
			}
		}
	}
}

// A candidate whose place is past the size the set was made with is counted, not left out.
func TestNoteAppliedGrowsToThePlaceOfTheCandidate(t *testing.T) {
	e := &Engine{}
	ci := &sg.Instr{}
	e.noteApplied(ci, 0, 2)
	e.noteApplied(ci, 1, 2)
	e.noteApplied(ci, 1, 2) // again: once
	e.noteApplied(ci, 70, 100)
	e.noteApplied(ci, 200, 3) // a larger list than the one it was made with
	if got := e.dispatchApplied(); got != 4 {
		t.Errorf("applied %d, want 4", got)
	}
}
