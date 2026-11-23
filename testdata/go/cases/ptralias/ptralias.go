// Package ptralias: a write through a pointer that is held in a cell. The analysis of Rust approximates it by
// aliasing (ptrAlias); the test turns that on for this Go program (sg.Program.HasEnv) to look at the note that
// the findings carry.
package ptralias

import "example.com/cases/lib"

type slot struct{ p *string }

// ThroughSlot writes x through the pointer held in a field of the struct: the flow to x is the one the aliasing
// makes.
func ThroughSlot() {
	x := ""
	s := &slot{}
	s.p = &x
	*s.p = lib.Source()
	lib.Sink(x) // @sink(ptralias-through)
}

// Direct has no pointer in a cell: the same kind of finding, with no aliasing in the path.
func Direct() {
	y := lib.Source()
	lib.Sink(y) // @sink(ptralias-direct)
}
