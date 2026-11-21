// Package partial_callers is a cell of the zoo.
//
// @cell(F/clos/partial-callers)
// @shape: one helper calls the function it is given; one caller gives a closure made right there, another a function it reads from a field (a caller whose value cannot be followed)
package partial_callers

import "zoo/mark"

func init() {
	mark.Case("F/clos/partial-callers/vuln", vuln)
	mark.Case("F/clos/partial-callers/fixed", fixed)
	mark.Case("F/clos/partial-callers/decoy", decoy)
}

// Each case has its own function type, so that the cases of the cell do not meet at the call of a function value.
type (
	fnV func(string)
	fnF func(string)
	fnD func(string)
)

type (
	holderV struct{ cb fnV }
	holderF struct{ cb fnF }
	holderD struct{ cb fnD }
)

func applyVuln(f fnV, s string) { f(s) }

func vuln() { // @vuln
	v := mark.Src("clos-partial-callers-vuln") // @source(clos-partial-callers-vuln)
	applyVuln(func(x string) {}, "constant")
	h := holderV{cb: func(x string) {
		mark.SinkCmd(x) // @sink(clos-partial-callers-vuln)
	}}
	applyVuln(h.cb, v)
}

func applyFixed(f fnF, s string) { f(s) }

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-partial-callers-fixed"))
	applyFixed(func(x string) {}, "constant")
	h := holderF{cb: func(x string) {
		mark.SinkCmd(x) // @sink(none)
	}}
	applyFixed(h.cb, v)
}

func applyDecoy(f fnD, s string) { f(s) }

func decoy() { // @decoy
	v := mark.Src("clos-partial-callers-decoy") // @source(clos-partial-callers-decoy)
	h := holderD{cb: func(x string) {
		mark.SinkCmd(x) // @sink(none)
	}}
	applyDecoy(h.cb, "constant")
	applyDecoy(func(x string) {
		mark.SinkSQL(x) // @sink(clos-partial-callers-decoy)
	}, v)
}
