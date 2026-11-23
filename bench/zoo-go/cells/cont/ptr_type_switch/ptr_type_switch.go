// Package ptr_type_switch is a cell of the zoo.
//
// @cell(F/cont/ptr-type-switch)
// @shape: a pointer bound by a type switch, and a field written through it, is read from the interface again
package ptr_type_switch

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-type-switch/vuln", vuln)
	mark.Case("F/cont/ptr-type-switch/fixed", fixed)
	mark.Case("F/cont/ptr-type-switch/decoy", decoy)
}

func vuln() { // @vuln
	var x any = &box{}
	switch p := x.(type) {
	case *box:
		p.v = mark.Src("cont-ptr-type-switch-vuln") // @source(cont-ptr-type-switch-vuln)
	}
	mark.SinkCmd(x.(*box).v) // @sink(cont-ptr-type-switch-vuln)
}

func fixed() { // @fixed
	var x any = &box{}
	switch p := x.(type) {
	case *box:
		p.v = mark.CleanCmd(mark.Src("cont-ptr-type-switch-fixed"))
	}
	mark.SinkCmd(x.(*box).v) // @sink(none)
}

func decoy() { // @decoy
	var x, y any = &box{}, &box{}
	switch p := x.(type) {
	case *box:
		p.v = mark.Src("cont-ptr-type-switch-decoy") // @source(cont-ptr-type-switch-decoy)
	}
	mark.SinkCmd(y.(*box).v) // @sink(none)
	mark.SinkSQL(x.(*box).v) // @sink(cont-ptr-type-switch-decoy)
}
