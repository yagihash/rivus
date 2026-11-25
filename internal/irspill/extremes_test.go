package irspill

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// How the decoder reads each number field of the IR. A new number field has to be put in one of these, or
// TestRecordNumberFieldsAreClassified fails: a field that holds a value (a position) must keep all 64 bits, and a
// field that is a place (a local, a block, a parameter) is checked against a range.
const (
	holdsValue = "value"
	holdsPlace = "place"
)

var numberFields = map[string]string{
	"ir.Pos.Offset":  holdsValue,
	"ir.Pos.Length":  holdsValue,
	"ir.Pos.Line":    holdsValue,
	"ir.Pos.Col":     holdsValue,
	"ir.Func.Roots":  holdsPlace, // []BlockID
	"ir.Block.ID":    holdsPlace,
	"ir.Edge.To":     holdsPlace,
	"ir.Term.Cond":   holdsPlace,
	"ir.Term.Values": holdsPlace, // []LocalID
	"ir.Instr.Dsts":  holdsPlace, // []LocalID
	"ir.Instr.Args":  holdsPlace, // []LocalID
	"ir.Instr.Index": holdsPlace,
}

// numberKind says whether a type is a number or a slice of numbers.
func numberKind(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
}

// walkTypes lists the number fields (and the fields that are slices of numbers) of the types a record keeps.
func walkTypes(t reflect.Type, seen map[reflect.Type]bool, out map[string]bool) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		if t.Kind() != reflect.Pointer && numberKind(t.Elem()) {
			return
		}
		walkTypes(t.Elem(), seen, out)
	case reflect.Struct:
		if seen[t] {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Tag.Get("json") == "-" {
				continue
			}
			name := t.String() + "." + f.Name
			ft := f.Type
			if numberKind(ft) || (ft.Kind() == reflect.Slice && numberKind(ft.Elem())) {
				out[name] = true
				continue
			}
			walkTypes(ft, seen, out)
		}
	}
}

func TestRecordNumberFieldsAreClassified(t *testing.T) {
	found := map[string]bool{}
	walkTypes(reflect.TypeOf(ir.Func{}), map[reflect.Type]bool{}, found)
	var names []string
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if _, ok := numberFields[n]; !ok {
			t.Errorf("%s is a number field of the IR that is not in numberFields: say how the decoder reads it, and put extreme values in it in extremeFunc", n)
		}
	}
	for n := range numberFields {
		if !found[n] {
			t.Errorf("%s is in numberFields and is not a number field of the IR", n)
		}
	}
}

func asInt(u uint64) int { return int(int64(u)) }

// The values of a field that holds a value.
var extremeValues = []int{
	0, 1, -1, math.MaxInt64, math.MinInt64, math.MaxInt64 - 1, math.MinInt64 + 1,
	math.MaxInt32, math.MinInt32, math.MaxInt32 * 4, math.MaxInt32*4 + 1, math.MaxInt32*4 - 1,
	-math.MaxInt32 * 4, -math.MaxInt32*4 - 1, 1 << 40, -(1 << 40), 1<<32 - 1, 1 << 32,
	asInt(math.MaxUint64),      // u64::MAX as an int64
	asInt(1 << 63),             // 1 << 63 as an int64
	asInt(1<<63 + 12),          // a big u64
	100_000 * 90_000,           // the offset of Rust: line 90000
	100_000*4_000_000 + 99_999, // and a file with 4 million lines
}

// The values of a field that is a place: the ones the decoder takes.
func placeValues() []int {
	var v []int
	for _, x := range extremeValues {
		if x <= math.MaxInt32*4 && x >= -math.MaxInt32*4 {
			v = append(v, x)
		}
	}
	return v
}

type cycle struct {
	vals []int
	n    int
}

func (c *cycle) next() int { v := c.vals[c.n%len(c.vals)]; c.n++; return v }

// extremeFunc is a function with every number field set to the extreme values, one after the other.
func extremeFunc() *ir.Func {
	val := &cycle{vals: extremeValues}
	pl := &cycle{vals: placeValues()}
	pos := func() ir.Pos {
		return ir.Pos{File: "a.rs", Offset: val.next(), Length: val.next(), Line: val.next(), Col: val.next()}
	}
	ids := func(n int) []ir.LocalID {
		v := make([]ir.LocalID, n)
		for i := range v {
			v[i] = ir.LocalID(pl.next())
		}
		return v
	}
	f := &ir.Func{ID: "f", Pos: pos()}
	for i := 0; i < len(extremeValues); i++ {
		f.Roots = append(f.Roots, ir.BlockID(pl.next()))
	}
	for bi := 0; bi < len(extremeValues); bi++ {
		b := ir.Block{ID: ir.BlockID(pl.next())}
		for k := 0; k < 3; k++ {
			b.Instrs = append(b.Instrs, ir.Instr{
				Op: ir.OpCall, Dsts: ids(2), Args: ids(3), Pos: pos(), AddrPos: pos(), Index: pl.next(),
				ArgPos: []ir.Pos{pos(), pos()}, Const: &ir.Const{Kind: ir.ConstInt, Value: "18446744073709551615"},
			})
		}
		b.Term = ir.Term{Kind: ir.TermBranch, Cond: ir.LocalID(pl.next()), CondPos: pos(), Values: ids(2), Pos: pos()}
		b.Succs = []ir.Edge{{To: ir.BlockID(pl.next()), Kind: ir.EdgeTrue}, {To: ir.BlockID(pl.next()), Kind: ir.EdgeFalse}}
		f.Blocks = append(f.Blocks, b)
	}
	return f
}

// TestRecordExtremeNumbers: every number field of the IR comes back as it went in, for the values at the edges of the
// integers (the smallest and the largest, the edges of the range of a place, a u64 as an int64).
func TestRecordExtremeNumbers(t *testing.T) {
	f := extremeFunc()
	rec := Encode(nil, f)
	g, n, err := Decode(rec)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(rec) {
		t.Fatalf("read %d of %d bytes", n, len(rec))
	}
	if d := Diff(f, g); d != "" {
		t.Fatalf("differs at %s", d)
	}
	// the same through every pair of values in a position: the difference from the one before may pass the integers
	for _, a := range extremeValues {
		for _, b := range extremeValues {
			h := &ir.Func{ID: "h", Pos: ir.Pos{File: "x", Offset: a, Line: a, Length: b, Col: b}}
			h.Blocks = []ir.Block{{Instrs: []ir.Instr{{Op: ir.OpCopy, Pos: ir.Pos{File: "x", Offset: b, Line: b, Length: a, Col: a}}}}}
			rg, _, err := Decode(Encode(nil, h))
			if err != nil {
				t.Fatalf("%d, %d: %v", a, b, err)
			}
			if d := Diff(h, rg); d != "" {
				t.Fatalf("%d, %d: differs at %s", a, b, d)
			}
		}
	}
}

// TestRecordPlaceOutOfRange: a place that is outside what the writer makes is a corrupt record (it is not a value).
func TestRecordPlaceOutOfRange(t *testing.T) {
	for _, v := range []int{math.MaxInt32*4 + 1, -math.MaxInt32*4 - 1, math.MaxInt64, math.MinInt64} {
		f := &ir.Func{ID: "f", Blocks: []ir.Block{{Instrs: []ir.Instr{{Op: ir.OpCopy, Dsts: []ir.LocalID{ir.LocalID(v)}}}}}}
		if _, _, err := Decode(Encode(nil, f)); err == nil {
			t.Errorf("a local %d is read", v)
		}
	}
}
