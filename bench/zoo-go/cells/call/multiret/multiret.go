// Package multiret is a cell of the zoo.
//
// @cell(F/call/multiret)
// @shape: a function returns two values, and the caller uses one of them
package multiret

import "zoo/mark"

func init() {
	mark.Case("F/call/multiret/vuln", vuln)
	mark.Case("F/call/multiret/fixed", fixed)
	mark.Case("F/call/multiret/decoy", decoy)
}

func pair(a, b string) (string, string) { return a, b }

func vuln() { // @vuln
	x, _ := pair(mark.Src("call-multiret-vuln"), "constant") // @source(call-multiret-vuln)
	mark.SinkCmd(x)                                          // @sink(call-multiret-vuln)
}

func fixed() { // @fixed
	x, _ := pair(mark.CleanCmd(mark.Src("call-multiret-fixed")), "constant")
	mark.SinkCmd(x) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("call-multiret-decoy") // @source(call-multiret-decoy)
	x, y := pair("constant", t)
	mark.SinkCmd(x) // @sink(none) @nosink(call-multiret-decoy)
	mark.SinkSQL(y) // @sink(call-multiret-decoy)
}
