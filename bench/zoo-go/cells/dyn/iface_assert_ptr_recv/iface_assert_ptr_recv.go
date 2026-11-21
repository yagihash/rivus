// Package iface_assert_ptr_recv is a cell of the zoo.
//
// @cell(F/dyn/iface-assert-ptr-recv)
// @shape: a pointer is held as an any, asserted to an interface (a type assertion), and read through it by a method with a pointer receiver
package iface_assert_ptr_recv

import "zoo/mark"

func init() {
	mark.Case("F/dyn/iface-assert-ptr-recv/vuln", vuln)
	mark.Case("F/dyn/iface-assert-ptr-recv/fixed", fixed)
	mark.Case("F/dyn/iface-assert-ptr-recv/decoy", decoy)
}

type getter interface{ Get() string }

type box struct{ v string }

func (b *box) Get() string { return b.v }

func vuln() { // @vuln
	var a any = &box{v: mark.Src("dyn-iface-assert-ptr-recv-vuln")} // @source(dyn-iface-assert-ptr-recv-vuln)
	g := a.(getter)
	mark.SinkCmd(g.Get()) // @sink(dyn-iface-assert-ptr-recv-vuln)
}

func fixed() { // @fixed
	var a any = &box{v: mark.CleanCmd(mark.Src("dyn-iface-assert-ptr-recv-fixed"))}
	g := a.(getter)
	mark.SinkCmd(g.Get()) // @sink(none)
}

func decoy() { // @decoy
	var a any = &box{v: mark.Src("dyn-iface-assert-ptr-recv-decoy")} // @source(dyn-iface-assert-ptr-recv-decoy)
	var b any = &box{v: "constant"}
	mark.SinkCmd(b.(getter).Get()) // @sink(none) @nosink(dyn-iface-assert-ptr-recv-decoy)
	mark.SinkSQL(a.(getter).Get()) // @sink(dyn-iface-assert-ptr-recv-decoy)
}
