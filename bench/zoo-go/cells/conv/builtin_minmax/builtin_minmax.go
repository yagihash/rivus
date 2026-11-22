// Package builtin_minmax is a cell of the zoo.
//
// @cell(F/conv/builtin-minmax)
// @shape: min / max of strings give one of their arguments: the value that is the result
package builtin_minmax

import "zoo/mark"

func init() {
	mark.Case("F/conv/builtin-minmax/vuln", vuln)
	mark.Case("F/conv/builtin-minmax/fixed", fixed)
	mark.Case("F/conv/builtin-minmax/decoy", decoy)
}

func vuln() { // @vuln
	s := mark.Src("conv-builtin-minmax-vuln") // @source(conv-builtin-minmax-vuln)
	mark.SinkCmd(min(s, "zzz"))               // @sink(conv-builtin-minmax-vuln)
}

func fixed() { // @fixed
	s := mark.CleanCmd(mark.Src("conv-builtin-minmax-fixed"))
	mark.SinkCmd(max("", s)) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-builtin-minmax-decoy") // @source(conv-builtin-minmax-decoy)
	mark.SinkCmd(min("b", "a"))                // @sink(none) @nosink(conv-builtin-minmax-decoy)
	mark.SinkSQL(min(a, "c"))                  // @sink(conv-builtin-minmax-decoy)
}
