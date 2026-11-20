// Package distinct: the two functions get distinct objects. Does not happen:
// each source reaches the sink only through its own function.
package distinct

import "os"

func helper(p string) { os.Create(p) }

func runA(args []string) { helper(args[0]) }
func runB(args []string) { helper(args[0]) }

func Main() {
	a := []string{os.Getenv("A")}
	b := []string{os.Getenv("B")}
	runA(a)
	runB(b)
}
