// Package local: the same as ptr, with a local slice.
//
// KNOWN WRONG (CORE-A): runB -> os.Create is reported; there is no flow.
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
