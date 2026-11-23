// Package other_kind is a cell of the zoo.
//
// @cell(F/san/other-kind)
// @shape: a sanitizer makes the value safe for the sinks of its own kind only (an HTML clean is no clean for a command or a query): the value goes on to the other kinds of sinks
package other_kind

import "zoo/mark"

func init() {
	mark.Case("F/san/other-kind/vuln", vuln)
	mark.Case("F/san/other-kind/vuln2", vuln2)
	mark.Case("F/san/other-kind/fixed", fixed)
	mark.Case("F/san/other-kind/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("san-other-kind-vuln") // @source(san-other-kind-vuln)
	mark.SinkCmd(mark.CleanHTML(v))      // @sink(san-other-kind-vuln)
}

func vuln2() { // @vuln
	v := mark.Src("san-other-kind-vuln2") // @source(san-other-kind-vuln2)
	mark.SinkHTML(mark.CleanCmd(v))       // @sink(san-other-kind-vuln2)
}

func fixed() { // @fixed
	v := mark.Src("san-other-kind-fixed")
	mark.SinkHTML(mark.CleanHTML(v)) // @sink(none)
	mark.SinkCmd(mark.CleanCmd(v))   // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("san-other-kind-decoy") // @source(san-other-kind-decoy)
	h := mark.CleanHTML(v)
	mark.SinkHTML(h) // @sink(none) @nosink(san-other-kind-decoy)
	mark.SinkSQL(h)  // @sink(san-other-kind-decoy)
}
