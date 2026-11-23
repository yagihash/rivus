// Package ptr_commaok_assert is a cell of the zoo.
//
// @cell(F/cont/ptr-commaok-assert)
// @shape: a pointer taken out of an interface by a comma-ok assertion, and a field written through it, is read from the interface again
package ptr_commaok_assert

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-commaok-assert/vuln", vuln)
	mark.Case("F/cont/ptr-commaok-assert/fixed", fixed)
	mark.Case("F/cont/ptr-commaok-assert/decoy", decoy)
}

func vuln() { // @vuln
	var x any = &box{}
	p, _ := x.(*box)
	p.v = mark.Src("cont-ptr-commaok-assert-vuln") // @source(cont-ptr-commaok-assert-vuln)
	mark.SinkCmd(x.(*box).v)                       // @sink(cont-ptr-commaok-assert-vuln)
}

func fixed() { // @fixed
	var x any = &box{}
	p, _ := x.(*box)
	p.v = mark.CleanCmd(mark.Src("cont-ptr-commaok-assert-fixed"))
	mark.SinkCmd(x.(*box).v) // @sink(none)
}

func decoy() { // @decoy
	var x, y any = &box{}, &box{}
	p, _ := x.(*box)
	p.v = mark.Src("cont-ptr-commaok-assert-decoy") // @source(cont-ptr-commaok-assert-decoy)
	mark.SinkCmd(y.(*box).v)                        // @sink(none)
	mark.SinkSQL(x.(*box).v)                        // @sink(cont-ptr-commaok-assert-decoy)
}
