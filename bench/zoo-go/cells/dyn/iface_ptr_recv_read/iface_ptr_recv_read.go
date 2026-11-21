// Package iface_ptr_recv_read is a cell of the zoo.
//
// @cell(F/dyn/iface-ptr-recv-read)
// @shape: a value is held by a struct with a pointer receiver, and is read through an interface
package iface_ptr_recv_read

import "zoo/mark"

func init() {
	mark.Case("F/dyn/iface-ptr-recv-read/vuln", vuln)
	mark.Case("F/dyn/iface-ptr-recv-read/fixed", fixed)
	mark.Case("F/dyn/iface-ptr-recv-read/decoy", decoy)
}

type getter interface{ Get() string }

type box struct{ v string }

func (b *box) Get() string { return b.v }

func vuln() { // @vuln
	var g getter = &box{v: mark.Src("dyn-iface-ptr-recv-read-vuln")} // @source(dyn-iface-ptr-recv-read-vuln)
	mark.SinkCmd(g.Get())                                            // @sink(dyn-iface-ptr-recv-read-vuln)
}

func fixed() { // @fixed
	var g getter = &box{v: mark.CleanCmd(mark.Src("dyn-iface-ptr-recv-read-fixed"))}
	mark.SinkCmd(g.Get()) // @sink(none)
}

func decoy() { // @decoy
	var a getter = &box{v: mark.Src("dyn-iface-ptr-recv-read-decoy")} // @source(dyn-iface-ptr-recv-read-decoy)
	var b getter = &box{v: "constant"}
	mark.SinkCmd(b.Get()) // @sink(none) @nosink(dyn-iface-ptr-recv-read-decoy)
	mark.SinkSQL(a.Get()) // @sink(dyn-iface-ptr-recv-read-decoy)
}
