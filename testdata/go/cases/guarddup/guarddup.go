// Package guarddup holds an element that a summary finds in two cells of the caller: one cell holds it behind a
// guard, the other holds it without one. The element is not guarded then.
package guarddup

import "example.com/cases/lib"

type S struct{ A, B string }

func get() string { return lib.Source() } // @source(x)

func id(s S) S { return s }

// s.A holds the source only when flag is true; s.B holds it always. Both cells hold the same element, and id(s)
// finds it in both: the guard of the first cell must not stay on what it gives back (r.B is tainted when flag is false).
func inner(flag bool) S {
	var s S
	s.B = get()
	a := ""
	if flag {
		a = get()
	}
	s.A = a
	return id(s)
}

func Caller() {
	r := inner(false)
	lib.Sink(r.B) // @sink(x)
}

func CallerTrue() {
	r := inner(true)
	lib.Sink(r.B) // @sink(x)
}
