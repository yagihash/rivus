// Package global_field is a cell of the zoo.
//
// @cell(F/state/global-field)
// @shape: a function writes one field of a package variable that is a struct, and another function reads it; the decoy reads another field
package global_field

import "zoo/mark"

func init() {
	mark.Case("F/state/global-field/vuln", vuln)
	mark.Case("F/state/global-field/fixed", fixed)
	mark.Case("F/state/global-field/decoy", decoy)
}

type conf struct{ a, b string }

var gVuln conf

func readVuln() { mark.SinkCmd(gVuln.a) } // @sink(state-global-field-vuln)

func vuln() { // @vuln
	gVuln.a = mark.Src("state-global-field-vuln") // @source(state-global-field-vuln)
	readVuln()
}

var gFixed conf

func readFixed() { mark.SinkCmd(gFixed.a) } // @sink(none)

func fixed() { // @fixed
	gFixed.a = mark.CleanCmd(mark.Src("state-global-field-fixed"))
	readFixed()
}

var gDecoy conf

func readDecoyB() { mark.SinkCmd(gDecoy.b) } // @sink(none) @nosink(state-global-field-decoy)
func readDecoyA() { mark.SinkSQL(gDecoy.a) } // @sink(state-global-field-decoy)

func decoy() { // @decoy
	gDecoy.a = mark.Src("state-global-field-decoy") // @source(state-global-field-decoy)
	gDecoy.b = "constant"
	readDecoyB()
	readDecoyA()
}
