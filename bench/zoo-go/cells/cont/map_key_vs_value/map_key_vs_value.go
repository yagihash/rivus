// Package map_key_vs_value is a cell of the zoo.
//
// @cell(F/cont/map-key-vs-value)
// @shape: a value is stored under one key of a map, and the decoy reads the other key
package map_key_vs_value

import "zoo/mark"

func init() {
	mark.Case("F/cont/map-key-vs-value/vuln", vuln)
	mark.Case("F/cont/map-key-vs-value/fixed", fixed)
	mark.Case("F/cont/map-key-vs-value/decoy", decoy)
}

func vuln() { // @vuln
	m := map[string]string{"a": mark.Src("cont-map-key-vs-value-vuln"), "b": "constant"} // @source(cont-map-key-vs-value-vuln)
	mark.SinkCmd(m["a"])                                                                 // @sink(cont-map-key-vs-value-vuln)
}

func fixed() { // @fixed
	m := map[string]string{"a": mark.CleanCmd(mark.Src("cont-map-key-vs-value-fixed")), "b": "constant"}
	mark.SinkCmd(m["a"]) // @sink(none)
}

func decoy() { // @decoy
	m := map[string]string{"a": mark.Src("cont-map-key-vs-value-decoy"), "b": "constant"} // @source(cont-map-key-vs-value-decoy)
	mark.SinkCmd(m["b"])                                                                  // @sink(none) @fp @nosink(cont-map-key-vs-value-decoy) @fp @ref(KEYED-MAP)
	mark.SinkSQL(m["a"])                                                                  // @sink(cont-map-key-vs-value-decoy)
}
