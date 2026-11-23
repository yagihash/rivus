// Package local_fn_from_closure is a cell of the zoo.
//
// @cell(F/clos/local-fn-from-closure)
// @shape: a function value made in a function is called from a closure (a callback) in the same function, and the value reaches the sink only through it
package local_fn_from_closure

import "zoo/mark"

func init() {
	mark.Case("F/clos/local-fn-from-closure/vuln", vuln)
	mark.Case("F/clos/local-fn-from-closure/fixed", fixed)
	mark.Case("F/clos/local-fn-from-closure/decoy", decoy)
}

func each(f func()) { f() }

func vuln() { // @vuln
	emit := func(s string) {
		mark.SinkCmd(s) // @sink(clos-local-fn-from-closure-vuln)
	}
	v := mark.Src("clos-local-fn-from-closure-vuln") // @source(clos-local-fn-from-closure-vuln)
	each(func() {
		emit(v)
	})
}

func fixed() { // @fixed
	emit := func(s string) {
		mark.SinkCmd(s) // @sink(none)
	}
	v := mark.CleanCmd(mark.Src("clos-local-fn-from-closure-fixed"))
	each(func() {
		emit(v)
	})
}

func decoy() { // @decoy
	emit := func(a, b string) {
		mark.SinkCmd(b) // @sink(none) @nosink(clos-local-fn-from-closure-decoy)
		mark.SinkSQL(a) // @sink(clos-local-fn-from-closure-decoy)
	}
	v := mark.Src("clos-local-fn-from-closure-decoy") // @source(clos-local-fn-from-closure-decoy)
	each(func() {
		emit(v, "constant")
	})
}
