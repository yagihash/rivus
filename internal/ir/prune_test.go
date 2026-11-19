package ir

import "testing"

func TestPruneDepsKeepsWhatTheProjectReaches(t *testing.T) {
	call := func(id FuncID) Instr {
		return Instr{Op: OpCall, Callee: &Callee{Kind: CalleeStatic, Func: id}}
	}
	fn := func(id FuncID, dep bool, in ...Instr) *Func {
		return &Func{ID: id, Dep: dep, Blocks: []Block{{Instrs: in}}}
	}
	p := &Program{Funcs: []*Func{
		fn("own", false, call("dep.A"), Instr{Op: OpMakeClosure, Func: "dep.C"}),
		fn("dep.A", true, call("dep.B")),
		fn("dep.B", true),
		fn("dep.C", true),
		fn("dep.Unused", true, call("dep.B")),
		fn("dep.ViaImpl", true),
	}, Impls: []Implementations{{Method: "m", Impls: []FuncID{"dep.ViaImpl"}}}}
	p.PruneDeps()
	var got []string
	for _, f := range p.Funcs {
		got = append(got, string(f.ID))
	}
	want := []string{"own", "dep.A", "dep.B", "dep.C", "dep.ViaImpl"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// The initializer of a global that a kept function reads is kept (yagihash/rivus #60).
func TestPruneDepsKeepsTheInitializerOfAGlobalThatIsRead(t *testing.T) {
	p := &Program{Funcs: []*Func{
		{ID: "own", Blocks: []Block{{Instrs: []Instr{{Op: OpGlobalLoad, Global: "dep.table"}}}}},
		{ID: "dep.table$init", Dep: true, Entry: true, Init: true},
		{ID: "dep.other$init", Dep: true, Entry: true, Init: true},
	}}
	p.PruneDeps()
	if len(p.Funcs) != 2 || p.Funcs[1].ID != "dep.table$init" {
		t.Fatalf("got %d functions: %v", len(p.Funcs), p.Funcs)
	}
}
