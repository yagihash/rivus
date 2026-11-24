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

// A slice that grows in a loop keeps what was appended in every round.
func AppendInLoop() {
	s := lib.Source() // @source(apploop)
	var out []string
	for i := 0; i < 3; i++ {
		out = append(out, s)
	}
	lib.Sink(out[0]) // @sink(apploop)
}

func AppendInRangeLoop() {
	s := lib.Source() // @source(apprange)
	var out []string
	for _, w := range strings.Split(s, ",") {
		out = append(out, w)
	}
	lib.Sink(out[0]) // @sink(apprange)
}

func ExpIndexStoreInLoop() {
	s := lib.Source() // @source(exp1)
	out := make([]string, 3)
	for i := 0; i < 3; i++ {
		out[i] = s
	}
	lib.Sink(out[0]) // @sink(exp1)
}

func ExpAppendTwiceNoLoop() {
	s := lib.Source() // @source(exp2)
	var out []string
	out = append(out, "a")
	out = append(out, s)
	lib.Sink(out[0]) // @sink(exp2)
}

func ExpAppendIfBranch(c bool) {
	s := lib.Source() // @source(exp3)
	var out []string
	if c {
		out = append(out, s)
	}
	lib.Sink(out[0]) // @sink(exp3)
}

func ExpAppendLoopSinkInside() {
	s := lib.Source() // @source(exp4)
	var out []string
	for i := 0; i < 3; i++ {
		out = append(out, s)
		lib.Sink(out[0]) // @sink(exp4)
	}
}

type held struct{ out, other string }

// set writes through its receiver, and a closure captures the receiver: the variable is then kept in a cell, and the write
// must still be a write to the receiver of the caller.
func (h *held) set(s string) {
	h.out = s
	f := func() { _ = h.other }
	f()
}

func WriteThroughCapturedReceiver() {
	s := lib.Source() // @source(capturedrecv)
	h := &held{}
	h.set(s)
	lib.Sink(h.out) // @sink(capturedrecv)
}

// The forms near WriteThroughCapturedReceiver: a closure captures the receiver (or a parameter), so the variable is kept
// in a cell, and a write through it must still be a write to what the caller passed.

type inner struct{ b string }
type outer struct {
	a     inner
	other string
}

// value receiver: a copy of the struct is written, the caller does not see it; the pointer receiver is the case above
type vheld struct{ out string }

func (h vheld) setCopy(s string) {
	h.out = s
	f := func() { _ = h.out }
	f()
}

func ValueReceiverIsACopy() {
	s := lib.Source() // @source(capvalue)
	h := vheld{}
	h.setCopy(s)
	lib.Sink(h.out) // @sink(none)
}

// the closure reads only
func (h *held) setRead(s string) {
	h.out = s
	f := func() string { return h.other }
	_ = f()
}

func CapturedReadOnly() {
	s := lib.Source() // @source(capread)
	h := &held{}
	h.setRead(s)
	lib.Sink(h.out) // @sink(capread)
}

// the closure writes
func (h *held) setInClosure(s string) {
	f := func() { h.out = s }
	f()
}

func CapturedWrittenInClosure() {
	s := lib.Source() // @source(capwrite)
	h := &held{}
	h.setInClosure(s)
	lib.Sink(h.out) // @sink(capwrite)
}

// two levels of closures
func (h *held) setNested(s string) {
	h.out = s
	f := func() {
		g := func() { _ = h.other }
		g()
	}
	f()
}

func CapturedByNestedClosure() {
	s := lib.Source() // @source(capnested)
	h := &held{}
	h.setNested(s)
	lib.Sink(h.out) // @sink(capnested)
}

// defer and go
func (h *held) setDefer(s string) {
	defer func() { _ = h.other }()
	h.out = s
}

func CapturedByDefer() {
	s := lib.Source() // @source(capdefer)
	h := &held{}
	h.setDefer(s)
	lib.Sink(h.out) // @sink(capdefer)
}

func (h *held) setGo(s string) {
	done := make(chan struct{})
	go func() { _ = h.other; close(done) }()
	h.out = s
	<-done
}

func CapturedByGo() {
	s := lib.Source() // @source(capgo)
	h := &held{}
	h.setGo(s)
	lib.Sink(h.out) // @sink(capgo)
}

// a method value of the receiver passed as a function value
func (h *held) touch() { _ = h.other }

func (h *held) setMethodValue(s string) {
	f := h.touch
	f()
	h.out = s
}

func CapturedByMethodValue() {
	s := lib.Source() // @source(capmethval)
	h := &held{}
	h.setMethodValue(s)
	lib.Sink(h.out) // @sink(capmethval)
}

// two parameters, one is captured and the other written
func setTwo(a, b *held, s string) {
	f := func() { _ = a.other }
	f()
	b.out = s
}

func TwoParametersOneWritten() {
	s := lib.Source() // @source(captwo)
	a, b := &held{}, &held{}
	setTwo(a, b, s)
	lib.Sink(b.out) // @sink(captwo)
	lib.Sink(a.out) // @sink(none)
}

// a field of a field
func (o *outer) setDeep(s string) {
	o.a.b = s
	f := func() { _ = o.other }
	f()
}

func CapturedFieldOfField() {
	s := lib.Source() // @source(capdeep)
	o := &outer{}
	o.setDeep(s)
	lib.Sink(o.a.b)   // @sink(capdeep)
	lib.Sink(o.other) // @sink(none)
}

// the captured parameter is given to another function that writes it
func fill(h *held, s string) { h.out = s }

func (h *held) setViaHelper(s string) {
	f := func() { fill(h, s) }
	f()
}

func CapturedPassedToHelper() {
	s := lib.Source() // @source(capcall)
	h := &held{}
	h.setViaHelper(s)
	lib.Sink(h.out) // @sink(capcall)
}

// not written: a closure that only reads does not move the value to a field, or to another receiver
func (h *held) setOther(s string) {
	f := func() { _ = h.other }
	f()
	h.out = "c"
	_ = s
}

func CapturedNoFlowToField() {
	s := lib.Source() // @source(capnone)
	h := &held{}
	h.setOther(s)
	lib.Sink(h.out)   // @sink(none)
	lib.Sink(h.other) // @sink(none)
}

func CapturedOtherReceiver() {
	s := lib.Source() // @source(capother)
	h, k := &held{}, &held{}
	h.set(s)
	lib.Sink(k.out) // @sink(none)
}

// the variable that holds the receiver is assigned again. The cell holds both pointers and the analysis does not follow the order
// (as for every variable that is kept in a cell): the write goes to both, so h.out of the first one is a flow that the code does not
// have (a known false positive of this form: docs/issues). It is not a cell of this test; r.out is the flow that must be found.
func (h *held) setThenReplace(s string, r *held) {
	f := func() { _ = h.other }
	f()
	h = r
	h.out = s
}

func CapturedReassigned() {
	s := lib.Source() // @source(capreassign)
	h, r := &held{}, &held{}
	h.setThenReplace(s, r)
	lib.Sink(r.out) // @sink(capreassign)
}
