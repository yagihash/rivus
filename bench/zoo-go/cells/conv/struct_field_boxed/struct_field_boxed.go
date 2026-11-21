// Package struct_field_boxed is a cell of the zoo.
//
// @cell(F/conv/struct-field-boxed)
// @shape: a statement is given to the sink as an interface (the pointer is in it: ADR-060), and the rule of the sink names the field (docs/DESIGN-field-sinks.md)
package struct_field_boxed

import "zoo/mark"

func init() {
	mark.Case("F/conv/struct-field-boxed/vuln", vuln)
	mark.Case("F/conv/struct-field-boxed/fixed", fixed)
	mark.Case("F/conv/struct-field-boxed/decoy", decoy)
}

func vuln() { // @vuln
	req := &mark.Stmt{}
	req.SQL = mark.Src("conv-struct-field-boxed-vuln") // @source(conv-struct-field-boxed-vuln)
	var v any = req
	mark.SinkStmtAny(v) // @sink(conv-struct-field-boxed-vuln)
}

func fixed() { // @fixed
	req := &mark.Stmt{}
	req.SQL = mark.CleanSQL(mark.Src("conv-struct-field-boxed-fixed"))
	var v any = req
	mark.SinkStmtAny(v) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("conv-struct-field-boxed-decoy") // @source(conv-struct-field-boxed-decoy)
	req := &mark.Stmt{SQL: "constant", Params: map[string]string{}}
	req.Params["p"] = t
	var v any = req
	mark.SinkStmtAny(v) // @sink(none) @nosink(conv-struct-field-boxed-decoy)
	mark.SinkCmd(t)     // @sink(conv-struct-field-boxed-decoy)
}
