// Package guardreapply holds a call that is applied again in a loop, so that the value it gives back comes to the call by two
// ways (from the loop variable, and from the argument). The element of that hop is not guarded, but the guard that is
// decided after the loop (the return that depends on skip) still holds: it must not be thrown away with the others.
package guardreapply

import "example.com/cases/lib"

func h(flag bool, s string) string {
	if flag {
		return s
	}
	return ""
}

func loop(skip bool, s string) string {
	r := ""
	for i := 0; i < 3; i++ {
		r = h(true, r+s)
	}
	if skip {
		return ""
	}
	return r
}

// skip is true: the function gives back "", nothing of the source reaches the sink.
func Skipped() {
	lib.Sink(loop(true, lib.Source())) // @sink(none)
}

// skip is false: the source is given back.
func Kept() {
	src := lib.Source()        // @source(kept)
	lib.Sink(loop(false, src)) // @sink(kept)
}
