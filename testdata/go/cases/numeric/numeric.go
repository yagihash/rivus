package numeric

import (
	"strconv"

	"example.com/cases/lib"
)

// An input that becomes a number is not followed: the place says so.
func Length() int {
	s := lib.Source()
	n, _ := strconv.Atoi(s)
	return n
}

// A source that never becomes a number leaves no such record.
func Plain() {
	lib.Sink(lib.Source())
}
