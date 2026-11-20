// Package globalw: W writes a source value to a global, R reads it into a
// sink. The path must go through W (CORE-B: today it does not).
package globalw

import "example.com/cases/lib"

var shared string

func W() { shared = lib.Source() }

func R() { lib.Sink(shared) }

func Main() {
	W()
	R()
}
