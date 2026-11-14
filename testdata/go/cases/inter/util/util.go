// Package util is a different package from its caller, to test summaries
// across package boundaries.
package util

import "example.com/cases/lib"

func Identity(s string) string { return s }

func Clean(s string) string { return lib.Sanitize(s) }

func Emit(s string) {
	lib.Sink(s) // @sink(crosspkg)
}

func FetchTainted() string {
	return lib.Source() // @source(crossret)
}
