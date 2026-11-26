package sg

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// A macro that writes a closure twice at one place makes two functions at the place: the type of the environment
// names the place, so it says no more than "one of the two". The alloc of the environment says the function of its
// closure (Instr.Func), and that is the one that is linked.
func TestLinkEnvClosuresByFuncID(t *testing.T) {
	env := ir.TypeID("{closure@src/c.rs:5:20: 5:40}")
	alloc := func(fn ir.FuncID) ir.Instr {
		return ir.Instr{Op: ir.OpAlloc, Dsts: []ir.LocalID{0}, Type: env, Class: ir.ClassFunc, Func: fn}
	}
	closure := func(id string) *ir.Func {
		return &ir.Func{ID: ir.FuncID(id), Symbol: id, File: "src/c.rs", Pos: ir.Pos{File: "src/c.rs", Line: 5, Col: 20}, Blocks: []ir.Block{{Term: ir.Term{Kind: ir.TermReturn}}}}
	}
	home := func(id string, in ir.Instr) *ir.Func {
		return &ir.Func{ID: ir.FuncID(id), Symbol: id, File: "src/c.rs", Entry: true, Locals: []ir.Local{{Class: ir.ClassFunc}},
			Blocks: []ir.Block{{Instrs: []ir.Instr{in}, Term: ir.Term{Kind: ir.TermReturn, Values: []ir.LocalID{0}}}}}
	}
	build := func(a, b ir.FuncID) *Program {
		p, err := Build(ir.MemSource(&ir.Program{Funcs: []*ir.Func{
			home("z::a", alloc(a)), home("z::b", alloc(b)),
			closure("z::v::{closure#0}"), closure("z::v::{closure#1}"),
		}}))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	linked := func(p *Program, fn string) string {
		for _, b := range p.ByID[fn].Blocks {
			for _, in := range b.Instrs {
				if c := p.EnvClosures[in]; c != nil {
					return c.ID
				}
			}
		}
		return ""
	}

	p := build("z::v::{closure#0}", "z::v::{closure#1}")
	if got := linked(p, "z::a"); got != "z::v::{closure#0}" {
		t.Errorf("a: linked to %q, want {closure#0}", got)
	}
	if got := linked(p, "z::b"); got != "z::v::{closure#1}" {
		t.Errorf("b: linked to %q, want {closure#1}", got)
	}
	// an old extractor says no function: two at the place are not told apart
	p = build("", "")
	if got := linked(p, "z::a") + linked(p, "z::b"); got != "" {
		t.Errorf("without a function ID, two at one place are linked: %q", got)
	}
	// a function ID that the program does not have: the type says it (here two at the place: none)
	p = build("z::gone::{closure#0}", "z::gone::{closure#1}")
	if got := linked(p, "z::a") + linked(p, "z::b"); got != "" {
		t.Errorf("a function that is not in the program is linked: %q", got)
	}
}
