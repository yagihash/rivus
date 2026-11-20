// Package lost: what ADR-048 gives up. runB is a source (kind: param) and only
// holds the request; the caller then uses the same request in a sink.
//
//	runB(r)
//	os.Create(r.Path)
//
// A param source is a statement about the value the function received, and is
// not written back to the caller. The flow "the request that runB received
// reaches the sink in Main" is lost: KNOWN MISS (silent). The taint of such
// a source stays in the function; if the request should be tainted in Main, the
// entry point is the caller (Main), or the source is declared there.
package lost

import "os"

type Req struct{ Path string }

func runB(r *Req) { _ = r }

func Main() {
	r := &Req{Path: "x"}
	runB(r)
	os.Create(r.Path)
}
