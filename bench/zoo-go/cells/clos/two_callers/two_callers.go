// Package two_callers is a cell of the zoo.
//
// @cell(F/clos/two-callers)
// @shape: one helper of the project calls the function it is given with a value; two callers give it their own closures
package two_callers

import "zoo/mark"

func init() {
	mark.Case("F/clos/two-callers/vuln", vuln)
	mark.Case("F/clos/two-callers/fixed", fixed)
	mark.Case("F/clos/two-callers/decoy", decoy)
}

func applyVuln(f func(string), s string) { f(s) }

func vuln() { // @vuln
	v := mark.Src("clos-two-callers-vuln") // @source(clos-two-callers-vuln)
	applyVuln(func(x string) {
		mark.SinkCmd(x) // @sink(clos-two-callers-vuln)
	}, v)
}

func applyFixed(f func(string), s string) { f(s) }

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-two-callers-fixed"))
	applyFixed(func(x string) {
		mark.SinkCmd(x) // @sink(none)
	}, v)
}

func applyDecoy(f func(string), s string) { f(s) }

func decoyA(s string) {
	applyDecoy(func(x string) {
		mark.SinkCmd(x) // @sink(none) @nosink(clos-two-callers-decoy)
	}, s)
}

func decoyB(s string) {
	applyDecoy(func(x string) {
		mark.SinkSQL(x) // @sink(clos-two-callers-decoy)
	}, s)
}

func decoy() { // @decoy
	v := mark.Src("clos-two-callers-decoy") // @source(clos-two-callers-decoy)
	decoyA("constant")
	decoyB(v)
}
