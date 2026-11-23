package numnone

import "example.com/cases/lib"

// Copy keeps the text whole: nothing is read as a number.
func Copy(s string) string { return s + "!" }

// First reads one place of the text (a constant index): not text handled one character at a time.
func First(s string) string { return string(s[0]) + s[1:] }

func Use() {
	lib.Sink(First(lib.Source()))
	lib.Sink(Copy(lib.Source()))
}
