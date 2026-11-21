// Package stored is a cell of the zoo.
//
// @cell(F/clos/stored)
// @shape: a closure that captures a value is stored (a field, a package variable, a slice) and another function calls it
package stored

import "zoo/mark"

func init() {
	mark.Case("F/clos/stored/vuln", vuln)
	mark.Case("F/clos/stored/vuln2", vuln2)
	mark.Case("F/clos/stored/vuln3", vuln3)
	mark.Case("F/clos/stored/fixed", fixed)
	mark.Case("F/clos/stored/decoy", decoy)
}

type holder struct{ a, b func() }

func callA(h *holder) { h.a() }

// a field
func vuln() { // @vuln
	v := mark.Src("clos-stored-vuln") // @source(clos-stored-vuln)
	h := &holder{a: func() {
		mark.SinkCmd(v) // @sink(clos-stored-vuln)
	}}
	callA(h)
}

// a package variable
var gFn func()

func callG() { gFn() }

func vuln2() { // @vuln
	v := mark.Src("clos-stored-vuln2") // @source(clos-stored-vuln2)
	gFn = func() {
		mark.SinkCmd(v) // @sink(clos-stored-vuln2)
	}
	callG()
}

// a slice
func runAll(fs []func()) {
	for _, f := range fs {
		f()
	}
}

func vuln3() { // @vuln
	v := mark.Src("clos-stored-vuln3") // @source(clos-stored-vuln3)
	runAll([]func(){func() {
		mark.SinkCmd(v) // @sink(clos-stored-vuln3)
	}})
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-stored-fixed"))
	h := &holder{a: func() {
		mark.SinkCmd(v) // @sink(none)
	}}
	callA(h)
}

func callBoth(h *holder) {
	h.a()
	h.b()
}

func decoy() { // @decoy
	v := mark.Src("clos-stored-decoy") // @source(clos-stored-decoy)
	k := "constant"
	h := &holder{
		a: func() {
			mark.SinkCmd(k) // @sink(none) @nosink(clos-stored-decoy)
		},
		b: func() {
			mark.SinkSQL(v) // @sink(clos-stored-decoy)
		},
	}
	callBoth(h)
}
