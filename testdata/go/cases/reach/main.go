// A program: it starts at main. What main cannot reach does not run.
package main

import "example.com/cases/lib"

func main() {
	used()
}

func used() {
	s := lib.Source() // @source(reached)
	lib.Sink(s)       // @sink(reached)
}

// orphan is called by nothing: a helper that only a test used, say.
func orphan() {
	s := lib.Source() // @source(orphan)
	lib.Sink(s)       // @sink(orphan)
}

type thing struct{}

func (thing) Run() {
	s := lib.Source() // @source(method)
	lib.Sink(s)       // @sink(method)
}

type runner interface{ Run() }

// viaInterface is reached through an interface: its candidate is thing.Run.
func viaInterface(r runner) { r.Run() }

func init() {
	viaInterface(thing{})
}
