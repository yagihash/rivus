package irssa

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/testutil"
)

func form(t *testing.T, name string) *Form {
	t.Helper()
	p, err := gossa.Load(testutil.Testdata(), []string{"./cases/irshape"}, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	prog, err := goir.Extract(p, goir.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range prog.Funcs {
		if string(f.ID) == "example.com/cases/cases/irshape."+name {
			fm, err := Build(f)
			if err != nil {
				t.Fatal(err)
			}
			return fm
		}
	}
	t.Fatalf("no function %s", name)
	return nil
}

// find returns the position of the first call whose symbol ends with suffix,
// from index `from`.
func find(t *testing.T, f *ir.Func, suffix string, nth int) (block, instr int) {
	t.Helper()
	for bi, b := range f.Blocks {
		for ii, in := range b.Instrs {
			if in.Op == ir.OpCall && strings.HasSuffix(in.Callee.Symbol, suffix) {
				if nth == 0 {
					return bi, ii
				}
				nth--
			}
		}
	}
	t.Fatalf("no call to %s", suffix)
	return
}

func TestPromotedVariableTakesTheValueStored(t *testing.T) {
	fm := form(t, "Straight")
	sb, si := find(t, fm.Func, "lib.Source", 0)
	kb, ki := find(t, fm.Func, "lib.Sink", 0)
	got := fm.Operand(kb, ki, 0)
	want := Ref{Kind: RefInstr, Block: sb, Index: si}
	if got != want {
		t.Errorf("the argument of Sink is %v, want the result of Source %v", got, want)
	}
}

func TestLoopNeedsAPhi(t *testing.T) {
	fm := form(t, "Loop")
	kb, ki := find(t, fm.Func, "lib.Sink", 0)
	got := fm.Operand(kb, ki, 0)
	if got.Kind != RefPhi {
		t.Fatalf("the argument of Sink is %v, want a phi", got)
	}
	var phi *Phi
	for _, p := range fm.Phis[got.Block] {
		if p.Var == got.Var {
			phi = &p
		}
	}
	if phi == nil || len(phi.Args) != 2 {
		t.Fatalf("phi: %+v", phi)
	}
	sb, si := find(t, fm.Func, "lib.Source", 0)
	var fromSource, fromBinop bool
	for _, a := range phi.Args {
		if a.Kind == RefInstr && a.Block == sb && a.Index == si {
			fromSource = true
		}
		if a.Kind == RefInstr && fm.Func.Blocks[a.Block].Instrs[a.Index].Op == ir.OpBinop {
			fromBinop = true
		}
	}
	if !fromSource || !fromBinop {
		t.Errorf("the phi takes %v: want the result of Source and of the concatenation", phi.Args)
	}
}

func TestUninitialisedVariableIsZero(t *testing.T) {
	fm := form(t, "Zero")
	kb, ki := find(t, fm.Func, "lib.Sink", 0)
	if got := fm.Operand(kb, ki, 0); got.Kind != RefZero {
		t.Errorf("the argument of Sink is %v, want the zero value", got)
	}
}

func TestEitherBranchJoins(t *testing.T) {
	fm := form(t, "Either")
	kb, ki := find(t, fm.Func, "lib.Sink", 0)
	got := fm.Operand(kb, ki, 0)
	if got.Kind != RefPhi {
		t.Fatalf("got %v, want a phi", got)
	}
	// One argument is a constant, the other the result of Source.
	kinds := map[ir.Op]bool{}
	for _, p := range fm.Phis[got.Block] {
		for _, a := range p.Args {
			if a.Kind == RefInstr {
				kinds[fm.Func.Blocks[a.Block].Instrs[a.Index].Op] = true
			}
		}
	}
	if !kinds[ir.OpConst] || !kinds[ir.OpCall] {
		t.Errorf("phi arguments come from %v: want a constant and a call", kinds)
	}
}

func TestAllocationWhoseAddressIsTakenStaysInMemory(t *testing.T) {
	fm := form(t, "AddrTaken")
	for l := range fm.Promoted {
		if fm.Func.Locals[l].Name == "s" {
			t.Errorf("s was promoted although its address is taken")
		}
	}
	promotedP := false
	for l := range fm.Promoted {
		if fm.Func.Locals[l].Name == "p" {
			promotedP = true
		}
	}
	if !promotedP {
		t.Errorf("p holds an address and is only loaded and stored: it should be promoted")
	}
}

func TestShortCircuitIsAVariable(t *testing.T) {
	fm := form(t, "ShortCircuit")
	if len(fm.Variables) != 1 {
		t.Errorf("variables: %v", fm.Variables)
	}
	phis := 0
	for _, ps := range fm.Phis {
		phis += len(ps)
	}
	if phis != 1 {
		t.Errorf("%d phis, want 1 (the join of a && b)", phis)
	}
}
