// Package struct_field_opaque is a cell of the zoo.
//
// @cell(F/conv/struct-field-opaque)
// @shape: a sibling of the dangerous field is tainted, and the structure goes through a function that the analysis has no body and no model of, which moves it into the dangerous field (docs/DESIGN-field-sinks.md)
package struct_field_opaque

import "zoo/mark"

func init() {
	mark.Case("F/conv/struct-field-opaque/vuln", vuln)
	mark.Case("F/conv/struct-field-opaque/fixed", fixed)
	mark.Case("F/conv/struct-field-opaque/decoy", decoy)
}

func vuln() { // @vuln
	req := &mark.Stmt{Params: map[string]string{}}
	req.Params["body"] = mark.Src("conv-struct-field-opaque-vuln") // @source(conv-struct-field-opaque-vuln)
	mark.Opaque(req)
	mark.SinkStmt(req) // @sink(conv-struct-field-opaque-vuln)
}

func fixed() { // @fixed
	req := &mark.Stmt{Params: map[string]string{}}
	req.Params["body"] = mark.CleanSQL(mark.Src("conv-struct-field-opaque-fixed"))
	mark.Opaque(req)
	mark.SinkStmt(req) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("conv-struct-field-opaque-decoy") // @source(conv-struct-field-opaque-decoy)
	req := &mark.Stmt{SQL: "constant", Params: map[string]string{}}
	req.Params["body"] = t
	other := &mark.Stmt{}
	mark.Opaque(other)
	mark.SinkStmt(req) // @sink(none) @nosink(conv-struct-field-opaque-decoy)
	mark.SinkCmd(t)    // @sink(conv-struct-field-opaque-decoy)
}
