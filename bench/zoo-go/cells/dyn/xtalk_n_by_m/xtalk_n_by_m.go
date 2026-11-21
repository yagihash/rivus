// Package xtalk_n_by_m is a cell of the zoo.
//
// @cell(F/dyn/xtalk-n-by-m)
// @shape: several handlers take the same parameter type (a source), and each has its own sink
package xtalk_n_by_m

import "zoo/mark"

func init() {
	mark.Case("F/dyn/xtalk-n-by-m/vuln", vuln)
	mark.Case("F/dyn/xtalk-n-by-m/fixed", fixed)
	mark.Case("F/dyn/xtalk-n-by-m/decoy", decoy)
}

func handler(in mark.In) { // @source(dyn-xtalk-n-by-m-vuln)
	mark.SinkCmd(in.Value()) // @sink(dyn-xtalk-n-by-m-vuln)
}

func vuln() { // @vuln
	handler(mark.NewIn("dyn-xtalk-n-by-m-vuln"))
}

func handlerFixed(in mark.In) {
	mark.SinkCmd(mark.CleanCmd(in.Value())) // @sink(none)
}

func fixed() { // @fixed
	handlerFixed(mark.NewIn("dyn-xtalk-n-by-m-fixed"))
}

// three handlers of one parameter type: each source reaches its own sink and no other
func handlerA(in mark.In) { // @source(dyn-xtalk-n-by-m-decoy)
	mark.SinkCmd(in.Value()) // @sink(dyn-xtalk-n-by-m-decoy)
}

func handlerB(in mark.In) { // @source(dyn-xtalk-n-by-m-decoy-b)
	mark.SinkSQL(in.Value()) // @sink(dyn-xtalk-n-by-m-decoy-b)
}

func handlerC(in mark.In) {
	mark.SinkCmd("constant") // @sink(none) @nosink(dyn-xtalk-n-by-m-decoy) @nosink(dyn-xtalk-n-by-m-decoy-b)
	_ = in
}

func decoy() { // @decoy
	handlerA(mark.NewIn("dyn-xtalk-n-by-m-decoy"))
	handlerB(mark.NewIn("dyn-xtalk-n-by-m-decoy-b"))
	handlerC(mark.NewIn("dyn-xtalk-n-by-m-decoy-c"))
}
