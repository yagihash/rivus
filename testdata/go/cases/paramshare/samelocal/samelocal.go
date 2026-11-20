// Package samelocal: both get the same local, and both reach the sink: 2 real
// flows, no duplicate.
package samelocal

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { helper(args[0]) }

func Main() {
	a := []string{os.Getenv("A")}
	runA(a)
	runB(a)
}
