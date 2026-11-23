// Package local_recursion is a cell of the zoo.
//
// @cell(F/clos/local-recursion)
// @shape: a function declared inside another (a var that holds a closure) calls itself, and the value reaches the sink only through the recursive call
package local_recursion

import "zoo/mark"

func init() {
	mark.Case("F/clos/local-recursion/vuln", vuln)
	mark.Case("F/clos/local-recursion/fixed", fixed)
	mark.Case("F/clos/local-recursion/decoy", decoy)
}

func vuln() { // @vuln
	// the two arguments change places in each call: the first one reaches the sink in the second call
	var rec func(x, y string, n int)
	rec = func(x, y string, n int) {
		if n == 0 {
			mark.SinkCmd(y) // @sink(clos-local-recursion-vuln)
			return
		}
		rec(y, x, n-1)
	}
	rec(mark.Src("clos-local-recursion-vuln"), "constant", 1) // @source(clos-local-recursion-vuln)
}

func fixed() { // @fixed
	var rec func(x, y string, n int)
	rec = func(x, y string, n int) {
		if n == 0 {
			mark.SinkCmd(y) // @sink(none)
			return
		}
		rec(y, x, n-1)
	}
	rec(mark.CleanCmd(mark.Src("clos-local-recursion-fixed")), "constant", 1)
}

func decoy() { // @decoy
	var rec func(a, b string, n int)
	rec = func(a, b string, n int) {
		if n == 0 {
			mark.SinkCmd(b) // @sink(none) @nosink(clos-local-recursion-decoy)
			mark.SinkSQL(a) // @sink(clos-local-recursion-decoy)
			return
		}
		rec(a, b, n-1)
	}
	rec(mark.Src("clos-local-recursion-decoy"), "constant", 1) // @source(clos-local-recursion-decoy)
}
