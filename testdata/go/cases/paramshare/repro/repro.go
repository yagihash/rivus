// Package repro: both functions are sources and both reach the sink: that is
// 2 real flows, 2 findings, each with a step in its own source function (before
// ADR-048 the path of the second went through the first: CORE-B/CORE-C).
package repro

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { helper(args[0]) }

func Main() {
	runA(os.Args[2:])
	runB(os.Args[2:])
}
