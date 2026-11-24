package valuecalls

import (
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// cellProgram: main holds a variable in a cell, stores the function `first` to it, makes the closure `reader` that gets the
// cell and loads it to call it, and stores `second` before or after it makes the closure (after: `late`).
func cellProgram(late bool) *ir.Program {
	pre := []ir.Instr{
		{Op: ir.OpAlloc, Dsts: []ir.LocalID{0}, Type: "cell"},
		{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{1}, Func: "first"},
		{Op: ir.OpStore, Args: []ir.LocalID{0, 1}},
		{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{2}, Func: "second"},
	}
	store2 := ir.Instr{Op: ir.OpStore, Args: []ir.LocalID{0, 2}}
	mk := ir.Instr{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{3}, Args: []ir.LocalID{0}, Func: "reader"}
	var instrs []ir.Instr
	if late {
		instrs = append(append(pre, mk), store2)
	} else {
		instrs = append(append(pre, store2), mk)
	}
	main := &ir.Func{
		ID:     "main",
		Locals: []ir.Local{{Class: ir.ClassPointer}, {Class: ir.ClassFunc}, {Class: ir.ClassFunc}, {Class: ir.ClassFunc}},
		Blocks: []ir.Block{{Instrs: instrs, Term: ir.Term{Kind: ir.TermReturn}}},
		Roots:  []ir.BlockID{0},
		Entry:  true,
	}
	reader := &ir.Func{
		ID:     "reader",
		Params: []ir.Param{{Class: ir.ClassPointer, Captured: true}},
		Locals: []ir.Local{{Class: ir.ClassPointer}, {Class: ir.ClassFunc}, {Class: ir.ClassOther}},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{
				{Op: ir.OpParam, Dsts: []ir.LocalID{0}, Index: 0},
				{Op: ir.OpLoad, Dsts: []ir.LocalID{1}, Args: []ir.LocalID{0}},
				{Op: ir.OpCall, Dsts: []ir.LocalID{2}, Args: []ir.LocalID{1}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
	}
	return &ir.Program{Funcs: []*ir.Func{main, reader, fnLeaf("first"), fnLeaf("second")}}
}

func cellCands(t *testing.T, late bool) (ids []string, partial bool) {
	t.Helper()
	prog := cellProgram(late)
	Resolve(ir.MemSource(prog))
	in := prog.Funcs[1].Blocks[0].Instrs[2]
	for _, c := range in.Callee.ValueCands {
		ids = append(ids, string(c.ID))
	}
	sort.Strings(ids)
	return ids, in.Callee.ValueLimit != ""
}

// A variable that was assigned `first` and then `second` before the closure was made holds `second` in the closure; the
// first value is not a candidate.
func TestCellValueIsTheOneThatReachesTheClosure(t *testing.T) {
	ids, partial := cellCands(t, false)
	if len(ids) != 1 || ids[0] != "second" || partial {
		t.Errorf("want [second], got %v (limit %v)", ids, partial)
	}
}

// A variable that is assigned after the closure was made (a `late` variable): the closure may be called after the
// assignment, so both are candidates.
func TestCellValueAssignedAfterTheClosure(t *testing.T) {
	ids, partial := cellCands(t, true)
	if len(ids) != 2 || ids[0] != "first" || ids[1] != "second" || partial {
		t.Errorf("want [first second], got %v (limit %v)", ids, partial)
	}
}

// A call in the function that holds the cell, between the two stores, calls what was stored before it (`var f = first;
// f(a); f = second;`), not the later one.
func TestCellLoadInTheHoldingFunction(t *testing.T) {
	prog := cellProgram(false)
	main := prog.Funcs[0]
	in := main.Blocks[0].Instrs
	// alloc, make first, store, [load, call], make second, store, make reader
	call := []ir.Instr{
		{Op: ir.OpLoad, Dsts: []ir.LocalID{4}, Args: []ir.LocalID{0}},
		{Op: ir.OpCall, Dsts: []ir.LocalID{5}, Args: []ir.LocalID{4}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
	}
	main.Locals = append(main.Locals, ir.Local{Class: ir.ClassFunc}, ir.Local{Class: ir.ClassOther})
	main.Blocks[0].Instrs = append(append(append([]ir.Instr{}, in[:3]...), call...), in[3:]...)
	Resolve(ir.MemSource(prog))
	got := main.Blocks[0].Instrs[4].Callee.ValueCands
	if len(got) != 1 || got[0].ID != "first" {
		t.Errorf("want [first], got %v", got)
	}
}
