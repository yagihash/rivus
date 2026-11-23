// Package to_lib is a cell of the zoo.
//
// @cell(F/clos/to-lib)
// @shape: a closure that captures a value is given to a function of the standard library that calls it (sync.Once.Do, sync.OnceValue, filepath.Walk)
package to_lib

import (
	"io/fs"
	"path/filepath"
	"sync"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-lib/vuln", vuln)
	mark.Case("F/clos/to-lib/vuln2", vuln2)
	mark.Case("F/clos/to-lib/vuln3", vuln3)
	mark.Case("F/clos/to-lib/fixed", fixed)
	mark.Case("F/clos/to-lib/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("clos-to-lib-vuln") // @source(clos-to-lib-vuln)
	var once sync.Once
	once.Do(func() {
		mark.SinkCmd(v) // @sink(clos-to-lib-vuln)
	})
}

// vuln2: the result of the closure comes back through the library.
func vuln2() { // @vuln
	v := mark.Src("clos-to-lib-vuln2") // @source(clos-to-lib-vuln2) @miss(assumed_call) @ref(CALLBACK-MODEL)
	get := sync.OnceValue(func() string { return v })
	mark.SinkCmd(get()) // @sink(clos-to-lib-vuln2)
}

// vuln3: the function given to a walk is called with the root it was given (a name that is not there is called with, too).
func vuln3() { // @vuln
	v := mark.Src("clos-to-lib-vuln3") // @source(clos-to-lib-vuln3)
	_ = filepath.Walk(v, func(path string, _ fs.FileInfo, _ error) error {
		mark.SinkCmd(path) // @sink(clos-to-lib-vuln3)
		return nil
	})
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-to-lib-fixed"))
	var once sync.Once
	once.Do(func() {
		mark.SinkCmd(v) // @sink(none)
	})
}

func decoy() { // @decoy
	v := mark.Src("clos-to-lib-decoy") // @source(clos-to-lib-decoy) @miss(assumed_call) @ref(CALLBACK-MODEL)
	k := "constant"
	var once sync.Once
	once.Do(func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-lib-decoy)
	})
	get := sync.OnceValue(func() string { return v })
	mark.SinkSQL(get()) // @sink(clos-to-lib-decoy)
}
