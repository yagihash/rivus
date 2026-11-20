// Package funcsink holds calls of function values whose callee a rule names:
// the sink is the function the value holds (yagihash/rivus #54).
package funcsink

import "example.com/cases/lib"

func noop(s string) {}

// --- the function was copied into a variable: it is the one called ---

func Alias() {
	x := lib.Source() // @source(alias)
	snk := lib.Sink
	snk(x) // @sink(alias)
}

var runSink = lib.Sink

func Global() {
	x := lib.Source() // @source(global)
	runSink(x)        // @sink(global)
}

// --- chosen at run time: every function that may be called is ---

func Phi(c bool) {
	x := lib.Source() // @source(phi)
	f := noop
	if c {
		f = lib.Sink
	}
	f(x) // @sink(phi)
}

// --- a function that was not chosen is not ---

func PhiNone(c bool) {
	x := lib.Source()
	f := noop
	if c {
		f = noop
	}
	f(x)
}

// --- the callers choose: each flow goes through the function its caller passed ---

func a(s string) {
	lib.Sink(s) // @sink(param_a)
}

func b(s string) {}

func run(f func(string), x string) { f(x) }

func CallA() {
	x := lib.Source() // @source(param_a)
	run(a, x)
}

// no finding: this path calls b, and the sink is in a
func CallB() {
	y := lib.Source()
	run(b, y)
}

// the function passed is a sink that has no body here

func runRule(f func(string), x string) {
	f(x) // @sink(param_rule)
}

func CallRule() {
	x := lib.Source() // @source(param_rule)
	runRule(lib.Sink, x)
}

// no finding: the other caller passes a function that is not a sink
func CallNoop() {
	y := lib.Source()
	runRule(noop, y)
}

// the caller does not say what it passes: the flow stays (it may be the sink)
func unknown() func(string) { return lib.Sink }

func CallUnknown() {
	x := lib.Source() // @source(param_rule)
	runRule(unknown(), x)
}
