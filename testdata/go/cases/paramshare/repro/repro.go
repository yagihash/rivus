// Package repro: both functions are sources and both reach the sink: that is
// 2 real flows. rivus reports 2 findings (the right number), but with a global
// the path of the second goes through the first (CORE-B/CORE-C: the source
// function is missing from the path of one of them).
package repro

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { helper(args[0]) }

func Main() {
	runA(os.Args[2:])
	runB(os.Args[2:])
}
