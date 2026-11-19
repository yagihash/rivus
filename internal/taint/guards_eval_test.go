package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
	"go/constant"
)

func evalParam(typ ir.TypeID) (*sg.Func, *sg.Value) {
	fn := &sg.Func{}
	p := &sg.Value{Kind: sg.KParam, Param: 0, Type: typ, Func: fn}
	fn.Params = []*sg.Value{p}
	return fn, p
}

func evalConstInt(s string) *sg.Value {
	return &sg.Value{Kind: sg.KConst, Const: &ir.Const{Kind: ir.ConstInt, Value: s}}
}

func evalBinop(op string, a, b *sg.Value) *sg.Value {
	return &sg.Value{Kind: sg.KResult, Instr: &sg.Instr{Op: ir.OpBinop, IR: &sg.InstrInfo{Operator: op}, Args: []*sg.Value{a, b}}}
}

// Kinds that do not compare decide nothing, and never panic (#3).
func TestEvalCondMixedKinds(t *testing.T) {
	_, p := evalParam("dynamic")
	for name, tc := range map[string]struct {
		cond *sg.Value
		env  constant.Value
	}{
		"string against int": {evalBinop("==", p, evalConstInt("1")), constant.MakeString("a")},
		"bool order":         {evalBinop("<", p, evalConstInt("3")), constant.MakeBool(true)},
	} {
		if _, decided := evalCond(tc.cond, map[int]constant.Value{0: tc.env}); decided {
			t.Errorf("%s: must be undecided", name)
		}
	}
	// the same kinds still decide
	if v, decided := evalCond(evalBinop("==", p, evalConstInt("1")), map[int]constant.Value{0: constant.MakeInt64(1)}); !decided || !v {
		t.Error("1 == 1 must be decided true")
	}
}

// A conversion that changes the type may change the value: int8(n) == 0
// with n = 256 is true at run time (#3).
func TestEvalCondConversionIsNotTransparent(t *testing.T) {
	_, p := evalParam("int")
	conv := &sg.Value{Kind: sg.KResult, Type: "int8", Instr: &sg.Instr{Op: ir.OpConvert, IR: &sg.InstrInfo{Operator: "convert"}, Args: []*sg.Value{p}}}
	cond := evalBinop("==", conv, evalConstInt("0"))
	if _, decided := evalCond(cond, map[int]constant.Value{0: constant.MakeInt64(256)}); decided {
		t.Error("a narrowing conversion must leave the condition undecided")
	}
}
