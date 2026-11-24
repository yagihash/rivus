package valuecalls

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// main passes the closure C to via; via calls the function it got (outer) with the closure X; C calls its own parameter g.
// The call of g reaches X: the callers of C include the call of outer, which has C as its candidate.
func TestCallbackOfCallbackIsFollowed(t *testing.T) {
	param := func(i int) ir.Instr { return ir.Instr{Op: ir.OpParam, Dsts: []ir.LocalID{ir.LocalID(i)}, Index: i} }
	c := &ir.Func{
		ID:     "C",
		Params: []ir.Param{{Name: "g", Class: ir.ClassFunc}, {Name: "a", Class: ir.ClassStruct}},
		Locals: []ir.Local{{Class: ir.ClassFunc}, {Class: ir.ClassStruct}, {Class: ir.ClassStruct}},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{
				param(0), param(1),
				{Op: ir.OpCall, Dsts: []ir.LocalID{2}, Args: []ir.LocalID{0, 1}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
	}
	via := &ir.Func{
		ID:     "via",
		Params: []ir.Param{{Name: "outer", Class: ir.ClassFunc}},
		Locals: []ir.Local{{Class: ir.ClassFunc}, {Class: ir.ClassFunc}, {Class: ir.ClassStruct}},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{
				param(0),
				{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{1}, Func: "X"},
				{Op: ir.OpCall, Dsts: []ir.LocalID{2}, Args: []ir.LocalID{0, 1}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
	}
	main := &ir.Func{
		ID:     "main",
		Locals: []ir.Local{{Class: ir.ClassFunc}, {Class: ir.ClassStruct}},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{
				{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{0}, Func: "C"},
				{Op: ir.OpCall, Dsts: []ir.LocalID{1}, Args: []ir.LocalID{0}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Func: "via", Symbol: "via"}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
		Entry: true,
	}
	prog := &ir.Program{Funcs: []*ir.Func{main, via, c, &ir.Func{ID: "X", Blocks: []ir.Block{{Term: ir.Term{Kind: ir.TermReturn}}}, Roots: []ir.BlockID{0}}}}
	Resolve(ir.MemSource(prog))
	got := c.Blocks[0].Instrs[2].Callee.ValueCands
	if len(got) != 1 || got[0].ID != "X" {
		t.Errorf("want the call of g to reach X, got %v", got)
	}
}
