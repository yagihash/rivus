package hello

import "example.com/cases/lib"

func Direct() {
	s := lib.Source() // @source(a)
	lib.Sink(s)       // @sink(a)
}
