// Package range_func is a cell of the zoo.
//
// @cell(F/clos/range-func)
// @shape: range over a function (Go 1.23): the values that the iterator passes to yield are the loop variable
package range_func

import "zoo/mark"

func init() {
	mark.Case("F/clos/range-func/vuln", vuln)
	mark.Case("F/clos/range-func/fixed", fixed)
	mark.Case("F/clos/range-func/decoy", decoy)
}

func vuln() { // @vuln
	s := mark.Src("clos-range-func-vuln") // @source(clos-range-func-vuln)
	seq := func(yield func(string) bool) { yield(s) }
	for v := range seq {
		mark.SinkCmd(v) // @sink(clos-range-func-vuln)
	}
}

func fixed() { // @fixed
	s := mark.CleanCmd(mark.Src("clos-range-func-fixed"))
	seq := func(yield func(string) bool) { yield(s) }
	for v := range seq {
		mark.SinkCmd(v) // @sink(none)
	}
}

func decoy() { // @decoy
	a := mark.Src("clos-range-func-decoy") // @source(clos-range-func-decoy)
	seq := func(yield func(string) bool) { yield("constant") }
	for v := range seq {
		mark.SinkCmd(v) // @sink(none) @nosink(clos-range-func-decoy)
	}
	mark.SinkSQL(a) // @sink(clos-range-func-decoy)
}
