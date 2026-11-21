// Package struct_field_shallow is a cell of the zoo.
//
// @cell(F/conv/struct-field-shallow)
// @shape: a statement (the SQL is the dangerous field, the bind parameters are not) is given whole to the call that runs it, and the rule of the sink names the field (docs/DESIGN-field-sinks.md)
package struct_field_shallow

import "zoo/mark"

func init() {
	mark.Case("F/conv/struct-field-shallow/vuln", vuln)
	mark.Case("F/conv/struct-field-shallow/fixed", fixed)
	mark.Case("F/conv/struct-field-shallow/decoy", decoy)
}

func vuln() { // @vuln
	req := &mark.Stmt{}
	req.SQL = mark.Src("conv-struct-field-shallow-vuln") // @source(conv-struct-field-shallow-vuln)
	mark.SinkStmt(req)                                   // @sink(conv-struct-field-shallow-vuln)
}

func fixed() { // @fixed
	req := &mark.Stmt{}
	req.SQL = mark.CleanSQL(mark.Src("conv-struct-field-shallow-fixed"))
	mark.SinkStmt(req) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("conv-struct-field-shallow-decoy") // @source(conv-struct-field-shallow-decoy)
	req := &mark.Stmt{SQL: "constant", Params: map[string]string{}}
	req.Params["p"] = t
	mark.SinkStmt(req) // @sink(none) @nosink(conv-struct-field-shallow-decoy)
	mark.SinkCmd(t)    // @sink(conv-struct-field-shallow-decoy)
}
