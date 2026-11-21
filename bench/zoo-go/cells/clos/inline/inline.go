// Package inline is a cell of the zoo.
//
// @cell(F/clos/inline)
// @shape: a closure is made and called where it is made
package inline

import "zoo/mark"

func init() {
	mark.Case("F/clos/inline/vuln", vuln)
	mark.Case("F/clos/inline/fixed", fixed)
	mark.Case("F/clos/inline/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("clos-inline-vuln") // @source(clos-inline-vuln)
	func() {
		mark.SinkCmd(v) // @sink(clos-inline-vuln)
	}()
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-inline-fixed"))
	func() {
		mark.SinkCmd(v) // @sink(none)
	}()
}

func decoy() { // @decoy
	v := mark.Src("clos-inline-decoy") // @source(clos-inline-decoy)
	k := "constant"
	func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-inline-decoy)
	}()
	func() {
		mark.SinkSQL(v) // @sink(clos-inline-decoy)
	}()
}
