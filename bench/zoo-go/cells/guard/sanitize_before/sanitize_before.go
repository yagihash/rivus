// Package sanitize_before is a cell of the zoo.
//
// @cell(F/guard/sanitize-before)
// @shape: the value is sanitized, and then used (the vulnerable case uses it before; the decoy sanitizes a copy and the original goes on)
package sanitize_before

import "zoo/mark"

func init() {
	mark.Case("F/guard/sanitize-before/vuln", vuln)
	mark.Case("F/guard/sanitize-before/fixed", fixed)
	mark.Case("F/guard/sanitize-before/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("guard-sanitize-before-vuln") // @source(guard-sanitize-before-vuln)
	mark.SinkCmd(v)                             // @sink(guard-sanitize-before-vuln)
	v = mark.CleanCmd(v)
	mark.SinkCmd(v) // @sink(none)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("guard-sanitize-before-fixed"))
	mark.SinkCmd(v) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("guard-sanitize-before-decoy") // @source(guard-sanitize-before-decoy)
	b := mark.CleanCmd(a)
	mark.SinkCmd(b) // @sink(none) @nosink(guard-sanitize-before-decoy)
	mark.SinkSQL(a) // @sink(guard-sanitize-before-decoy)
}
