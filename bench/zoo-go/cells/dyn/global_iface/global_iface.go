// Package global_iface is a cell of the zoo.
//
// @cell(F/dyn/global-iface)
// @shape: a package variable of an interface type holds a pointer to a struct that holds the value, and another function reads it
package global_iface

import "zoo/mark"

func init() {
	mark.Case("F/dyn/global-iface/vuln", vuln)
	mark.Case("F/dyn/global-iface/fixed", fixed)
	mark.Case("F/dyn/global-iface/decoy", decoy)
}

type getter interface{ Get() string }

type box struct{ v string }

func (b *box) Get() string { return b.v }

var gVuln getter

func readVuln() { mark.SinkCmd(gVuln.Get()) } // @sink(dyn-global-iface-vuln)

func vuln() { // @vuln
	gVuln = &box{v: mark.Src("dyn-global-iface-vuln")} // @source(dyn-global-iface-vuln)
	readVuln()
}

var gFixed getter

func readFixed() { mark.SinkCmd(gFixed.Get()) } // @sink(none)

func fixed() { // @fixed
	gFixed = &box{v: mark.CleanCmd(mark.Src("dyn-global-iface-fixed"))}
	readFixed()
}

var gDecoyA, gDecoyB getter

func readDecoy() {
	mark.SinkCmd(gDecoyB.Get()) // @sink(none) @nosink(dyn-global-iface-decoy)
	mark.SinkSQL(gDecoyA.Get()) // @sink(dyn-global-iface-decoy)
}

func decoy() { // @decoy
	gDecoyA = &box{v: mark.Src("dyn-global-iface-decoy")} // @source(dyn-global-iface-decoy)
	gDecoyB = &box{v: "constant"}
	readDecoy()
}
