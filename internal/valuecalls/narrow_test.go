package valuecalls

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The narrowing of the calls through Fn::call (docs/DESIGN-capture.md, ADR-079) on hand-written IR: it may
// drop a candidate only when the walk followed everything. A test names the call by the function that holds
// it and says which closures must stay.

func resolved(t *testing.T, file string) *ir.Program {
	t.Helper()
	fh, err := os.Open(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	prog, err := ir.Read(fh)
	if err != nil {
		t.Fatal(err)
	}
	Resolve(prog)
	return prog
}

// fnCall is the call through Fn::call that the function holds.
func fnCall(t *testing.T, prog *ir.Program, holder string) *ir.Callee {
	t.Helper()
	for _, f := range prog.Funcs {
		if string(f.ID) != holder {
			continue
		}
		for _, b := range f.Blocks {
			for i := range b.Instrs {
				if in := &b.Instrs[i]; in.Op == ir.OpCall && isFnCall(in.Callee) {
					return in.Callee
				}
			}
		}
	}
	t.Fatalf("no call through Fn::call in %s", holder)
	return nil
}

func ids(c *ir.Callee) []string {
	var out []string
	for _, id := range c.Candidates {
		out = append(out, string(id))
	}
	sort.Strings(out)
	return out
}

func mustKeep(t *testing.T, c *ir.Callee, want ...string) {
	t.Helper()
	have := map[string]bool{}
	for _, id := range ids(c) {
		have[id] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("the candidate %s was narrowed away (candidates now %v)", w, ids(c))
		}
	}
}

// home(a, b, ..) and home(b, a, ..): the closure b reaches the call as the parameter 0 and as the parameter 1. The
// sink of b is a real flow.
func TestNarrowKeepsAClosureThatReachesThroughAnotherParameter(t *testing.T) {
	prog := resolved(t, "narrow_param_order.jsonl")
	c := fnCall(t, prog, "curated::zz::home")
	const a, b = "curated::zz::main::{closure#0}", "curated::zz::main::{closure#1}"
	mustKeep(t, c, a, b)
	for _, id := range []string{a, b} {
		if p, ok := c.CandParam[ir.FuncID(id)]; ok && p >= 0 {
			t.Errorf("%s is told to come through the parameter %d only: it comes through both", id, p)
		}
	}
}
