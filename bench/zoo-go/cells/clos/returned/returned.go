// Package returned is a cell of the zoo.
//
// @cell(F/clos/returned)
// @shape: a function returns a closure that captures its argument, and the caller calls it
package returned

import "zoo/mark"

func init() {
	mark.Case("F/clos/returned/vuln", vuln)
	mark.Case("F/clos/returned/fixed", fixed)
	mark.Case("F/clos/returned/decoy", decoy)
}

func make1(s string) func() {
	return func() {
		mark.SinkCmd(s) // @sink(clos-returned-vuln)
	}
}

func vuln() { // @vuln
	make1(mark.Src("clos-returned-vuln"))() // @source(clos-returned-vuln)
}

func makeFixed(s string) func() {
	return func() {
		mark.SinkCmd(s) // @sink(none)
	}
}

func fixed() { // @fixed
	makeFixed(mark.CleanCmd(mark.Src("clos-returned-fixed")))()
}

func makeDecoy(a, b string) func() {
	return func() {
		mark.SinkCmd(b) // @sink(none) @nosink(clos-returned-decoy)
		mark.SinkSQL(a) // @sink(clos-returned-decoy)
	}
}

func decoy() { // @decoy
	makeDecoy(mark.Src("clos-returned-decoy"), "constant")() // @source(clos-returned-decoy)
}
