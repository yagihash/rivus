// Package iface_val_recv is a cell of the zoo.
//
// @cell(F/dyn/iface-val-recv)
// @shape: a value is held by a struct with a value receiver, and is read through an interface
package iface_val_recv

import "zoo/mark"

func init() {
	mark.Case("F/dyn/iface-val-recv/vuln", vuln)
	mark.Case("F/dyn/iface-val-recv/fixed", fixed)
	mark.Case("F/dyn/iface-val-recv/decoy", decoy)
}

type getter interface{ Get() string }

type box struct{ v string }

func (b box) Get() string { return b.v }

func vuln() { // @vuln
	var g getter = box{v: mark.Src("dyn-iface-val-recv-vuln")} // @source(dyn-iface-val-recv-vuln)
	mark.SinkCmd(g.Get())                                      // @sink(dyn-iface-val-recv-vuln)
}

func fixed() { // @fixed
	var g getter = box{v: mark.CleanCmd(mark.Src("dyn-iface-val-recv-fixed"))}
	mark.SinkCmd(g.Get()) // @sink(none)
}

func decoy() { // @decoy
	var a getter = box{v: mark.Src("dyn-iface-val-recv-decoy")} // @source(dyn-iface-val-recv-decoy)
	var b getter = box{v: "constant"}
	mark.SinkCmd(b.Get()) // @sink(none) @nosink(dyn-iface-val-recv-decoy)
	mark.SinkSQL(a.Get()) // @sink(dyn-iface-val-recv-decoy)
}
