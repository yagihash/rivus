// Package gwrite: handler (kind: param) writes what it received to a global;
// worker reads the global into a sink. A param source is not written back
// (ADR-048), but a write the function makes itself is a flow: 1 finding.
package gwrite

import "os"

type Req struct{ Path string }

var shared string

func handler(r *Req) { shared = r.Path }
func worker()        { os.Create(shared) }

func Main() {
	handler(&Req{})
	worker()
}
