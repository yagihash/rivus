// Package sink_as_value_map is a cell of the zoo.
//
// @cell(F/dyn/sink-as-value-map)
// @shape: a function that a rule names as a sink is made a function value; the function value is stored in a map and called by a constant key
package sink_as_value_map

import "zoo/mark"

func init() {
	mark.Case("F/dyn/sink-as-value-map/vuln", vuln)
	mark.Case("F/dyn/sink-as-value-map/fixed", fixed)
	mark.Case("F/dyn/sink-as-value-map/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("dyn-sink-as-value-map-vuln") // @source(dyn-sink-as-value-map-vuln)
	m := map[string]func(string){"cmd": mark.SinkCmd}
	m["cmd"](v) // @sink(dyn-sink-as-value-map-vuln)
}

func fixed() { // @fixed
	// cleaned for every kind (the four, one after the other): the function values that are read from a map may be any sink of the package
	v := mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dyn-sink-as-value-map-fixed")))))
	m := map[string]func(string){"cmd": mark.SinkCmd}
	m["cmd"](v) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("dyn-sink-as-value-map-decoy") // @source(dyn-sink-as-value-map-decoy)
	m := map[string]func(string){"cmd": mark.SinkCmd, "sql": mark.SinkSQL}
	m["cmd"]("constant") // @sink(none) @nosink(dyn-sink-as-value-map-decoy)
	m["sql"](v)          // @sink(dyn-sink-as-value-map-decoy)
}
