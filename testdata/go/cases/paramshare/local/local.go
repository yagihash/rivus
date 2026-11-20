// Package local: the same as ptr, with a local slice.
//
// No finding (ADR-048; CORE-A before: runB -> os.Create was reported).
package local

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { _ = len(args) }

func Main() {
	a := []string{"x"}
	runA(a)
	runB(a)
}
