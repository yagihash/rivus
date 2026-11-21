// Package direct is a cell of the zoo.
//
// @cell(F/call/direct)
// @shape: a value of a source goes to a sink in the same function
package direct

import "zoo/mark"

func init() {
	mark.Case("F/call/direct/vuln", vuln)
	mark.Case("F/call/direct/fixed", fixed)
	mark.Case("F/call/direct/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("call-direct-vuln") // @source(call-direct-vuln)
	mark.SinkCmd(v)                   // @sink(call-direct-vuln)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("call-direct-fixed"))
	mark.SinkCmd(v) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("call-direct-decoy") // @source(call-direct-decoy)
	mark.SinkCmd("constant")           // @sink(none) @nosink(call-direct-decoy)
	mark.SinkCmd(t)                    // @sink(call-direct-decoy)
}
