package nomodelcb

import (
	"strconv"

	"example.com/cases/lib"
)

// A function that has no body in the program and no model (lib.Thunk: the rules do not name it) that is given a
// closure is taken to call it, and what the closure returns is taken to reach the result (an assumed_call). A
// value that the closure captured is one of them.

func Captured() {
	src := lib.Source() // @source(nm_captured)
	out := lib.Thunk(func() string { return src })
	lib.Sink(out) // @sink(nm_captured)
}

// What the closure returns is what reaches the result: a closure that does not return the value does not carry it.
func NotReturned() {
	src := lib.Source() // @source(nm_decoy)
	out := lib.Thunk(func() string { _ = src; return "constant" })
	lib.Sink(out) // @sink(none)
}

// A closure that returns a clean value (the value was made clean before) does not carry it.
func Cleaned() {
	src := lib.Sanitize(lib.Source())
	out := lib.Thunk(func() string { return src })
	lib.Sink(out) // @sink(none)
}

// A call without a result: what the other operands hold may reach the parameters of the function that is given, in
// any position (the first operand is not the one that the first parameter is).
func NoResult() {
	src := lib.Source() // @source(nm_noresult)
	lib.Visit("prefix", []string{src}, func(x string) {
		lib.Sink(x) // @sink(nm_noresult)
	})
}

func NoResultFixed() {
	src := lib.Sanitize(lib.Source())
	lib.Visit("prefix", []string{src}, func(x string) {
		lib.Sink(x) // @sink(none)
	})
}

// A number carries nothing into the body.
func Number() {
	src := lib.Source()
	lib.VisitN([]string{src}, func(i int) {
		lib.Sink(strconv.Itoa(i)) // @sink(none)
	})
}
