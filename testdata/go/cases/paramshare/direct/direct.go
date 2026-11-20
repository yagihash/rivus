// Package direct: the parameter itself reaches a sink inside the function that
// declares it as a source. Not writing the taint back to the caller (ADR-048)
// must not lose this flow.
package direct

import "os"

type Req struct{ Path string }

func runB(r *Req) { os.Create(r.Path) }

func Main() {
	r := &Req{Path: "x"}
	runB(r)
}
