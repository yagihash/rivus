// Package generics holds sinks inside generic functions, methods of generic
// types and method values: their bodies are the program's own code, though
// go/ssa makes them without a package (yagihash/rivus #50).
package generics

import "example.com/cases/lib"

func SinkIn[T any](v T, s string) {
	lib.Sink(s) // @sink(generic-func)
}

type Box[T any] struct{ v T }

func (b *Box[T]) Put(s string) {
	lib.Sink(s) // @sink(generic-method)
}

type W struct{}

func (W) Do(s string) {
	lib.Sink(s) // @sink(bound)
}

func Generic() {
	a := lib.Source() // @source(generic-func)
	SinkIn(1, a)
}

func GenericMethod() {
	a := lib.Source() // @source(generic-method)
	(&Box[int]{}).Put(a)
}

func BoundMethod() {
	a := lib.Source() // @source(bound)
	f := W{}.Do
	f(a)
}
