package variadicdst

import "example.com/cases/lib"

// A model that sends data into a destination that is a variadic argument (row.Columns(&a, &b)): the data goes to
// what the pointers in the slice point to, not only to the slice.

func IntoTwo() {
	var a, b string
	lib.ReadInto(lib.Source(), &a, &b) // @source(variadicdst_two)
	lib.Sink(b)                        // @sink(variadicdst_two)
}

func IntoOne() {
	var a string
	lib.ReadInto(lib.Source(), &a) // @source(variadicdst_one)
	lib.Sink(a)                    // @sink(variadicdst_one)
}

// a variable that the call is not given
func Other() {
	var a, c string
	lib.ReadInto(lib.Source(), &a)
	lib.Sink(c)
}
