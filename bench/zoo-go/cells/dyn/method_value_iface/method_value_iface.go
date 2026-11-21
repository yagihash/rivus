// Package method_value_iface is a cell of the zoo.
//
// @cell(F/dyn/method-value-iface)
// @shape: a method value of an interface (g := i.Get) whose implementation has a pointer receiver; the decoy binds two objects
package method_value_iface

import "zoo/mark"

func init() {
	mark.Case("F/dyn/method-value-iface/vuln", vuln)
	mark.Case("F/dyn/method-value-iface/fixed", fixed)
	mark.Case("F/dyn/method-value-iface/decoy", decoy)
}

type getter interface{ Get() string }

type box struct{ v string }

func (b *box) Get() string { return b.v }

func vuln() { // @vuln
	var i getter = &box{v: mark.Src("dyn-method-value-iface-vuln")} // @source(dyn-method-value-iface-vuln)
	g := i.Get
	mark.SinkCmd(g()) // @sink(dyn-method-value-iface-vuln)
}

func fixed() { // @fixed
	var i getter = &box{v: mark.CleanCmd(mark.Src("dyn-method-value-iface-fixed"))}
	g := i.Get
	mark.SinkCmd(g()) // @sink(none)
}

func decoy() { // @decoy
	var a getter = &box{v: mark.Src("dyn-method-value-iface-decoy")} // @source(dyn-method-value-iface-decoy)
	var b getter = &box{v: "constant"}
	ga, gb := a.Get, b.Get
	mark.SinkCmd(gb()) // @sink(none) @nosink(dyn-method-value-iface-decoy)
	mark.SinkSQL(ga()) // @sink(dyn-method-value-iface-decoy)
}
