// Package func_field is a cell of the zoo.
//
// @cell(F/dyn/func-field)
// @shape: a function value kept in a field of a struct is called; the struct has another function field with another sink
package func_field

import "zoo/mark"

func init() {
	mark.Case("F/dyn/func-field/vuln", vuln)
	mark.Case("F/dyn/func-field/fixed", fixed)
	mark.Case("F/dyn/func-field/decoy", decoy)
}

// Each case has its own function types, so that the cases of the cell do not meet at a call (the same type in
// one case is the point of the decoy).
type fnV func(string)
type fnF func(string)
type fnD func(string)

type handlersV struct{ cmd fnV }
type handlersF struct{ cmd fnF }
type handlersD struct{ cmd, sql fnD }

func vuln() { // @vuln
	v := mark.Src("dyn-func-field-vuln") // @source(dyn-func-field-vuln)
	h := handlersV{cmd: func(x string) {
		mark.SinkCmd(x) // @sink(dyn-func-field-vuln)
	}}
	h.cmd(v)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("dyn-func-field-fixed"))
	h := handlersF{cmd: func(x string) {
		mark.SinkCmd(x) // @sink(none)
	}}
	h.cmd(v)
}

func decoy() { // @decoy
	v := mark.Src("dyn-func-field-decoy") // @source(dyn-func-field-decoy)
	h := handlersD{
		cmd: func(x string) {
			mark.SinkCmd(x) // @sink(none) @nosink(dyn-func-field-decoy)
		},
		sql: func(x string) {
			mark.SinkSQL(x) // @sink(dyn-func-field-decoy)
		},
	}
	h.cmd("constant")
	h.sql(v)
}
