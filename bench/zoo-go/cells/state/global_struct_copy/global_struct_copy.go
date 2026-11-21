// Package global_struct_copy is a cell of the zoo.
//
// @cell(F/state/global-struct-copy)
// @shape: a function copies a package variable that is a struct, whole, and reads a field of the copy; the decoy reads another field
package global_struct_copy

import "zoo/mark"

func init() {
	mark.Case("F/state/global-struct-copy/vuln", vuln)
	mark.Case("F/state/global-struct-copy/fixed", fixed)
	mark.Case("F/state/global-struct-copy/decoy", decoy)
}

type conf struct {
	a, b string
	p    *string
}

var gVuln conf

func readVuln() {
	c := gVuln
	mark.SinkCmd(c.a) // @sink(state-global-struct-copy-vuln)
}

func vuln() { // @vuln
	gVuln.a = mark.Src("state-global-struct-copy-vuln") // @source(state-global-struct-copy-vuln)
	readVuln()
}

var gFixed conf

func readFixed() {
	c := gFixed
	mark.SinkCmd(c.a) // @sink(none)
}

func fixed() { // @fixed
	gFixed.a = mark.CleanCmd(mark.Src("state-global-struct-copy-fixed"))
	readFixed()
}

var gDecoy conf

func readDecoy() {
	c := gDecoy
	mark.SinkCmd(c.b) // @sink(none) @nosink(state-global-struct-copy-decoy)
	mark.SinkSQL(c.a) // @sink(state-global-struct-copy-decoy)
}

func decoy() { // @decoy
	gDecoy.a = mark.Src("state-global-struct-copy-decoy") // @source(state-global-struct-copy-decoy)
	gDecoy.b = "constant"
	readDecoy()
}
