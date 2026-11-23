package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// A call of a reserved symbol (chan.send, iter.next, ...) with no arguments or
// results is refused before the engine sees it, instead of crashing it (#17).
func TestReservedCallOfTheWrongShapeIsRefused(t *testing.T) {
	for _, sym := range []string{"chan.send", "chan.recv", "iter.start", "iter.next", "iter.next_string"} {
		f := &ir.Func{
			ID: "f", Symbol: "f",
			Blocks: []ir.Block{{ID: 0,
				Instrs: []ir.Instr{{Op: ir.OpCall, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: sym}}},
				Term:   ir.Term{Kind: ir.TermReturn}}},
			Roots: []ir.BlockID{0},
			Entry: true,
		}
		prog := &ir.Program{Funcs: []*ir.Func{f}}
		if err := prog.Validate(); err == nil {
			t.Errorf("%s: a call with no arguments and results passes the validation", sym)
		}
		if _, err := sg.Build(ir.MemSource(prog)); err == nil {
			t.Errorf("%s: the graph is built from it", sym)
		}
	}
}
