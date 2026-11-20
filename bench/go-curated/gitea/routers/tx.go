package routers

import (
	stdctx "context"

	"curated/gitea/context"
	"curated/gitea/db"
	"curated/gitea/storage"
	"curated/sys"
)

// DeleteInTx is the other shape of a Gitea service: the work is a closure given to
// the transaction helper (db.WithTx), and the closure uses what the function
// captured. Every service of Gitea is written in this way. The helper calls
// the closure, and a captured value does not get into it from the caller (the
// closure is a function value that the helper takes by a parameter and calls).
func DeleteInTx(ctx *context.Context) { // @source(tx_capture) @miss(dynamic_dispatch)
	uuid := ctx.Req.FormValue("uuid")
	_ = db.WithTx(ctx.Req.Context(), func(c stdctx.Context) error {
		return storage.Attachments.Delete(uuid) // @sink(tx_capture)
	})
}

// RunInTx is the same with a sink that is not behind an interface: the engine
// leaves no trace of the lost flow.
func RunInTx(ctx *context.Context) { // @source(tx_capture_plain) @miss(depth_limit)
	name := ctx.Req.FormValue("name")
	_ = db.WithTx(ctx.Req.Context(), func(c stdctx.Context) error {
		return sys.Run(name) // @sink(tx_capture_plain)
	})
}
