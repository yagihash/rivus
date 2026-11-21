// Package method_value is a cell of the zoo.
//
// @cell(F/dyn/method-value)
// @shape: a method value (a bound method) is called; the receiver and the argument change places in the decoy
package method_value

import "zoo/mark"

func init() {
	mark.Case("F/dyn/method-value/vuln", vuln)
	mark.Case("F/dyn/method-value/fixed", fixed)
	mark.Case("F/dyn/method-value/decoy", decoy)
}

type printer struct{ prefix string }

func (p printer) Run(arg string) { mark.SinkCmd(arg) } // @sink(dyn-method-value-vuln)

func vuln() { // @vuln
	f := printer{prefix: "constant"}.Run
	f(mark.Src("dyn-method-value-vuln")) // @source(dyn-method-value-vuln)
}

type printerFixed struct{ prefix string }

func (p printerFixed) Run(arg string) { mark.SinkCmd(arg) } // @sink(none)

func fixed() { // @fixed
	f := printerFixed{prefix: "constant"}.Run
	f(mark.CleanCmd(mark.Src("dyn-method-value-fixed")))
}

type printerDecoy struct{ prefix string }

func (p printerDecoy) Run(arg string) {
	mark.SinkCmd(arg)      // @sink(none) @nosink(dyn-method-value-decoy)
	mark.SinkSQL(p.prefix) // @sink(dyn-method-value-decoy)
}

func decoy() { // @decoy
	f := printerDecoy{prefix: mark.Src("dyn-method-value-decoy")}.Run // @source(dyn-method-value-decoy)
	f("constant")
}
