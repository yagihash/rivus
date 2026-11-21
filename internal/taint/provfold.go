package taint

import (
	"math/big"
	"strconv"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// The tracing of a guard's value tells a value only when it is a constant or
// a plain copy of one. When an operation (a negation, an arithmetic
// operation, a comparison, a conversion) is on the way, the operands are
// traced on their own and the operation is folded; when that cannot be done
// the operation is recorded as unresolved. The value of an operand is never
// told as the value of the result.

// provVal is a constant the tracing can compute with.
type provVal struct {
	kind byte // 'b' bool, 'i' integer, 's' string
	b    bool
	i    *big.Int
	s    string
}

func (c provVal) text() string {
	switch c.kind {
	case 'b':
		return strconv.FormatBool(c.b)
	case 'i':
		return c.i.String()
	}
	return strconv.Quote(c.s)
}

func (c provVal) equal(d provVal) bool {
	switch c.kind {
	case 'b':
		return c.b == d.b
	case 'i':
		return c.i.Cmp(d.i) == 0
	}
	return c.s == d.s
}

// provConst reads a constant (or the zero value) of a basic type.
func provConst(v *sg.Value) (provVal, bool) {
	if v.Const == nil {
		switch v.Class {
		case ir.ClassBool:
			return provVal{kind: 'b'}, true
		case ir.ClassString:
			return provVal{kind: 's'}, true
		case ir.ClassNumber:
			if _, ok := intWidth(v.Type); ok {
				return provVal{kind: 'i', i: new(big.Int)}, true
			}
		}
		return provVal{}, false
	}
	switch v.Const.Kind {
	case ir.ConstBool:
		return provVal{kind: 'b', b: v.Const.Value == "true"}, v.Const.Value == "true" || v.Const.Value == "false"
	case ir.ConstString:
		return provVal{kind: 's', s: v.Const.Value}, !v.Const.Truncated
	case ir.ConstInt:
		n, ok := new(big.Int).SetString(v.Const.Value, 0)
		if !ok {
			return provVal{}, false
		}
		return provVal{kind: 'i', i: n}, true
	}
	return provVal{}, false
}

// intWidth gives the size of an integer type by its name: Go, Rust and Dart
// spell them differently. An unknown (named) type has no known size.
func intWidth(t ir.TypeID) (w intInfo, ok bool) {
	switch string(t) {
	case "int", "int64", "i64", "isize":
		return intInfo{64, true}, true
	case "int32", "i32", "rune":
		return intInfo{32, true}, true
	case "int16", "i16":
		return intInfo{16, true}, true
	case "int8", "i8":
		return intInfo{8, true}, true
	case "uint", "uint64", "u64", "usize", "uintptr":
		return intInfo{64, false}, true
	case "uint32", "u32":
		return intInfo{32, false}, true
	case "uint16", "u16":
		return intInfo{16, false}, true
	case "uint8", "byte", "u8":
		return intInfo{8, false}, true
	}
	return intInfo{}, false
}

type intInfo struct {
	bits   uint
	signed bool
}

func (w intInfo) fits(n *big.Int) bool {
	var lo, hi *big.Int
	if w.signed {
		hi = new(big.Int).Lsh(big.NewInt(1), w.bits-1)
		lo = new(big.Int).Neg(hi)
		hi.Sub(hi, big.NewInt(1))
	} else {
		lo = new(big.Int)
		hi = new(big.Int).Lsh(big.NewInt(1), w.bits)
		hi.Sub(hi, big.NewInt(1))
	}
	return n.Cmp(lo) >= 0 && n.Cmp(hi) <= 0
}

// wrap truncates n to the integer type, as a conversion does.
func (w intInfo) wrap(n *big.Int) *big.Int {
	mod := new(big.Int).Lsh(big.NewInt(1), w.bits)
	r := new(big.Int).Mod(n, mod)
	if w.signed && r.Cmp(new(big.Int).Lsh(big.NewInt(1), w.bits-1)) >= 0 {
		r.Sub(r, mod)
	}
	return r
}

// foldUnop computes the operator on a constant. res is the result value: its
// type says the size of an integer.
func foldUnop(op string, a provVal, res *sg.Value) (provVal, bool) {
	switch {
	case op == "!" && a.kind == 'b':
		return provVal{kind: 'b', b: !a.b}, true
	case a.kind != 'i':
		return provVal{}, false
	}
	w, ok := intWidth(res.Type)
	if !ok {
		return provVal{}, false
	}
	switch op {
	case "-":
		r := new(big.Int).Neg(a.i)
		return provVal{kind: 'i', i: r}, w.fits(r)
	case "^", "!", "~":
		// Bitwise not: -x-1 on a signed integer, all bits flipped on an
		// unsigned one.
		r := new(big.Int).Not(a.i)
		if !w.signed {
			r = new(big.Int).Xor(a.i, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), w.bits), big.NewInt(1)))
		}
		return provVal{kind: 'i', i: r}, w.fits(r)
	case "+":
		return a, w.fits(a.i)
	}
	return provVal{}, false
}

