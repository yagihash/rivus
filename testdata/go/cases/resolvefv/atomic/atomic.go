// Package atomic holds a call of a function value that nothing resolves: the function was stored in an atomic.Value, which
// the call graph does not follow. -resolve-func-values runs the analysis of the function values for it (a run is
// made: the call has no candidate), and the model of atomic.Value does not say that a function value goes in and out of it,
// so nothing is carried and the result is the same. A Go call graph resolves nearly every call of a function value; the
// calls that -resolve-func-values gives candidates to are tested on the IR of testdata/ir/funcvalues.jsonl.
package atomic

import (
	"sync/atomic"

	"example.com/cases/lib"
)

var cell atomic.Value

func put() { cell.Store(func(s string) string { return s }) }

func Entry() {
	put()
	x := lib.Source() // @source(atomic)
	f := cell.Load().(func(string) string)
	lib.Sink(f(x)) // @sink(atomic)
}
