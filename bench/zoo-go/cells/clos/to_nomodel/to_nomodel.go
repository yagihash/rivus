// Package to_nomodel is a cell of the zoo.
//
// @cell(F/clos/to-nomodel)
// @shape: a closure that captures a value is given to a function of the standard library (regexp's ReplaceAllStringFunc), and the result of the call is used
package to_nomodel

import (
	"container/ring"
	"regexp"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-nomodel/vuln", vuln)
	mark.Case("F/clos/to-nomodel/vuln2", vuln2)
	mark.Case("F/clos/to-nomodel/fixed", fixed)
	mark.Case("F/clos/to-nomodel/decoy", decoy)
}

var re = regexp.MustCompile("x")

func vuln() { // @vuln
	v := mark.Src("clos-to-nomodel-vuln") // @source(clos-to-nomodel-vuln)
	out := re.ReplaceAllStringFunc("xx", func(string) string { return v })
	mark.SinkCmd(out) // @sink(clos-to-nomodel-vuln)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-to-nomodel-fixed"))
	out := re.ReplaceAllStringFunc("xx", func(string) string { return v })
	mark.SinkCmd(out) // @sink(none)
	r := ring.New(1)
	r.Value = v
	r.Do(func(x any) {
		mark.SinkCmd(x.(string)) // @sink(none)
	})
}

func decoy() { // @decoy
	v := mark.Src("clos-to-nomodel-decoy") // @source(clos-to-nomodel-decoy)
	out := re.ReplaceAllStringFunc("xx", func(string) string { _ = v; return "constant" })
	mark.SinkCmd(out) // @sink(none) @nosink(clos-to-nomodel-decoy)
	mark.SinkSQL(v)   // @sink(clos-to-nomodel-decoy)
	r := ring.New(1)
	r.Value = "constant"
	r.Do(func(x any) {
		mark.SinkCmd(x.(string)) // @sink(none) @nosink(clos-to-nomodel-decoy)
	})
}

// vuln2: no result. What the receiver holds reaches the parameter of the function that is given (ring.Do has no row).
func vuln2() { // @vuln
	v := mark.Src("clos-to-nomodel-vuln2") // @source(clos-to-nomodel-vuln2)
	r := ring.New(1)
	r.Value = v
	r.Do(func(x any) {
		mark.SinkCmd(x.(string)) // @sink(clos-to-nomodel-vuln2)
	})
}
