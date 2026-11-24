package valuecalls

import (
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// fnLeaf is a function that does nothing (a closure body); fnIdentity returns its first parameter.
func fnLeaf(id ir.FuncID) *ir.Func {
	return &ir.Func{
		ID:     id,
		Blocks: []ir.Block{{Term: ir.Term{Kind: ir.TermReturn}}},
		Roots:  []ir.BlockID{0},
	}
}

func fnIdentity(id ir.FuncID) *ir.Func {
	return &ir.Func{
		ID:     id,
		Params: []ir.Param{{Name: "f", Class: ir.ClassFunc}},
		Locals: []ir.Local{{Class: ir.ClassFunc}},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{{Op: ir.OpParam, Dsts: []ir.LocalID{0}, Index: 0}},
			Term:   ir.Term{Kind: ir.TermReturn, Values: []ir.LocalID{0}},
		}},
		Roots: []ir.BlockID{0},
	}
}

// main makes the closures A and B, passes each to the same function (id), and calls what comes back. The call of what id
// returned for A is a call of A only: the walk follows the call that it came through, not every caller of id.
func returnedProgram() *ir.Program {
	main := &ir.Func{
		ID: "main",
		Locals: []ir.Local{
			{Class: ir.ClassFunc}, {Class: ir.ClassFunc}, {Class: ir.ClassFunc}, {Class: ir.ClassFunc}, {Class: ir.ClassStruct}, {Class: ir.ClassStruct},
		},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{
				{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{0}, Func: "A"},
				{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{1}, Func: "B"},
				{Op: ir.OpCall, Dsts: []ir.LocalID{2}, Args: []ir.LocalID{0}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Func: "id", Symbol: "id"}},
				{Op: ir.OpCall, Dsts: []ir.LocalID{3}, Args: []ir.LocalID{1}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Func: "id", Symbol: "id"}},
				{Op: ir.OpCall, Dsts: []ir.LocalID{4}, Args: []ir.LocalID{2}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
				{Op: ir.OpCall, Dsts: []ir.LocalID{5}, Args: []ir.LocalID{3}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
		Entry: true,
	}
	id, a, b := fnIdentity("id"), fnLeaf("A"), fnLeaf("B")
	return &ir.Program{Funcs: []*ir.Func{main, id, a, b}}
}

func TestReturnedValueIsFollowedPerCall(t *testing.T) {
	prog := returnedProgram()
	Resolve(ir.MemSource(prog))
	var got [][]string
	for _, in := range prog.Funcs[0].Blocks[0].Instrs {
		if in.Op == ir.OpCall && in.Callee.Kind == ir.CalleeValue {
			var ids []string
			for _, c := range in.Callee.ValueCands {
				ids = append(ids, string(c.ID))
			}
			sort.Strings(ids)
			got = append(got, ids)
		}
	}
	if len(got) != 2 || len(got[0]) != 1 || got[0][0] != "A" || len(got[1]) != 1 || got[1][0] != "B" {
		t.Errorf("want [A] and [B], got %v", got)
	}
}
