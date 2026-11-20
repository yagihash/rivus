// Package gwriters: several functions that do not call each other (components
// of the same level) write the same source value to a global, and R reads it
// into a sink. All the writers are candidates for the "first writer" the path
// starts at (ADR-049): which one it is must not depend on which goroutine
// finished first (TestDeterministic*).
package gwriters

import "example.com/cases/lib"

var shared string

func get() string { return lib.Source() }

func W1() { shared = get() }
func W2() { shared = get() }
func W3() { shared = get() }
func W4() { shared = get() }
func W5() { shared = get() }
func W6() { shared = get() }
func W7() { shared = get() }
func W8() { shared = get() }

func R() { lib.Sink(shared) }

func Main() {
	W8()
	W7()
	W6()
	W5()
	W4()
	W3()
	W2()
	W1()
	R()
}
