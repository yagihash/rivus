// Package global: the same as local, with a global (os.Args).
//
// KNOWN WRONG (CORE-A): runB -> os.Create is reported; there is no flow.
// KNOWN WRONG (CORE-B): the path has no step in runB, and the finding says
// nothing about the global (confidence no_uncertainty_on_path).
package global

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { _ = len(args) }

func Main() {
	runA(os.Args[2:])
	runB(os.Args[2:])
}
