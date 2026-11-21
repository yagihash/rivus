// Package relay is a cell of the zoo.
//
// @cell(F/clos/relay)
// @shape: a function gives the function it was given to a helper that calls it (wrapper(g) calls run(g)); two callers give it their own closures
package relay

import "zoo/mark"

func init() {
	mark.Case("F/clos/relay/vuln", vuln)
	mark.Case("F/clos/relay/fixed", fixed)
	mark.Case("F/clos/relay/decoy", decoy)
}

type fnV func(string)

func runVuln(f fnV, s string)     { f(s) }
func wrapperVuln(g fnV, s string) { runVuln(g, s) }

func vuln() { // @vuln
	v := mark.Src("clos-relay-vuln") // @source(clos-relay-vuln)
	wrapperVuln(func(x string) {
		mark.SinkCmd(x) // @sink(clos-relay-vuln)
	}, v)
}

type fnF func(string)

func runFixed(f fnF, s string)     { f(s) }
func wrapperFixed(g fnF, s string) { runFixed(g, s) }

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-relay-fixed"))
	wrapperFixed(func(x string) {
		mark.SinkCmd(x) // @sink(none)
	}, v)
}

type fnD func(string)

func runDecoy(f fnD, s string)     { f(s) }
func wrapperDecoy(g fnD, s string) { runDecoy(g, s) }

func decoyA(s string) {
	wrapperDecoy(func(x string) {
		mark.SinkCmd(x) // @sink(none) @fp @nosink(clos-relay-decoy) @fp @ref(XTALK)
	}, s)
}

func decoyB(s string) {
	wrapperDecoy(func(x string) {
		mark.SinkSQL(x) // @sink(clos-relay-decoy)
	}, s)
}

func decoy() { // @decoy
	v := mark.Src("clos-relay-decoy") // @source(clos-relay-decoy)
	decoyA("constant")
	decoyB(v)
}
