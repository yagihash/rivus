// Package direct_in is a cell of the zoo.
//
// @cell(F/call/direct-in)
// @shape: the source is the type of a parameter of the handler (mark.In), and the handler passes its text to a sink
package direct_in

import "zoo/mark"

func init() {
	mark.Case("F/call/direct-in/vuln", vuln)
	mark.Case("F/call/direct-in/fixed", fixed)
	mark.Case("F/call/direct-in/decoy", decoy)
}

func handle(in mark.In) { // @source(call-direct-in-vuln)
	mark.SinkCmd(in.Value()) // @sink(call-direct-in-vuln)
}

func vuln() { // @vuln
	handle(mark.NewIn("call-direct-in-vuln"))
}

func handleFixed(in mark.In) {
	mark.SinkCmd(mark.CleanCmd(in.Value())) // @sink(none)
}

func fixed() { // @fixed
	handleFixed(mark.NewIn("call-direct-in-fixed"))
}

func handleDecoy(in mark.In, other string) { // @source(call-direct-in-decoy)
	mark.SinkCmd(other)      // @sink(none) @nosink(call-direct-in-decoy)
	mark.SinkCmd(in.Value()) // @sink(call-direct-in-decoy)
}

func decoy() { // @decoy
	handleDecoy(mark.NewIn("call-direct-in-decoy"), "constant")
}
