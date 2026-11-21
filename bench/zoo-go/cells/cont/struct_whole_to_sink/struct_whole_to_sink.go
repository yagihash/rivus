// Package struct_whole_to_sink is a cell of the zoo.
//
// @cell(F/cont/struct-whole-to-sink)
// @shape: a struct with a field that holds the value is given whole to a sink (SinkAny walks the fields)
package struct_whole_to_sink

import "zoo/mark"

func init() {
	mark.Case("F/cont/struct-whole-to-sink/vuln", vuln)
	mark.Case("F/cont/struct-whole-to-sink/fixed", fixed)
	mark.Case("F/cont/struct-whole-to-sink/decoy", decoy)
}

type pair struct{ a, b string }

// cleanAll sanitizes for every class: SinkAny judges the text by all of them
func cleanAll(s string) string {
	return mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(s))))
}

func vuln() { // @vuln
	p := pair{a: mark.Src("cont-struct-whole-to-sink-vuln"), b: "constant"} // @source(cont-struct-whole-to-sink-vuln)
	mark.SinkAny(p)                                                         // @sink(cont-struct-whole-to-sink-vuln)
}

func fixed() { // @fixed
	p := pair{a: cleanAll(mark.Src("cont-struct-whole-to-sink-fixed")), b: "constant"}
	mark.SinkAny(p) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("cont-struct-whole-to-sink-decoy") // @source(cont-struct-whole-to-sink-decoy)
	p := pair{a: "constant", b: "constant"}
	mark.SinkAny(p) // @sink(none) @nosink(cont-struct-whole-to-sink-decoy)
	mark.SinkCmd(t) // @sink(cont-struct-whole-to-sink-decoy)
}
