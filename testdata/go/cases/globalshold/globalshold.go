// Package globalshold: a package variable that the rules say holds nothing (os.Stderr: what is written to it
// goes out of the program, nobody reads it back). Without the rule, what one function writes to os.Stderr is
// carried by every function that hands os.Stderr on (the logger of each library), and by everything those are
// stored in: all the code would be tied together by one variable.
package globalshold

import (
	"fmt"
	"log"
	"os"

	"example.com/cases/lib"
)

// LogIt writes tainted data to the standard error.
func LogIt() {
	x := lib.Source()
	fmt.Fprintln(os.Stderr, x)
}

var logger = log.New(os.Stderr, "", 0)

// UseLogger uses the logger that was made from os.Stderr: nothing of what LogIt wrote is in it.
func UseLogger() {
	lib.Sink(fmt.Sprint(logger)) // @sink(none)
}

// WriteAndRead reads, in the function that wrote, what it wrote: inside one function the variable is a variable
// like any other (what is not followed is what leaves the function through it).
func WriteAndRead() {
	x := lib.Source() // @source(same)
	fmt.Fprintln(os.Stderr, x)
	lib.Sink(fmt.Sprint(os.Stderr)) // @sink(same)
}

// The tainted data that is written to a variable that holds nothing is still seen as a call: a rule that names
// fmt.Fprintln as a sink would catch it there.
//
// What the rule does not follow, and no fixture can say (the analysis is silent about it): a program that reads
// its own output back (os.Pipe and os.Stdout replaced by the writer of the pipe, then read from the other end)
// loses the flow. A rule file says "holds: everything" for the variable to get it back.
