// Package to_own_helper is a cell of the zoo.
//
// @cell(F/clos/to-own-helper)
// @shape: a closure that captures a value is given to a helper of the project that calls it; two callers with their own closures in the decoy
package to_own_helper

import "zoo/mark"

func init() {
	mark.Case("F/clos/to-own-helper/vuln", vuln)
	mark.Case("F/clos/to-own-helper/fixed", fixed)
	mark.Case("F/clos/to-own-helper/decoy", decoy)
}

func run(f func()) { f() }

func vuln() { // @vuln
	v := mark.Src("clos-to-own-helper-vuln") // @source(clos-to-own-helper-vuln)
	run(func() {
		mark.SinkCmd(v) // @sink(clos-to-own-helper-vuln)
	})
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-to-own-helper-fixed"))
	run(func() {
		mark.SinkCmd(v) // @sink(none)
	})
}

func decoy() { // @decoy
	v := mark.Src("clos-to-own-helper-decoy") // @source(clos-to-own-helper-decoy)
	k := "constant"
	run(func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-own-helper-decoy)
	})
	run(func() {
		mark.SinkSQL(v) // @sink(clos-to-own-helper-decoy)
	})
}
