// Package sink_in_method is a cell of the zoo.
//
// @cell(F/dyn/sink-in-method)
// @shape: a constructor stores the value in a struct, and the sink is in a method of the struct
package sink_in_method

import "zoo/mark"

func init() {
	mark.Case("F/dyn/sink-in-method/vuln", vuln)
	mark.Case("F/dyn/sink-in-method/fixed", fixed)
	mark.Case("F/dyn/sink-in-method/decoy", decoy)
}

type runner struct{ cmd string }

func newRunner(c string) *runner { return &runner{cmd: c} }

func (r *runner) Exec() { mark.SinkCmd(r.cmd) } // @sink(dyn-sink-in-method-vuln)

func vuln() { // @vuln
	newRunner(mark.Src("dyn-sink-in-method-vuln")).Exec() // @source(dyn-sink-in-method-vuln)
}

type runnerFixed struct{ cmd string }

func (r *runnerFixed) Exec() { mark.SinkCmd(r.cmd) } // @sink(none)

func fixed() { // @fixed
	(&runnerFixed{cmd: mark.CleanCmd(mark.Src("dyn-sink-in-method-fixed"))}).Exec()
}

type runnerDecoy struct{ a, b string }

func (r *runnerDecoy) Exec() {
	mark.SinkCmd(r.b) // @sink(none) @nosink(dyn-sink-in-method-decoy)
	mark.SinkSQL(r.a) // @sink(dyn-sink-in-method-decoy)
}

func decoy() { // @decoy
	r := &runnerDecoy{a: mark.Src("dyn-sink-in-method-decoy"), b: "constant"} // @source(dyn-sink-in-method-decoy)
	r.Exec()
}
