// Package to_lib_call is a cell of the zoo.
//
// @cell(F/clos/to-lib-call)
// @shape: a closure that captures a value is made into another function value by the standard library (sync.OnceFunc, http.HandlerFunc), and the result is called: the closure runs in that call
package to_lib_call

import (
	"net/http"
	"sync"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-lib-call/vuln", vuln)
	mark.Case("F/clos/to-lib-call/vuln2", vuln2)
	mark.Case("F/clos/to-lib-call/fixed", fixed)
	mark.Case("F/clos/to-lib-call/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("clos-to-lib-call-vuln") // @source(clos-to-lib-call-vuln)
	f := sync.OnceFunc(func() {
		mark.SinkCmd(v) // @sink(clos-to-lib-call-vuln)
	})
	f()
}

// vuln2: the method of a function type calls the function.
func vuln2() { // @vuln
	v := mark.Src("clos-to-lib-call-vuln2") // @source(clos-to-lib-call-vuln2)
	h := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		mark.SinkCmd(v) // @sink(clos-to-lib-call-vuln2)
	})
	h.ServeHTTP(nil, nil)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-to-lib-call-fixed"))
	f := sync.OnceFunc(func() {
		mark.SinkCmd(v) // @sink(none)
	})
	f()
	h := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		mark.SinkCmd(v) // @sink(none)
	})
	h.ServeHTTP(nil, nil)
}

func decoy() { // @decoy
	v := mark.Src("clos-to-lib-call-decoy") // @source(clos-to-lib-call-decoy)
	k := "constant"
	f := sync.OnceFunc(func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-lib-call-decoy)
	})
	f()
	h := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-lib-call-decoy)
	})
	h.ServeHTTP(nil, nil)
	mark.SinkSQL(v) // @sink(clos-to-lib-call-decoy)
}
