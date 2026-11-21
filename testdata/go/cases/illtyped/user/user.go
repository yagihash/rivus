// Package user imports the package with the type error: it type-checks by itself, and is not built.
package user

import (
	"example.com/illtyped/broken"
	"example.com/illtyped/lib"
)

// Run would be a finding if the package were built.
func Run() {
	lib.Sink(broken.Fetch())
}

func Quiet() string { return broken.Other() }
