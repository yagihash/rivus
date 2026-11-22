// Package sink_as_value is a cell of the zoo.
//
// @cell(F/dyn/sink-as-value)
// @shape: a function that a rule names as a sink is made a function value (f := mark.SinkCmd) and the value is called
package sink_as_value

import "zoo/mark"

func init() {
	mark.Case("F/dyn/sink-as-value/vuln", vuln)
	mark.Case("F/dyn/sink-as-value/fixed", fixed)
	mark.Case("F/dyn/sink-as-value/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("dyn-sink-as-value-vuln") // @source(dyn-sink-as-value-vuln)
	f := mark.SinkCmd
	f(v) // @sink(dyn-sink-as-value-vuln)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("dyn-sink-as-value-fixed"))
	f := mark.SinkCmd
	f(v) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("dyn-sink-as-value-decoy") // @source(dyn-sink-as-value-decoy)
	f := mark.SinkCmd
	g := mark.SinkSQL
	f("constant") // @sink(none) @nosink(dyn-sink-as-value-decoy)
	g(v)          // @sink(dyn-sink-as-value-decoy)
}
