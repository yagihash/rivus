// Package sink_as_value_field is a cell of the zoo.
//
// @cell(F/dyn/sink-as-value-field)
// @shape: a function that a rule names as a sink is made a function value; the function value is stored in a field of a struct and called through the field
package sink_as_value_field

import "zoo/mark"

func init() {
	mark.Case("F/dyn/sink-as-value-field/vuln", vuln)
	mark.Case("F/dyn/sink-as-value-field/fixed", fixed)
	mark.Case("F/dyn/sink-as-value-field/decoy", decoy)
}

type holder struct{ cmd, sql func(string) }

func vuln() { // @vuln
	v := mark.Src("dyn-sink-as-value-field-vuln") // @source(dyn-sink-as-value-field-vuln)
	h := holder{cmd: mark.SinkCmd}
	h.cmd(v) // @sink(dyn-sink-as-value-field-vuln)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("dyn-sink-as-value-field-fixed"))
	h := holder{cmd: mark.SinkCmd}
	h.cmd(v) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("dyn-sink-as-value-field-decoy") // @source(dyn-sink-as-value-field-decoy)
	h := holder{cmd: mark.SinkCmd, sql: mark.SinkSQL}
	h.cmd("constant") // @sink(none) @nosink(dyn-sink-as-value-field-decoy)
	h.sql(v)          // @sink(dyn-sink-as-value-field-decoy)
}
