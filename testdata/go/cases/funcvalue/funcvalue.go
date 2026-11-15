// Package funcvalue holds calls through function values: the callee is found
// from the functions that reach the value.
package funcvalue

import "example.com/cases/lib"

func apply(f func(string) string, s string) string { return f(s) }

func id(s string) string { return s }

func constant(s string) string { return "constant" }

// (The call graph does not tell the calls of apply apart: what one caller hands
// over, the others may call too. That is the precision of the call graph.)

// A function handed to a parameter that is called there.
func ThroughParameter() {
	x := lib.Source()      // @source(param)
	lib.Sink(apply(id, x)) // @sink(param)
}

// Chosen at run time: either may be called.
func Chosen(c bool) {
	f := constant
	if c {
		f = id
	}
	x := lib.Source() // @source(chosen)
	lib.Sink(f(x))    // @sink(chosen)
}

// A closure that captures nothing.
func Literal() {
	x := lib.Source()                                      // @source(literal)
	lib.Sink(apply(func(s string) string { return s }, x)) // @sink(literal)
}
