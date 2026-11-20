// Package ctxptr: like ctxstate, but reader gets the App itself (reader(app)),
// not the Ctx. handler writes the source into c.App.Last and reader reads
// a.Last into a sink: the same object reaches the two functions through two
// pointers (c.App and a). KNOWN MISS: no finding. A param source is not written
// back to the caller (ADR-048) and the object is not followed from the field of
// one parameter to another parameter (no full pointer analysis: an accepted
// approximation).
package ctxptr

import "os"

type App struct{ Last string }
type Ctx struct {
	App   *App
	Query string
}

func handler(c *Ctx) { c.App.Last = c.Query }
func reader(a *App)  { os.Create(a.Last) }

func Main() {
	app := &App{}
	c := &Ctx{App: app}
	handler(c)
	reader(app)
}
