// Package depth3 is a cell of the zoo.
//
// @cell(F/call/depth3)
// @shape: a value goes through three calls (each passes it on) to a sink
package depth3

import "zoo/mark"

func init() {
	mark.Case("F/call/depth3/vuln", vuln)
	mark.Case("F/call/depth3/fixed", fixed)
	mark.Case("F/call/depth3/decoy", decoy)
}

func f1(s string) { f2(s) }
func f2(s string) { f3(s) }
func f3(s string) { mark.SinkCmd(s) } // @sink(call-depth3-vuln)

func vuln() { // @vuln
	f1(mark.Src("call-depth3-vuln")) // @source(call-depth3-vuln)
}

func g1(s string) { g2(s) }
func g2(s string) { g3(s) }
func g3(s string) { mark.SinkCmd(s) } // @sink(none)

func fixed() { // @fixed
	g1(mark.CleanCmd(mark.Src("call-depth3-fixed")))
}

// h1 passes both of its arguments down; only the second one reaches the sink.
func h1(a, b string) { h2(a, b) }
func h2(a, b string) { h3(a, b) }
func h3(a, b string) {
	mark.SinkCmd(b) // @sink(none) @nosink(call-depth3-decoy)
	mark.SinkSQL(a) // @sink(call-depth3-decoy)
}

func decoy() { // @decoy
	t := mark.Src("call-depth3-decoy") // @source(call-depth3-decoy)
	h1(t, "constant")
}
