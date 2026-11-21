// Package capture_write_by_ref is a cell of the zoo.
//
// @cell(F/clos/capture-write-by-ref)
// @shape: a closure captures a variable by reference: the variable is written after the closure is made, or the closure writes it and the outside reads it; also a loop variable, and the closure given to a helper of the project or to sync.Once.Do
package capture_write_by_ref

import (
	"sync"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/capture-write-by-ref/vuln", vuln)
	mark.Case("F/clos/capture-write-by-ref/vuln2", vuln2)
	mark.Case("F/clos/capture-write-by-ref/vuln3", vuln3)
	mark.Case("F/clos/capture-write-by-ref/vuln4", vuln4)
	mark.Case("F/clos/capture-write-by-ref/vuln5", vuln5)
	mark.Case("F/clos/capture-write-by-ref/vuln6", vuln6)
	mark.Case("F/clos/capture-write-by-ref/vuln7", vuln7)
	mark.Case("F/clos/capture-write-by-ref/fixed", fixed)
	mark.Case("F/clos/capture-write-by-ref/decoy", decoy)
}

func run(f func()) { f() }

// vuln: written after the closure is made, then the closure is called.
func vuln() { // @vuln
	x := ""
	f := func() {
		mark.SinkCmd(x) // @sink(clos-capture-write-by-ref-vuln)
	}
	x = mark.Src("clos-capture-write-by-ref-vuln") // @source(clos-capture-write-by-ref-vuln)
	f()
}

// vuln2: the closure writes the variable; the outside reads it.
func vuln2() { // @vuln
	x := ""
	f := func() {
		x = mark.Src("clos-capture-write-by-ref-vuln2") // @source(clos-capture-write-by-ref-vuln2)
	}
	f()
	mark.SinkCmd(x) // @sink(clos-capture-write-by-ref-vuln2)
}

// vuln3: the loop variable (Go 1.22 and later: one variable for each iteration); the closures are called later.
func vuln3() { // @vuln
	var fs []func()
	for _, s := range []string{mark.Src("clos-capture-write-by-ref-vuln3")} { // @source(clos-capture-write-by-ref-vuln3)
		fs = append(fs, func() {
			mark.SinkCmd(s) // @sink(clos-capture-write-by-ref-vuln3)
		})
	}
	for _, f := range fs {
		f()
	}
}

// vuln4: written after the closure is made; a helper of the project calls it.
func vuln4() { // @vuln
	x := ""
	f := func() {
		mark.SinkCmd(x) // @sink(clos-capture-write-by-ref-vuln4)
	}
	x = mark.Src("clos-capture-write-by-ref-vuln4") // @source(clos-capture-write-by-ref-vuln4)
	run(f)
}

// vuln5: written after the closure is made; sync.Once.Do calls it.
func vuln5() { // @vuln
	x := ""
	f := func() {
		mark.SinkCmd(x) // @sink(clos-capture-write-by-ref-vuln5)
	}
	x = mark.Src("clos-capture-write-by-ref-vuln5") // @source(clos-capture-write-by-ref-vuln5)
	var once sync.Once
	once.Do(f)
}

// vuln6: the closure writes, a helper of the project calls it; the outside reads.
func vuln6() { // @vuln
	x := ""
	run(func() {
		x = mark.Src("clos-capture-write-by-ref-vuln6") // @source(clos-capture-write-by-ref-vuln6)
	})
	mark.SinkCmd(x) // @sink(clos-capture-write-by-ref-vuln6)
}

// vuln7: the closure writes, sync.Once.Do calls it; the outside reads.
func vuln7() { // @vuln
	x := ""
	var once sync.Once
	once.Do(func() {
		x = mark.Src("clos-capture-write-by-ref-vuln7") // @source(clos-capture-write-by-ref-vuln7)
	})
	mark.SinkCmd(x) // @sink(clos-capture-write-by-ref-vuln7)
}

func fixed() { // @fixed
	x := ""
	f := func() {
		mark.SinkCmd(x) // @sink(none)
	}
	x = mark.CleanCmd(mark.Src("clos-capture-write-by-ref-fixed"))
	f()
	y := ""
	run(func() {
		y = mark.CleanCmd(mark.Src("clos-capture-write-by-ref-fixed"))
	})
	mark.SinkCmd(y) // @sink(none)
}

func decoy() { // @decoy
	x := ""
	y := "constant"
	set := func() {
		x = mark.Src("clos-capture-write-by-ref-decoy") // @source(clos-capture-write-by-ref-decoy)
	}
	read := func() {
		mark.SinkSQL(x) // @sink(clos-capture-write-by-ref-decoy)
	}
	quiet := func() {
		mark.SinkCmd(y) // @sink(none) @nosink(clos-capture-write-by-ref-decoy)
	}
	run(set)
	run(read)
	run(quiet)
}
