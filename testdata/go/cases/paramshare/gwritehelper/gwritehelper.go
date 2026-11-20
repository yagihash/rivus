// Package gwritehelper: the same as gwrite, with the write to the global in a
// helper (the summary of the callee writes it): 1 finding.
package gwritehelper

import "os"

type Req struct{ Path string }

var shared string

func save(p string) { shared = p }

func handler(r *Req) { save(r.Path) }
func worker()        { os.Create(shared) }

func Main() {
	handler(&Req{})
	worker()
}
