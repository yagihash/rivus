// Package slice is a cell of the zoo.
//
// @cell(F/cont/slice)
// @shape: a value is an element of a slice literal, and the element is read by its index (the decoy reads the other element)
package slice

import "zoo/mark"

func init() {
	mark.Case("F/cont/slice/vuln", vuln)
	mark.Case("F/cont/slice/fixed", fixed)
	mark.Case("F/cont/slice/decoy", decoy)
}

func vuln() { // @vuln
	s := []string{mark.Src("cont-slice-vuln"), "constant"} // @source(cont-slice-vuln)
	mark.SinkCmd(s[0])                                     // @sink(cont-slice-vuln)
}

func fixed() { // @fixed
	s := []string{mark.CleanCmd(mark.Src("cont-slice-fixed")), "constant"}
	mark.SinkCmd(s[0]) // @sink(none)
}

func decoy() { // @decoy
	s := []string{mark.Src("cont-slice-decoy"), "constant"} // @source(cont-slice-decoy)
	mark.SinkCmd(s[1])                                      // @sink(none) @fp @nosink(cont-slice-decoy) @fp @ref(KEYED-MAP)
	mark.SinkSQL(s[0])                                      // @sink(cont-slice-decoy)
}
