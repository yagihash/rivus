// Package ptr: two functions get the same pointer; only runB is a source
// (kind: param) and only runA reaches the sink.
//
// KNOWN WRONG (CORE-A): rivus reports runB -> os.Create. The taint of the
// parameter writes back to main's object (OutParam) and runA then loads it.
// The right answer is no finding. See cmd/rivus/paramshare_test.go.
package ptr

import "os"

type Req struct{ Path string }

func helper(p string) { os.Create(p) }

func runA(r *Req) { helper(r.Path) }
func runB(r *Req) { _ = r }

func Main() {
	r := &Req{Path: "x"}
	runA(r)
	runB(r)
}
