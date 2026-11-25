package givensink

import "slices"

func Sink(s string) bool { return false }

func Other(s string) bool { return false }

// Direct gives the function that the rules declare a sink to a function of the library that calls it.
func Direct(xs []string) {
	slices.IndexFunc(xs, Sink)
}

// Variable gives it through a variable.
func Variable(xs []string) {
	f := Sink
	slices.ContainsFunc(xs, f)
}

// Closure writes it as a closure: the sink is the call in it.
func Closure(xs []string) {
	slices.IndexFunc(xs, func(e string) bool { return Sink(e) })
}

// NotSink gives a function that is no sink.
func NotSink(xs []string) {
	slices.IndexFunc(xs, Other)
}

// Mixed has a call of the sink after the call that gives it.
func Mixed(xs []string, s string) {
	slices.IndexFunc(xs, Sink)
	Sink(s)
}
