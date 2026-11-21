// Package global_direct is a cell of the zoo.
//
// @cell(F/state/global-direct)
// @shape: a function writes a package variable and another function reads it
package global_direct

import "zoo/mark"

func init() {
	mark.Case("F/state/global-direct/vuln", vuln)
	mark.Case("F/state/global-direct/fixed", fixed)
	mark.Case("F/state/global-direct/decoy", decoy)
}

var gVuln string

func readVuln() { mark.SinkCmd(gVuln) } // @sink(state-global-direct-vuln)

func vuln() { // @vuln
	gVuln = mark.Src("state-global-direct-vuln") // @source(state-global-direct-vuln)
	readVuln()
}

var gFixed string

func readFixed() { mark.SinkCmd(gFixed) } // @sink(none)

func fixed() { // @fixed
	gFixed = mark.CleanCmd(mark.Src("state-global-direct-fixed"))
	readFixed()
}

var gDecoyA, gDecoyB string

func readDecoy() {
	mark.SinkCmd(gDecoyB) // @sink(none) @nosink(state-global-direct-decoy)
	mark.SinkSQL(gDecoyA) // @sink(state-global-direct-decoy)
}

func decoy() { // @decoy
	gDecoyA = mark.Src("state-global-direct-decoy") // @source(state-global-direct-decoy)
	gDecoyB = "constant"
	readDecoy()
}
