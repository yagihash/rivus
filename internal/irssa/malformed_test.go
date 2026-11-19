package irssa

import "testing"

import "github.com/yagihash/rivus/internal/ir"

func handFunc(locals int, params []ir.Param, instrs []ir.Instr, ret ...ir.LocalID) *ir.Func {
	f := &ir.Func{ID: "f", Params: params}
	for i := 0; i < locals; i++ {
		f.Locals = append(f.Locals, ir.Local{Name: string(rune('a' + i))})
	}
	f.Blocks = []ir.Block{{ID: 0, Instrs: instrs, Term: ir.Term{Kind: ir.TermReturn, Values: ret}}}
	return f
}

// A store whose value is a load of the same variable, with the store first
// (a use before the definition) must not recurse without end (#23).
func TestStoreOfALaterLoadOfTheSameVariableTerminates(t *testing.T) {
	f := handFunc(2, nil, []ir.Instr{
		{Op: ir.OpAlloc, Dsts: []ir.LocalID{0}, Type: "T"},
		{Op: ir.OpStore, Args: []ir.LocalID{0, 1}},
		{Op: ir.OpLoad, Dsts: []ir.LocalID{1}, Args: []ir.LocalID{0}},
	}, 1)
	fm, err := Build(f)
	if err != nil {
		t.Fatal(err)
	}
	_ = fm.Operand(0, 1, 1)
	_ = fm.Operand(0, 3, 0)
}

// A load whose destination is written more than once must not hide the other
// writes: the variable stays in memory (#24).
func TestLoadIntoAMultiplyWrittenLocalKeepsItsWrites(t *testing.T) {
	f := handFunc(4, []ir.Param{{Name: "p"}}, []ir.Instr{
		{Op: ir.OpParam, Dsts: []ir.LocalID{3}, Index: 0},
		{Op: ir.OpAlloc, Dsts: []ir.LocalID{0}, Type: "T"},
		{Op: ir.OpStore, Args: []ir.LocalID{0, 3}},
		{Op: ir.OpConst, Dsts: []ir.LocalID{1}, Const: &ir.Const{Kind: ir.ConstNil}},
		{Op: ir.OpLoad, Dsts: []ir.LocalID{1}, Args: []ir.LocalID{0}},
	}, 1)
	fm, err := Build(f)
	if err != nil {
		t.Fatal(err)
	}
	got := fm.Operand(0, 5, 0)
	want := Ref{Kind: RefInstr, Block: 0, Index: 4}
	if fm.Promoted[0] {
		want = Ref{Kind: RefInstr, Block: 0, Index: 0}
	}
	if got != want {
		t.Errorf("the returned local resolves to %v, want %v (promoted=%v)", got, want, fm.Promoted)
	}
}