// foldBinop computes the operator on two constants.
func foldBinop(op string, a, b provVal, res *sg.Value) (provVal, bool) {
	if a.kind != b.kind {
		return provVal{}, false
	}
	boolean := func(x bool) (provVal, bool) { return provVal{kind: 'b', b: x}, true }
	switch op {
	case "==":
		return boolean(a.equal(b))
	case "!=":
		return boolean(!a.equal(b))
	}
	switch a.kind {
	case 'b':
		switch op {
		case "&&", "&":
			return boolean(a.b && b.b)
		case "||", "|":
			return boolean(a.b || b.b)
		case "^":
			return boolean(a.b != b.b)
		}
	case 's':
		switch op {
		case "+":
			return provVal{kind: 's', s: a.s + b.s}, true
		case "<":
			return boolean(a.s < b.s)
		case "<=":
			return boolean(a.s <= b.s)
		case ">":
			return boolean(a.s > b.s)
		case ">=":
			return boolean(a.s >= b.s)
		}
	case 'i':
		c := a.i.Cmp(b.i)
		switch op {
		case "<":
			return boolean(c < 0)
		case "<=":
			return boolean(c <= 0)
		case ">":
			return boolean(c > 0)
		case ">=":
			return boolean(c >= 0)
		}
		w, ok := intWidth(res.Type)
		if !ok {
			return provVal{}, false
		}
		r := new(big.Int)
		switch op {
		case "+":
			r.Add(a.i, b.i)
		case "-":
			r.Sub(a.i, b.i)
		case "*":
			r.Mul(a.i, b.i)
		case "/", "%":
			if b.i.Sign() == 0 {
				return provVal{}, false
			}
			if op == "/" {
				r.Quo(a.i, b.i)
			} else {
				r.Rem(a.i, b.i)
			}
		case "&":
			r.And(a.i, b.i)
		case "|":
			r.Or(a.i, b.i)
		case "^":
			r.Xor(a.i, b.i)
		case "&^":
			r.AndNot(a.i, b.i)
		case "<<", ">>":
			if b.i.Sign() < 0 || b.i.Cmp(big.NewInt(int64(w.bits))) >= 0 {
				return provVal{}, false
			}
			if op == "<<" {
				r.Lsh(a.i, uint(b.i.Int64()))
			} else {
				r.Rsh(a.i, uint(b.i.Int64()))
			}
		default:
			return provVal{}, false
		}
		// An overflow is not folded: the languages differ on what it does.
		return provVal{kind: 'i', i: r}, w.fits(r)
	}
	return provVal{}, false
}

// foldConvert computes a conversion of a constant to the type of res.
func foldConvert(a provVal, res *sg.Value) (provVal, bool) {
	switch {
	case a.kind == 'b' && res.Class == ir.ClassBool:
		return a, true
	case a.kind == 's' && res.Class == ir.ClassString:
		return a, true
	case a.kind == 'i' && res.Class == ir.ClassNumber:
		w, ok := intWidth(res.Type)
		if !ok {
			return provVal{}, false
		}
		return provVal{kind: 'i', i: w.wrap(a.i)}, true
	}
	return provVal{}, false
}
