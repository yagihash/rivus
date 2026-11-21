package api

import (
	"curated/sys"
	"curated/web"
)

// The closure that is given away and writes a package variable (C-7, ADR-059). A write of a value that
// the closure was not given by the variable it writes is a flow; the write-back of the whole value that was
// read from the same variable where the closure was made (the cleanup of a mock) adds nothing to the
// variable, and must not lift what is below one of its fields to all of them.

type restInner struct{ m map[string]string }

type restOpts struct {
	a string
	p *restInner
}

var (
	gRest    restOpts
	gRew     restOpts
	gLast    string
	gMix     string
	gPhi     string
	gPending []func()
)

// later keeps the closure, and nothing calls it where it can be seen.
func later(f func()) { gPending = append(gPending, f) }

func capRestores(c *web.Context) {
	capGlobalWrite(c)
	capRestore(c)
	capMixedWrite(c)
	capPhiWrite(c)
	capFieldRewrite(c)
}

// capGlobalWrite: what the closure writes to the variable comes from what it captured: a flow.
func capGlobalWrite(c *web.Context) {
	a := c.Query("gwrite") // @source(cap-gwrite)
	later(func() { gLast = a })
	sys.Run(gLast) // @sink(cap-gwrite)
}

// capRestore: the closure puts back the value that the function read from the variable. What is below the
// pointer field is not what the other field holds.
func capRestore(c *web.Context) {
	gRest.p = &restInner{m: map[string]string{}}
	gRest.p.m["k"] = c.Query("restore") // @source(cap-restore)
	before := gRest
	gRest = restOpts{}
	later(func() { gRest = before })
	sys.Run(gRest.p.m["k"]) // @sink(cap-restore)
	sys.Run(gRest.a)        // @nosink(cap-restore)
}

// capMixedWrite: the value that is put back has something else in it: the write is applied.
func capMixedWrite(c *web.Context) {
	before := gMix
	src := c.Query("mix") // @source(cap-mix)
	later(func() { gMix = before + src })
	sys.Run(gMix) // @sink(cap-mix)
}

// capPhiWrite: the value that is put back is the one that was read, or another one: the write is applied.
func capPhiWrite(c *web.Context) {
	before := gPhi
	if c.Query("pick") == "1" {
		before = c.Query("phi") // @source(cap-phi)
	}
	later(func() { gPhi = before })
	sys.Run(gPhi) // @sink(cap-phi)
}

// capFieldRewrite: a field of the copy is changed before it is put back: the write is applied.
func capFieldRewrite(c *web.Context) {
	before := gRew
	before.a = c.Query("rewrite") // @source(cap-rewrite)
	later(func() { gRew = before })
	sys.Run(gRew.a) // @sink(cap-rewrite)
}
