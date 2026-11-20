// Package ctxstate: handler stores what it received in a state shared through
// the context (c.App.Last); reader gets the same App and uses it in a sink:
// 1 finding. The write is made by the function, it is not the source being
// written back (ADR-048).
package ctxstate

import "os"

type App struct{ Last string }
type Ctx struct {
	App   *App
	Query string
}

func handler(c *Ctx) { c.App.Last = c.Query }
func reader(c *Ctx)  { os.Create(c.App.Last) }

func Main() {
	c := &Ctx{App: &App{}}
	handler(c)
	reader(c)
}
