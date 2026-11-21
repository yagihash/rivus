package callbacks

import "example.com/cases/lib"

// A function of a library that calls the function it is given (a model with calls) is a call of that function:
// what it is called with is what the model says, and what it returns goes where the model says.

// The value the library passes arrives in the closure's parameter.
func PassedIn() {
	src := lib.Source() // @source(cb_passed)
	lib.Each([]string{src}, func(s string) {
		lib.Sink(s) // @sink(cb_passed)
	})
}

// What the closure returns comes out of the library, and a value it captured is one of them.
func ReturnedCaptured() {
	src := lib.Source() // @source(cb_captured)
	out := lib.Apply("constant", func(string) string { return src })
	lib.Sink(out) // @sink(cb_captured)
}

// What the library passes does not come out when the closure does not return it (the decoy of the flow that
// every argument comes out of the call).
func PassedNotReturned() {
	src := lib.Source() // @source(cb_decoy)
	out := lib.Apply(src, func(string) string { return "constant" })
	lib.Sink(out) // @sink(none)
}

// What it returns comes from what it was given.
func Through() {
	src := lib.Source() // @source(cb_through)
	out := lib.Apply(src, func(s string) string { return s + "!" })
	lib.Sink(out) // @sink(cb_through)
}

// A captured value reaches a sink in a closure that is run on its own.
func Background() {
	src := lib.Source() // @source(cb_go)
	lib.Later(func() {
		lib.Sink(src) // @sink(cb_go)
	})
}

// The function is a parameter: it cannot be told, and the flows that the row stands for are used (what goes in
// comes out), each use an assumed_call.
func Wrapper(f func(string) string, s string) string {
	return lib.Apply(s, f)
}

func NotKnown() {
	src := lib.Source() // @source(cb_unknown)
	out := Wrapper(func(s string) string { return "constant" }, src)
	lib.Sink(out) // @sink(cb_unknown)
}
