package heap

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"example.com/cases/lib"
)

type box struct{ a, b string }

type nested struct {
	in box
	p  *box
}

func NestedField() {
	s := lib.Source() // @source(nested)
	var n nested
	n.in.a = s
	lib.Sink(n.in.a) // @sink(nested)
	lib.Sink(n.in.b) // @sink(none)
}

func PointerField() {
	s := lib.Source() // @source(pfield)
	n := &nested{p: &box{}}
	n.p.a = s
	lib.Sink(n.p.a) // @sink(pfield)
	lib.Sink(n.p.b) // @sink(none)
}

func StructLiteral() {
	s := lib.Source() // @source(lit)
	b := box{a: s}
	c := b
	lib.Sink(c.a) // @sink(lit)
	lib.Sink(c.b) // @sink(none)
}

func MapKeyAndValue() {
	s := lib.Source() // @source(mapkey)
	m := map[string]string{}
	m[s] = "const"
	lib.Sink(m["k"]) // @sink(none)
	for k := range m {
		lib.Sink(k) // @sink(mapkey)
	}
}

func MapValue() {
	s := lib.Source() // @source(mapval)
	m := map[string]string{"k": "const"}
	m["k"] = s
	for _, v := range m {
		lib.Sink(v) // @sink(mapval)
	}
}

func Channel() {
	s := lib.Source() // @source(chan)
	ch := make(chan string, 1)
	ch <- s
	lib.Sink(<-ch) // @sink(chan)
}

func SliceOfStructs() {
	s := lib.Source() // @source(slicestruct)
	items := []box{{a: s}}
	lib.Sink(items[0].a) // @sink(slicestruct)
	lib.Sink(items[0].b) // @sink(none)
}

func makeBox(s string) box { return box{a: s} }

func ReturnedStruct() {
	s := lib.Source()      // @source(retstruct)
	lib.Sink(makeBox(s).a) // @sink(retstruct)
	lib.Sink(makeBox(s).b) // @sink(none)
}

func newBox(s string) *box { return &box{a: s} }

func ReturnedPointer() {
	s := lib.Source()     // @source(retptr)
	lib.Sink(newBox(s).a) // @sink(retptr)
	lib.Sink(newBox(s).b) // @sink(none)
}

func ClosureMutatesCapturedVariable() {
	s := lib.Source() // @source(closmut)
	var out string
	f := func() { out = s }
	f()
	lib.Sink(out) // @sink(closmut)
}

func GoroutineClosure() {
	s := lib.Source() // @source(gogo)
	go func() {
		lib.Sink(s) // @sink(gogo)
	}()
}

type acc struct{ s string }

func (a *acc) Set(v string) { a.s = v }

func (a *acc) Get() string { return a.s }

func MethodMutatesReceiver() {
	s := lib.Source() // @source(recv)
	a := &acc{}
	a.Set(s)
	lib.Sink(a.Get()) // @sink(recv)
}

var cfg struct{ val, other string }

func InitConfig(s string) { cfg.val = s }

func UseConfig() {
	lib.Sink(cfg.val)   // @sink(globalfield)
	lib.Sink(cfg.other) // @sink(none)
}

func FillConfig() {
	s := lib.Source() // @source(globalfield)
	InitConfig(s)
}

func InterfaceHolds() {
	s := lib.Source() // @source(iface)
	var x any = s
	lib.Sink(x.(string)) // @sink(iface)
}

func ErrorText() {
	s := lib.Source() // @source(err)
	err := fmt.Errorf("failed: %s", s)
	lib.Sink(err.Error()) // @sink(err)
}

func RangeOverSlice() {
	s := lib.Source() // @source(rangeslice)
	for _, v := range []string{s} {
		lib.Sink(v) // @sink(rangeslice)
	}
}

func Builder() {
	s := lib.Source() // @source(builder)
	var sb strings.Builder
	sb.WriteString("x=")
	sb.WriteString(s)
	lib.Sink(sb.String()) // @sink(builder)
}

func UnmodeledStdlib() {
	s := lib.Source()                     // @source(unmodeled)
	lib.Sink(strings.ToValidUTF8(s, "?")) // @sink(unmodeled)
}

func Variadic(parts ...string) string { return strings.Join(parts, ",") }

func VariadicArguments() {
	s := lib.Source()          // @source(variadic)
	lib.Sink(Variadic("a", s)) // @sink(variadic)
}

func VariadicOnlyConstants() {
	_ = lib.Source()
	lib.Sink(Variadic("a", "b")) // @sink(none)
}

func PureFunctionsDoNotPropagate() {
	s := lib.Source()
	n, _ := strconv.Atoi(s) // an unmodeled function: the number is tainted
	deadline := time.Now().Add(time.Duration(n))
	lib.SinkAny(deadline) // @sink(none)
}

// A number made from a tainted string has no text to inject: taint stops at it
// (unless -numeric-taint).
func NumberStopsTaint() {
	s := lib.Source() // @source(numstop)
	n, _ := strconv.Atoi(s)
	lib.Sink(strconv.Itoa(n)) // @sink(none)
	lib.Sink(s)               // @sink(numstop)
}

// A model that writes to an argument writes to what the pointer refers to,
// also when the pointer went through an interface (any(&b)).
func UnmarshalIntoPointer() {
	s := lib.Source() // @source(unmarshal)
	var b box
	_ = json.Unmarshal([]byte(s), &b)
	lib.Sink(b.a) // @sink(unmarshal)
}

func UnmarshalIntoString() {
	s := lib.Source() // @source(unmarshalstr)
	var t string
	_ = json.Unmarshal([]byte(s), &t)
	lib.Sink(t) // @sink(unmarshalstr)
}
