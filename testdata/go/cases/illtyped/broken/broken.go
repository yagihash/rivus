// Package broken has a type error in one line. go/ssa builds nothing for it, nor for the packages that
// import it (user, top): what they hold is not in the analysis, and the output has to say so.
package broken

import "example.com/illtyped/lib"

// Count is the line that does not type-check.
var Count int = "three"

func Fetch() string { return lib.Source() }

func Other() string { return "x" }
