package sanitizeown

import "example.com/cases/lib"

// A method of an interface that the rules call a sanitizer says only that its result is clean. The
// implementations of the project are read all the same: what they do to a sink, or to an argument, a
// receiver or a global, is theirs; only what they return is dropped (ADR-086).

// an implementation that returns what it was given: the rule says the result is clean
type passthrough struct{}

func (passthrough) Clean(s string) string { return s }

// no finding: the result of Clean is clean by the rule, whatever the body returns
func ResultIsClean() {
	lib.Sink(clean(passthrough{}, lib.Source()))
}

func clean(c lib.Cleaner, s string) string { return c.Clean(s) }

// an implementation that reaches a sink with what it was given
type sinking struct{}

func (sinking) Clean(s string) string {
	lib.Sink(s) // @sink(sanitizeown_sink)
	return ""
}

func SinkInBody() {
	var c lib.Cleaner = sinking{}
	_ = c.Clean(lib.Source()) // @source(sanitizeown_sink)
}

// an implementation that writes to a global
var held string

type globalKeeper struct{}

func (globalKeeper) Clean(s string) string {
	held = s
	return ""
}

func WriteToGlobal() {
	var c lib.Cleaner = globalKeeper{}
	_ = c.Clean(lib.Source()) // @source(sanitizeown_global)
}

func ReadGlobal() {
	lib.Sink(held) // @sink(sanitizeown_global)
}

// no finding: this implementation cleans before its sink
type careful struct{}

func (careful) Clean(s string) string {
	lib.Sink(lib.Sanitize(s))
	return ""
}

func Careful() {
	var c lib.Cleaner = careful{}
	_ = c.Clean(lib.Source())
}
