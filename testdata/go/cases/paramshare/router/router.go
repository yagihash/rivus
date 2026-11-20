// Package router: the shape of a router and its handlers. The router calls
// every handler with the same request; both handlers are entry points
// (kind: param), only handlerA reaches the sink. handlerB only holds the
// request: it is no finding (ADR-048; before, the taint of handlerB's
// parameter was written back to the request and handlerA read it, CORE-A).
package router

import "os"

type Req struct{ Path string }

func helper(p string) { os.Create(p) }

func handlerA(r *Req) { helper(r.Path) }
func handlerB(r *Req) { _ = r }

func route(r *Req, fa, fb func(*Req)) {
	fa(r)
	fb(r)
}

func Main() {
	r := &Req{Path: "x"}
	route(r, handlerA, handlerB)
}
