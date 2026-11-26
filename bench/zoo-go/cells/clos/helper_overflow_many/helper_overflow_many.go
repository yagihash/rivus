// Package helper_overflow_many is a cell of the zoo.
//
// @cell(F/clos/helper-overflow-many)
// @shape: thirty-four different closures are passed to one helper that calls its function argument, the same 34 closures as the Dart cell: Go writes this form as it is (a func literal passed to `transform`); the 32 of ADR-050 is the limit of the Dart value candidates, and Go takes the candidates from the call graph, so all 34 flows are found (the Dart cell records a depth_limit miss in issue 77; the Go cell records none)
package helper_overflow_many

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/clos/helper-overflow-many/vuln", vuln)
	mark.Case("F/clos/helper-overflow-many/vuln2", vuln2)
	mark.Case("F/clos/helper-overflow-many/vuln3", vuln3)
	mark.Case("F/clos/helper-overflow-many/vuln4", vuln4)
	mark.Case("F/clos/helper-overflow-many/vuln5", vuln5)
	mark.Case("F/clos/helper-overflow-many/vuln6", vuln6)
	mark.Case("F/clos/helper-overflow-many/vuln7", vuln7)
	mark.Case("F/clos/helper-overflow-many/vuln8", vuln8)
	mark.Case("F/clos/helper-overflow-many/vuln9", vuln9)
	mark.Case("F/clos/helper-overflow-many/vuln10", vuln10)
	mark.Case("F/clos/helper-overflow-many/vuln11", vuln11)
	mark.Case("F/clos/helper-overflow-many/vuln12", vuln12)
	mark.Case("F/clos/helper-overflow-many/vuln13", vuln13)
	mark.Case("F/clos/helper-overflow-many/vuln14", vuln14)
	mark.Case("F/clos/helper-overflow-many/vuln15", vuln15)
	mark.Case("F/clos/helper-overflow-many/vuln16", vuln16)
	mark.Case("F/clos/helper-overflow-many/vuln17", vuln17)
	mark.Case("F/clos/helper-overflow-many/vuln18", vuln18)
	mark.Case("F/clos/helper-overflow-many/vuln19", vuln19)
	mark.Case("F/clos/helper-overflow-many/vuln20", vuln20)
	mark.Case("F/clos/helper-overflow-many/vuln21", vuln21)
	mark.Case("F/clos/helper-overflow-many/vuln22", vuln22)
	mark.Case("F/clos/helper-overflow-many/vuln23", vuln23)
	mark.Case("F/clos/helper-overflow-many/vuln24", vuln24)
	mark.Case("F/clos/helper-overflow-many/vuln25", vuln25)
	mark.Case("F/clos/helper-overflow-many/vuln26", vuln26)
	mark.Case("F/clos/helper-overflow-many/vuln27", vuln27)
	mark.Case("F/clos/helper-overflow-many/vuln28", vuln28)
	mark.Case("F/clos/helper-overflow-many/vuln29", vuln29)
	mark.Case("F/clos/helper-overflow-many/vuln30", vuln30)
	mark.Case("F/clos/helper-overflow-many/vuln31", vuln31)
	mark.Case("F/clos/helper-overflow-many/vuln32", vuln32)
	mark.Case("F/clos/helper-overflow-many/vuln33", vuln33)
	mark.Case("F/clos/helper-overflow-many/vuln34", vuln34)
	mark.Case("F/clos/helper-overflow-many/fixed", fixed)
	mark.Case("F/clos/helper-overflow-many/decoy", decoy)
}

func transform(s string, f func(string) string) string { return f(s) }

func vuln() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln") // @source(clos-helper-overflow-many-vuln)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln)
		return x + "1"
	})
}

func vuln2() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln2") // @source(clos-helper-overflow-many-vuln2)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln2)
		return x + "2"
	})
}

func vuln3() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln3") // @source(clos-helper-overflow-many-vuln3)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln3)
		return x + "3"
	})
}

func vuln4() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln4") // @source(clos-helper-overflow-many-vuln4)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln4)
		return x + "4"
	})
}

func vuln5() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln5") // @source(clos-helper-overflow-many-vuln5)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln5)
		return x + "5"
	})
}

func vuln6() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln6") // @source(clos-helper-overflow-many-vuln6)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln6)
		return x + "6"
	})
}

