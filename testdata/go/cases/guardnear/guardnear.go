// Package guardnear holds the shapes close to guardreapply: a guard that is nearer to the element than a hop that is not guarded
// holds, and one that is beyond it does not.
package guardnear

import "example.com/cases/lib"

func h(flag bool, s string) string {
	if flag {
		return s
	}
	return ""
}

// One of the two ways to the join has a guard (!skip2), the other has none: the element comes to r by both, so the guard of the
// first one must not take the flow away (skip2 is true and the source is still given back by b), and the return after the join
// (!skip) holds.
func join(skip, skip2 bool, s string) string {
	a := s
	if skip2 {
		a = ""
	}
	b := s
	r := a + b
	if skip {
		return ""
	}
	return r
}

func JoinStays() {
	src := lib.Source()              // @source(join)
	lib.Sink(join(false, true, src)) // @sink(join)
}

func JoinSkipped() {
	lib.Sink(join(true, true, lib.Source())) // @sink(none)
}

// Two hops that are marked (a call that is applied again in a loop, in a loop): the walk stops at the nearest of them, and the
// guard after the loops holds.
func nested(skip bool, s string) string {
	r := ""
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r = h(true, r+s)
		}
		r = h(true, r+s)
	}
	if skip {
		return ""
	}
	return r
}

func NestedStays() {
	src := lib.Source()          // @source(nested)
	lib.Sink(nested(false, src)) // @sink(nested)
}

func NestedSkipped() {
	lib.Sink(nested(true, lib.Source())) // @sink(none)
}

// The guard is in the loop: the call that is applied again is behind it.
func inLoop(skip bool, s string) string {
	r := ""
	for i := 0; i < 3; i++ {
		if skip {
			return ""
		}
		r = h(true, r+s)
	}
	return r
}

func InLoopStays() {
	src := lib.Source()          // @source(inloop)
	lib.Sink(inLoop(false, src)) // @sink(inloop)
}

func InLoopSkipped() {
	lib.Sink(inLoop(true, lib.Source())) // @sink(none)
}
