// Package reapply_loop is a cell of the zoo.
//
// @cell(F/guard/reapply-loop)
// @shape: a call is applied again in a loop, and the return after the loop depends on a parameter (the case runs both values)
package reapply_loop

import "zoo/mark"

func init() {
	mark.Case("F/guard/reapply-loop/vuln", vuln)
	mark.Case("F/guard/reapply-loop/fixed", fixed)
	mark.Case("F/guard/reapply-loop/decoy", decoy)
}

func pass(flag bool, s string) string {
	if flag {
		return s
	}
	return ""
}

// the call is applied again with what it gave back (r + s), and skip decides what is returned
func loop(skip bool, s string) string {
	r := ""
	for i := 0; i < 3; i++ {
		r = pass(true, r+s)
	}
	if skip {
		return ""
	}
	return r
}

func viaSink(skip bool, s string) {
	mark.SinkCmd(loop(skip, s)) // @sink(guard-reapply-loop-vuln)
}

func vuln() { // @vuln
	v := mark.Src("guard-reapply-loop-vuln") // @source(guard-reapply-loop-vuln)
	viaSink(false, v)
	viaSink(true, v)
	_ = pass(false, v) // the way of pass that gives back "" (the loop calls it with true)
}

func viaSinkFixed(skip bool, s string) {
	mark.SinkCmd(mark.CleanCmd(loop(skip, s))) // @sink(none)
}

func fixed() { // @fixed
	v := mark.Src("guard-reapply-loop-fixed")
	viaSinkFixed(false, v)
	viaSinkFixed(true, v)
}

// skip is true on the line of the command: the function gives back "" there, and the value reaches the other line only
func decoy() { // @decoy
	v := mark.Src("guard-reapply-loop-decoy") // @source(guard-reapply-loop-decoy)
	mark.SinkCmd(loop(true, v))               // @sink(none) @nosink(guard-reapply-loop-decoy)
	mark.SinkSQL(loop(false, v))              // @sink(guard-reapply-loop-decoy)
}
