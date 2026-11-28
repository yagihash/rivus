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

// The same through an interface method: a call that goes to an implementation has the notes of the dispatch (the call is not
// exact), and the hop of the call that gets the element must still be the one that is not guarded.
type ider interface{ id(s S) S }

type impl struct{}

func (impl) id(s S) S { return s }

func innerDyn(flag bool, d ider) S {
	var s S
	s.B = get()
	a := ""
	if flag {
		a = get()
	}
	s.A = a
	return d.id(s)
}

func CallerDyn() {
	r := innerDyn(false, impl{})
	lib.Sink(r.B) // @sink(x)
}

func CallerDynTrue() {
	r := innerDyn(true, impl{})
	lib.Sink(r.B) // @sink(x)
}
