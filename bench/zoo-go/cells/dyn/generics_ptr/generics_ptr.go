// Package generics_ptr is a cell of the zoo.
//
// @cell(F/dyn/generics-ptr)
// @shape: a generic type instantiated with string (*G[string]) with a pointer receiver, read through an interface
package generics_ptr

import "zoo/mark"

func init() {
	mark.Case("F/dyn/generics-ptr/vuln", vuln)
	mark.Case("F/dyn/generics-ptr/fixed", fixed)
	mark.Case("F/dyn/generics-ptr/decoy", decoy)
}

type getter interface{ Get() string }

type G[T any] struct{ v T }

func (g *G[T]) Get() T { return g.v }

func vuln() { // @vuln
	var i getter = &G[string]{v: mark.Src("dyn-generics-ptr-vuln")} // @source(dyn-generics-ptr-vuln)
	mark.SinkCmd(i.Get())                                           // @sink(dyn-generics-ptr-vuln)
}

func fixed() { // @fixed
	var i getter = &G[string]{v: mark.CleanCmd(mark.Src("dyn-generics-ptr-fixed"))}
	mark.SinkCmd(i.Get()) // @sink(none)
}

func decoy() { // @decoy
	var a getter = &G[string]{v: mark.Src("dyn-generics-ptr-decoy")} // @source(dyn-generics-ptr-decoy)
	var b getter = &G[string]{v: "constant"}
	mark.SinkCmd(b.Get()) // @sink(none) @nosink(dyn-generics-ptr-decoy)
	mark.SinkSQL(a.Get()) // @sink(dyn-generics-ptr-decoy)
}
