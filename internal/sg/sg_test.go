package sg

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// A library whose only entries are the initializers of its variables is still read from its exported
// functions; a program that has a main is read from its entries alone (yagihash/rivus #60).
func TestReachableOfALibraryWithInitializers(t *testing.T) {
	fn := func(id string, exported, entry, init bool) *ir.Func {
		return &ir.Func{ID: ir.FuncID(id), Exported: exported, Entry: entry, Init: init, Blocks: []ir.Block{{Term: ir.Term{Kind: ir.TermReturn}}}}
	}
	reached := func(fs ...*ir.Func) map[string]bool {
		p, err := Build(ir.MemSource(&ir.Program{Funcs: fs}))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for f := range p.Reachable() {
			out[f.ID] = true
		}
		return out
	}
	lib := reached(fn("a.k$init", false, true, true), fn("a.handle", true, false, false))
	if !lib["a.handle"] || !lib["a.k$init"] {
		t.Errorf("library: %v", lib)
	}
	prog := reached(fn("a.k$init", false, true, true), fn("a.main", true, true, false), fn("a.handle", true, false, false))
	if prog["a.handle"] || !prog["a.main"] {
		t.Errorf("program: %v", prog)
	}
}
