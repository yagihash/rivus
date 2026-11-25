// Package helper_overflow is a cell of the zoo.
//
// @cell(F/clos/helper-overflow)
// @shape: ten different closures are passed to one helper that calls its function argument (more than eight candidates)
package helper_overflow

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/clos/helper-overflow/vuln", vuln)
	mark.Case("F/clos/helper-overflow/vuln2", vuln2)
	mark.Case("F/clos/helper-overflow/vuln3", vuln3)
	mark.Case("F/clos/helper-overflow/vuln4", vuln4)
	mark.Case("F/clos/helper-overflow/vuln5", vuln5)
	mark.Case("F/clos/helper-overflow/vuln6", vuln6)
	mark.Case("F/clos/helper-overflow/vuln7", vuln7)
	mark.Case("F/clos/helper-overflow/vuln8", vuln8)
	mark.Case("F/clos/helper-overflow/vuln9", vuln9)
	mark.Case("F/clos/helper-overflow/vuln10", vuln10)
	mark.Case("F/clos/helper-overflow/fixed", fixed)
	mark.Case("F/clos/helper-overflow/decoy", decoy)
}

func transform(s string, f func(string) string) string { return f(s) }

func vuln() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln") // @source(clos-helper-overflow-vuln)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln)
		return x + "1"
	})
}

func vuln2() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln2") // @source(clos-helper-overflow-vuln2)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln2)
		return x + "2"
	})
}

func vuln3() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln3") // @source(clos-helper-overflow-vuln3)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln3)
		return x + "3"
	})
}

func vuln4() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln4") // @source(clos-helper-overflow-vuln4)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln4)
		return x + "4"
	})
}

func vuln5() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln5") // @source(clos-helper-overflow-vuln5)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln5)
		return x + "5"
	})
}

func vuln6() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln6") // @source(clos-helper-overflow-vuln6)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln6)
		return x + "6"
	})
}

func vuln7() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln7") // @source(clos-helper-overflow-vuln7)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln7)
		return x + "7"
	})
}

func vuln8() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln8") // @source(clos-helper-overflow-vuln8)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln8)
		return x + "8"
	})
}

func vuln9() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln9") // @source(clos-helper-overflow-vuln9)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln9)
		return x + "9"
	})
}

func vuln10() { // @vuln
	a := mark.Src("clos-helper-overflow-vuln10") // @source(clos-helper-overflow-vuln10)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-vuln10)
		return x + "10"
	})
}

func fixed() { // @fixed
	a := mark.CleanCmd(mark.Src("clos-helper-overflow-fixed"))
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(none)
		return x + "f"
	})
}

func decoy() { // @decoy
	a := mark.Src("clos-helper-overflow-decoy") // @source(clos-helper-overflow-decoy)
	transform("constant", func(x string) string {
		mark.SinkCmd(x) // @sink(none) @nosink(clos-helper-overflow-decoy)
		return x + "d"
	})
	mark.SinkSQL(a) // @sink(clos-helper-overflow-decoy)
}
