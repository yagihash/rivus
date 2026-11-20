// Package global: the same as local, with a global (os.Args).
//
// No finding (ADR-048; CORE-A before: runB -> os.Create was reported, with a
// path with no step in runB and no word about the global).
package global

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { _ = len(args) }

func Main() {
	runA(os.Args[2:])
	runB(os.Args[2:])
}