func vuln7() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln7") // @source(clos-helper-overflow-many-vuln7)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln7)
		return x + "7"
	})
}

func vuln8() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln8") // @source(clos-helper-overflow-many-vuln8)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln8)
		return x + "8"
	})
}

func vuln9() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln9") // @source(clos-helper-overflow-many-vuln9)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln9)
		return x + "9"
	})
}

func vuln10() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln10") // @source(clos-helper-overflow-many-vuln10)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln10)
		return x + "10"
	})
}

func vuln11() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln11") // @source(clos-helper-overflow-many-vuln11)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln11)
		return x + "11"
	})
}

func vuln12() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln12") // @source(clos-helper-overflow-many-vuln12)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln12)
		return x + "12"
	})
}

func vuln13() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln13") // @source(clos-helper-overflow-many-vuln13)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln13)
		return x + "13"
	})
}

func vuln14() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln14") // @source(clos-helper-overflow-many-vuln14)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln14)
		return x + "14"
	})
}

func vuln15() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln15") // @source(clos-helper-overflow-many-vuln15)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln15)
		return x + "15"
	})
}

func vuln16() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln16") // @source(clos-helper-overflow-many-vuln16)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln16)
		return x + "16"
	})
}

func vuln17() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln17") // @source(clos-helper-overflow-many-vuln17)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln17)
		return x + "17"
	})
}

func vuln18() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln18") // @source(clos-helper-overflow-many-vuln18)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln18)
		return x + "18"
	})
}

func vuln19() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln19") // @source(clos-helper-overflow-many-vuln19)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln19)
		return x + "19"
	})
}

func vuln20() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln20") // @source(clos-helper-overflow-many-vuln20)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln20)
		return x + "20"
	})
}

func vuln21() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln21") // @source(clos-helper-overflow-many-vuln21)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln21)
		return x + "21"
	})
}

func vuln22() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln22") // @source(clos-helper-overflow-many-vuln22)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln22)
		return x + "22"
	})
}

func vuln23() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln23") // @source(clos-helper-overflow-many-vuln23)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln23)
		return x + "23"
	})
}

func vuln24() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln24") // @source(clos-helper-overflow-many-vuln24)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln24)
		return x + "24"
	})
}

func vuln25() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln25") // @source(clos-helper-overflow-many-vuln25)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln25)
		return x + "25"
	})
}

func vuln26() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln26") // @source(clos-helper-overflow-many-vuln26)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln26)
		return x + "26"
	})
}

func vuln27() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln27") // @source(clos-helper-overflow-many-vuln27)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln27)
		return x + "27"
	})
}

func vuln28() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln28") // @source(clos-helper-overflow-many-vuln28)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln28)
		return x + "28"
	})
}

func vuln29() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln29") // @source(clos-helper-overflow-many-vuln29)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln29)
		return x + "29"
	})
}

func vuln30() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln30") // @source(clos-helper-overflow-many-vuln30)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln30)
		return x + "30"
	})
}

func vuln31() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln31") // @source(clos-helper-overflow-many-vuln31)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln31)
		return x + "31"
	})
}

func vuln32() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln32") // @source(clos-helper-overflow-many-vuln32)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln32)
		return x + "32"
	})
}

func vuln33() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln33") // @source(clos-helper-overflow-many-vuln33)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln33)
		return x + "33"
	})
}

func vuln34() { // @vuln
	a := mark.Src("clos-helper-overflow-many-vuln34") // @source(clos-helper-overflow-many-vuln34)
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(clos-helper-overflow-many-vuln34)
		return x + "34"
	})
}

func fixed() { // @fixed
	a := mark.CleanCmd(mark.Src("clos-helper-overflow-many-fixed"))
	transform(a, func(x string) string {
		mark.SinkCmd(x) // @sink(none)
		return x + "f"
	})
}

func decoy() { // @decoy
	a := mark.Src("clos-helper-overflow-many-decoy") // @source(clos-helper-overflow-many-decoy)
	transform("constant", func(x string) string {
		mark.SinkCmd(x) // @sink(none) @nosink(clos-helper-overflow-many-decoy)
		return x + "d"
	})
	mark.SinkSQL(a) // @sink(clos-helper-overflow-many-decoy)
}
