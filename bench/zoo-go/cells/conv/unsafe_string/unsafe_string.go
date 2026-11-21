// Package unsafe_string is a cell of the zoo.
//
// @cell(F/conv/unsafe-string)
// @shape: unsafe.String and unsafe.Slice make a value of the bytes that a pointer holds: the result is made of the pointer's value
package unsafe_string

import (
	"unsafe"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/unsafe-string/vuln", vuln)
	mark.Case("F/conv/unsafe-string/fixed", fixed)
	mark.Case("F/conv/unsafe-string/decoy", decoy)
}

func vuln() { // @vuln
	b := []byte(mark.Src("conv-unsafe-string-vuln")) // @source(conv-unsafe-string-vuln)
	mark.SinkCmd(unsafe.String(&b[0], len(b)))       // @sink(conv-unsafe-string-vuln)
}

func fixed() { // @fixed
	b := []byte(mark.CleanCmd(mark.Src("conv-unsafe-string-fixed")))
	mark.SinkCmd(string(unsafe.Slice(&b[0], len(b)))) // @sink(none)
}

func decoy() { // @decoy
	a := []byte(mark.Src("conv-unsafe-string-decoy")) // @source(conv-unsafe-string-decoy)
	c := []byte("constant")
	mark.SinkCmd(unsafe.String(&c[0], len(c))) // @sink(none) @nosink(conv-unsafe-string-decoy)
	mark.SinkSQL(unsafe.String(&a[0], len(a))) // @sink(conv-unsafe-string-decoy)
}
