// Package str: a string is a value, not an object: nothing is written back.
package str

import "os"

func helper(p string) { os.Create(p) }

func runA(arg string) { helper(arg) }
func runB(arg string) { helper(arg) }

func Main() {
	a := os.Getenv("A")
	runA(a)
	runB(a)
}
