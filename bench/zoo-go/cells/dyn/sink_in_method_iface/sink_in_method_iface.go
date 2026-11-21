// Package sink_in_method_iface is a cell of the zoo.
//
// @cell(F/dyn/sink-in-method-iface)
// @shape: a struct with a pointer receiver is held as an interface, and the sink is in the method that the interface calls
package sink_in_method_iface

import "zoo/mark"

func init() {
	mark.Case("F/dyn/sink-in-method-iface/vuln", vuln)
	mark.Case("F/dyn/sink-in-method-iface/fixed", fixed)
	mark.Case("F/dyn/sink-in-method-iface/decoy", decoy)
}

type execer interface{ Exec() }

type runner struct{ cmd string }

func (r *runner) Exec() { mark.SinkCmd(r.cmd) } // @sink(dyn-sink-in-method-iface-vuln)

func vuln() { // @vuln
	var e execer = &runner{cmd: mark.Src("dyn-sink-in-method-iface-vuln")} // @source(dyn-sink-in-method-iface-vuln)
	e.Exec()
}

type runnerFixed struct{ cmd string }

func (r *runnerFixed) Exec() { mark.SinkCmd(r.cmd) } // @sink(none)

func fixed() { // @fixed
	var e execer = &runnerFixed{cmd: mark.CleanCmd(mark.Src("dyn-sink-in-method-iface-fixed"))}
	e.Exec()
}

type runnerDecoy struct{ a, b string }

func (r *runnerDecoy) Exec() {
	mark.SinkCmd(r.b) // @sink(none) @nosink(dyn-sink-in-method-iface-decoy)
	mark.SinkSQL(r.a) // @sink(dyn-sink-in-method-iface-decoy)
}

func decoy() { // @decoy
	var e execer = &runnerDecoy{a: mark.Src("dyn-sink-in-method-iface-decoy"), b: "constant"} // @source(dyn-sink-in-method-iface-decoy)
	e.Exec()
}
