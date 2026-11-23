package numnone

import "example.com/cases/lib"

// Copy keeps the text whole: nothing is read as a number.
func Copy(s string) string { return s + "!" }

func Use() {
	lib.Sink(Copy(lib.Source()))
}
