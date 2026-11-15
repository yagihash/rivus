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
