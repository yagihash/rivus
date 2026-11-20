// Package ptype: the param_type version. Two handlers get the same request
// type; HandleA reaches the sink, HandleC only holds the request. One finding
// (HandleA), with no other_sources (before ADR-048 HandleC was listed there).
package ptype

import "os"

type Req struct{ Path string }

func save(p string) { os.Create(p) }

func HandleA(r *Req) { save(r.Path) }
func HandleC(r *Req) { _ = r }

func Main() {
	r := &Req{Path: "x"}
	HandleA(r)
	HandleC(r)
}
