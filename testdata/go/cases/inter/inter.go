package inter

import (
	"example.com/cases/cases/inter/util"
	"example.com/cases/lib"
)

type box struct{ a, b string }

func Pass(s string) string { return s }

func ThroughReturn() {
	s := lib.Source() // @source(pass)
	lib.Sink(Pass(s)) // @sink(pass)
}

func SinkInside(s string) {
	lib.Sink(s) // @sink(inner)
}

func IntoCallee() {
	s := lib.Source() // @source(inner)
	SinkInside(s)
}

func Produce() string {
	return lib.Source() // @source(ret)
}

func FromReturn() {
	lib.Sink(Produce()) // @sink(ret)
}

func Mutate(dst *box, v string) { dst.a = v }

func ThroughPointerArgument() {
	s := lib.Source() // @source(mut)
	var b box
	Mutate(&b, s)
	lib.Sink(b.a) // @sink(mut)
	lib.Sink(b.b) // @sink(none)
}

func Wrapper(s string) string { return lib.Sanitize(s) }

func SanitizerInsideWrapper() {
	s := lib.Source()
	lib.Sink(Wrapper(s)) // @sink(none)
}

func A(s string) string { return B(s) }
func B(s string) string { return C(s) }
func C(s string) string { return s }

func DeepChain() {
	s := lib.Source() // @source(chain)
	lib.Sink(A(s))    // @sink(chain)
}

func Recursive(s string, n int) string {
	if n == 0 {
		return s
	}
	return Recursive(s, n-1)
}

func ThroughRecursion() {
	s := lib.Source()         // @source(rec)
	lib.Sink(Recursive(s, 3)) // @sink(rec)
}

func Even(s string, n int) string {
	if n == 0 {
		return s
	}
	return Odd(s, n-1)
}

func Odd(s string, n int) string {
	if n == 0 {
		return "const"
	}
	return Even(s, n-1)
}

func ThroughMutualRecursion() {
	s := lib.Source()    // @source(mutual)
	lib.Sink(Even(s, 4)) // @sink(mutual)
}

func Two(s string) (string, string) { return s, "const" }

func PerResult() {
	s := lib.Source() // @source(multi)
	a, b := Two(s)
	lib.Sink(a) // @sink(multi)
	lib.Sink(b) // @sink(none)
}

var stored string

func Store(s string) { stored = s }

func Load() string { return stored }

func WriteGlobal() {
	s := lib.Source() // @source(glob)
	Store(s)
}

func ReadGlobal() {
	lib.Sink(Load()) // @sink(glob)
}

type Doer interface{ Do(s string) }

type impl struct{}

func (impl) Do(s string) {
	lib.Sink(s) // @sink(disp)
}

func Dispatch(d Doer) {
	s := lib.Source() // @source(disp)
	d.Do(s)
}

func UseImpl() { Dispatch(impl{}) }

func Closure() {
	s := lib.Source() // @source(clos)
	f := func() {
		lib.Sink(s) // @sink(clos)
	}
	f()
}

func CrossPackageThrough() {
	s := lib.Source()          // @source(xthrough)
	lib.Sink(util.Identity(s)) // @sink(xthrough)
}

func CrossPackageSanitized() {
	s := lib.Source()
	lib.Sink(util.Clean(s)) // @sink(none)
}

func CrossPackageSink() {
	s := lib.Source() // @source(crosspkg)
	util.Emit(s)
}

func CrossPackageSource() {
	lib.Sink(util.FetchTainted()) // @sink(crossret)
}
