package sg

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// A method that overrides what a dependency declares is a method that a framework calls: an entry point
// of its own (ReachableWithCallbacks), and what it reaches. One that overrides a method of the code that
// was read is not: nothing says that a framework calls it (ADR-073).
func TestFrameworkCallbacks(t *testing.T) {
	fn := func(id string, entry, dep bool, overrides ...string) *ir.Func {
		return &ir.Func{ID: ir.FuncID(id), Symbol: id, Entry: entry, Dep: dep, Overrides: overrides, Exported: true, Blocks: []ir.Block{{Term: ir.Term{Kind: ir.TermReturn}}}}
	}
	p, err := Build(&ir.Program{Funcs: []*ir.Func{
		fn("a.main", true, false),
		fn("a.Page.build", false, false, "flutter.State.build"),
		fn("a.Child.run", false, false, "a.Base.run"),
		fn("a.Base.run", false, false),
		fn("flutter.State.build", false, true), // the dependency's own method, read
		fn("flutter.Other.x", false, true, "flutter.Super.x"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range p.FrameworkCallbacks() {
		names = append(names, f.ID)
	}
	if len(names) != 1 || names[0] != "a.Page.build" {
		t.Errorf("framework callbacks: %v, want only a.Page.build (a dependency's own method, and an override of the code's own, are not)", names)
	}
	r, rc := p.Reachable(), p.ReachableWithCallbacks()
	if r[p.ByID["a.Page.build"]] || !rc[p.ByID["a.Page.build"]] || !r[p.ByID["a.main"]] || !rc[p.ByID["a.main"]] {
		t.Errorf("reach: %v, with callbacks: %v", r, rc)
	}
	if rc[p.ByID["a.Child.run"]] {
		t.Error("an override of a method of the code that was read is not an entry")
	}
}
