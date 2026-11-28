package valuecalls

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// A function that has no SSA form (irssa.Build refuses it) holds a call of a function value: the call is not "a call of no
// function". It has the uncertainty "partial" with the cause no_ssa_form (funcsOf says it so, where the walk meets such a function).
func TestCallOfAValueInAFunctionWithNoFormIsUncertain(t *testing.T) {
	f := &ir.Func{
		ID:     "F",
		Params: []ir.Param{{Name: "g", Class: ir.ClassFunc}},
		// a local of an unknown type class: Validate (so Build) refuses the function
		Locals: []ir.Local{{Class: ir.ClassFunc}, {Class: "no-such-class"}},
		Blocks: []ir.Block{{
			Instrs: []ir.Instr{
				{Op: ir.OpParam, Dsts: []ir.LocalID{0}, Index: 0},
				{Op: ir.OpCall, Dsts: []ir.LocalID{1}, Args: []ir.LocalID{0}, Callee: &ir.Callee{Kind: ir.CalleeValue}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
	}
	Resolve(ir.MemSource(&ir.Program{Funcs: []*ir.Func{f}}))
	c := f.Blocks[0].Instrs[1].Callee
	if c.ValueLimit != ir.ValuePartial || len(c.ValueWhy) != 1 || c.ValueWhy[0] != ir.ValueWhyNoForm {
		t.Fatalf("limit = %q, why = %v: want partial / [no_ssa_form]", c.ValueLimit, c.ValueWhy)
	}
}
