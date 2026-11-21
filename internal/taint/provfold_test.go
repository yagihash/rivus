package taint

import (
	"math/big"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

func intVal(n int64) provVal { return provVal{kind: 'i', i: big.NewInt(n)} }

// The folding is the same for every language: the operators are spelled as
// Go, Rust and Dart write them, and the types by their names in each.
func TestProvenanceFoldsOperations(t *testing.T) {
	res := func(typ string, c ir.TypeClass) *sg.Value { return &sg.Value{Type: ir.TypeID(typ), Class: c} }
	for _, tc := range []struct {
		name string
		got  func() (provVal, bool)
		want string // "" when it cannot be folded
	}{
		{"not true", func() (provVal, bool) { return foldUnop("!", provVal{kind: 'b', b: true}, res("bool", ir.ClassBool)) }, "false"},
		{"negate", func() (provVal, bool) { return foldUnop("-", intVal(9), res("int", ir.ClassNumber)) }, "-9"},
		{"rust i32 sub", func() (provVal, bool) { return foldBinop("-", intVal(9), intVal(9), res("i32", ir.ClassNumber)) }, "0"},
		{"dart int sub", func() (provVal, bool) { return foldBinop("-", intVal(9), intVal(9), res("int", ir.ClassNumber)) }, "0"},
		{"compare", func() (provVal, bool) { return foldBinop(">", intVal(0), intVal(5), res("bool", ir.ClassBool)) }, "false"},
		{"unsigned underflow", func() (provVal, bool) { return foldBinop("-", intVal(0), intVal(1), res("u32", ir.ClassNumber)) }, ""},
		{"unknown int type", func() (provVal, bool) { return foldBinop("+", intVal(1), intVal(1), res("Foo", ir.ClassNumber)) }, ""},
		{"divide by zero", func() (provVal, bool) { return foldBinop("/", intVal(1), intVal(0), res("int", ir.ClassNumber)) }, ""},
		{"truncating conversion", func() (provVal, bool) { return foldConvert(intVal(256), res("int8", ir.ClassNumber)) }, "0"},
		{"int to string", func() (provVal, bool) { return foldConvert(intVal(98), res("string", ir.ClassString)) }, ""},
		{"string concat", func() (provVal, bool) {
			return foldBinop("+", provVal{kind: 's', s: "a"}, provVal{kind: 's', s: "b"}, res("string", ir.ClassString))
		}, `"ab"`},
		{"unknown operator", func() (provVal, bool) { return foldBinop("??", intVal(1), intVal(1), res("int", ir.ClassNumber)) }, ""},
	} {
		v, ok := tc.got()
		switch {
		case tc.want == "" && ok:
			t.Errorf("%s: folded to %s, want not folded", tc.name, v.text())
		case tc.want != "" && (!ok || v.text() != tc.want):
			t.Errorf("%s: got %v %q, want %q", tc.name, ok, v.text(), tc.want)
		}
	}
}
