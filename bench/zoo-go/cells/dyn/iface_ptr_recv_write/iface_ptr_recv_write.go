// Package iface_ptr_recv_write is a cell of the zoo.
//
// @cell(F/dyn/iface-ptr-recv-write)
// @shape: a pointer receiver writes a field through an interface (i.Set(src)), and the original pointer is read afterwards
package iface_ptr_recv_write

import "zoo/mark"

func init() {
	mark.Case("F/dyn/iface-ptr-recv-write/vuln", vuln)
	mark.Case("F/dyn/iface-ptr-recv-write/fixed", fixed)
	mark.Case("F/dyn/iface-ptr-recv-write/decoy", decoy)
}

type setter interface{ Set(s string) }

type holder struct{ v string }

func (h *holder) Set(s string) { h.v = s }

func vuln() { // @vuln
	h := &holder{}
	var i setter = h
	i.Set(mark.Src("dyn-iface-ptr-recv-write-vuln")) // @source(dyn-iface-ptr-recv-write-vuln) @miss(silent) @ref(CORE-J)
	mark.SinkCmd(h.v)                                // @sink(dyn-iface-ptr-recv-write-vuln)
}

func fixed() { // @fixed
	h := &holder{}
	var i setter = h
	i.Set(mark.CleanCmd(mark.Src("dyn-iface-ptr-recv-write-fixed")))
	mark.SinkCmd(h.v) // @sink(none)
}

func decoy() { // @decoy
	h := &holder{}
	other := &holder{}
	var i setter = h
	i.Set(mark.Src("dyn-iface-ptr-recv-write-decoy")) // @source(dyn-iface-ptr-recv-write-decoy) @miss(silent) @ref(CORE-J)
	mark.SinkCmd(other.v)                             // @sink(none) @nosink(dyn-iface-ptr-recv-write-decoy)
	mark.SinkSQL(h.v)                                 // @sink(dyn-iface-ptr-recv-write-decoy)
}
