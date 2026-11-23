package taint

import (
	"fmt"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// fieldWalk is a function that reads field f_i of a pointer p in a loop and
// makes the result the new p, for each of s fields: the locations a value can
// refer to grow as s^K.
func fieldWalk(s int) *ir.Program {
	const p, c = 0, 1
	f := &ir.Func{ID: "m.walk", Symbol: "m.walk", Roots: []ir.BlockID{0}, Entry: true,
		Params: []ir.Param{{Name: "p", Class: ir.ClassPointer}, {Name: "c", Class: ir.ClassBool}},
		Locals: []ir.Local{{Name: "p", Class: ir.ClassPointer}, {Name: "c", Class: ir.ClassBool}},
	}
	for i := 0; i < s; i++ {
		f.Locals = append(f.Locals, ir.Local{Class: ir.ClassPointer})
	}
	// blocks: 0 entry, 1..s headers (one test each), s+1..2s the arms, 2s+1 exit
	exit := ir.BlockID(2*s + 1)
	entry := ir.Block{ID: 0, Instrs: []ir.Instr{
		{Op: ir.OpParam, Dsts: []ir.LocalID{p}, Index: 0},
		{Op: ir.OpParam, Dsts: []ir.LocalID{c}, Index: 1},
	}, Term: ir.Term{Kind: ir.TermJump}, Succs: []ir.Edge{{To: 1, Kind: ir.EdgeNormal}}}
	f.Blocks = append(f.Blocks, entry)
	for i := 0; i < s; i++ {
		next := ir.BlockID(i + 2)
		if i == s-1 {
			next = exit
		}
		f.Blocks = append(f.Blocks, ir.Block{ID: ir.BlockID(i + 1), Term: ir.Term{Kind: ir.TermBranch, Cond: c},
			Succs: []ir.Edge{{To: ir.BlockID(s + 1 + i), Kind: ir.EdgeTrue}, {To: next, Kind: ir.EdgeFalse}}})
	}
	for i := 0; i < s; i++ {
		q := ir.LocalID(2 + i)
		f.Blocks = append(f.Blocks, ir.Block{ID: ir.BlockID(s + 1 + i), Instrs: []ir.Instr{
			{Op: ir.OpFieldLoad, Dsts: []ir.LocalID{q}, Args: []ir.LocalID{p}, Field: fmt.Sprintf("f%d", i)},
			{Op: ir.OpCopy, Dsts: []ir.LocalID{p}, Args: []ir.LocalID{q}},
		}, Term: ir.Term{Kind: ir.TermJump}, Succs: []ir.Edge{{To: 1, Kind: ir.EdgeNormal}}})
	}
	f.Blocks = append(f.Blocks, ir.Block{ID: exit, Term: ir.Term{Kind: ir.TermReturn}})
	return &ir.Program{Funcs: []*ir.Func{f}}
}

func runWalk(t *testing.T, s int, opts Options) (*Result, time.Duration) {
	t.Helper()
	prog := fieldWalk(s)
	if err := prog.Validate(); err != nil {
		t.Fatal(err)
	}
	g, err := sg.Build(ir.MemSource(prog))
	if err != nil {
		t.Fatal(err)
	}
	rules := &model.Rules{}
	rules.Sinks = map[string][]model.Sink{}
	st := sites.Enumerate(ir.MemSource(prog), ir.NewStreamTexts(prog), rules)
	start := time.Now()
	res := New(g, ir.NewStreamTexts(prog), st, rules, opts).Run()
	return res, time.Since(start)
}

// The fixed point of the locations of the values answers to the time limits
// like the rest of the analysis of a function (#18): a function that makes it
// grow as s^3 does not run past them, and the result says that it was cut.
func TestLocationsFixedPointStopsAtTheLimit(t *testing.T) {
	opts := DefaultOptions()
	opts.Workers = 1
	opts.FuncTimeout = 300 * time.Millisecond
	opts.Timeout = 2 * time.Second
	res, took := runWalk(t, 200, opts)
	if took > 4*time.Second {
		t.Errorf("the run took %v with a limit of %v", took, opts.Timeout)
	}
	cut := false
	for _, u := range res.Uncertainties {
		if u.Kind == KindDepthLimit {
			cut = true
		}
	}
	if !cut {
		t.Errorf("the cut is not in the result: %+v", res.Uncertainties)
	}
}

// Within the limits the same function is analysed in full, and not cut.
func TestLocationsFixedPointWithinTheLimitsIsNotCut(t *testing.T) {
	res, _ := runWalk(t, 20, DefaultOptions())
	for _, u := range res.Uncertainties {
		if u.Kind == KindDepthLimit {
			t.Errorf("cut: %+v", u)
		}
	}
}
