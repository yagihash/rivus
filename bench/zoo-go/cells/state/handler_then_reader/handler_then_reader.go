// Package handler_then_reader is a cell of the zoo.
//
// @cell(F/state/handler-then-reader)
// @shape: one entry point (a handler) writes a package variable from its parameter, and another entry point reads it
package handler_then_reader

import "zoo/mark"

func init() {
	mark.Case("F/state/handler-then-reader/vuln", vuln)
	mark.Case("F/state/handler-then-reader/fixed", fixed)
	mark.Case("F/state/handler-then-reader/decoy", decoy)
}

var lastVuln string

func writeVuln(in mark.In) { // @source(state-handler-then-reader-vuln)
	lastVuln = in.Value()
}

func readVuln() { mark.SinkCmd(lastVuln) } // @sink(state-handler-then-reader-vuln)

func vuln() { // @vuln
	writeVuln(mark.NewIn("state-handler-then-reader-vuln"))
	readVuln()
}

var lastFixed string

func writeFixed(in mark.In) {
	lastFixed = mark.CleanCmd(in.Value())
}

func readFixed() { mark.SinkCmd(lastFixed) } // @sink(none)

func fixed() { // @fixed
	writeFixed(mark.NewIn("state-handler-then-reader-fixed"))
	readFixed()
}

var lastDecoyA, lastDecoyB string

func writeDecoyA(in mark.In) { // @source(state-handler-then-reader-decoy)
	lastDecoyA = in.Value()
}

func writeDecoyB(s string) { lastDecoyB = s }

func readDecoy() {
	mark.SinkCmd(lastDecoyB) // @sink(none) @nosink(state-handler-then-reader-decoy)
	mark.SinkSQL(lastDecoyA) // @sink(state-handler-then-reader-decoy)
}

func decoy() { // @decoy
	writeDecoyA(mark.NewIn("state-handler-then-reader-decoy"))
	writeDecoyB("constant")
	readDecoy()
}
