package api

import (
	"curated/sys"
	"curated/web"
)

// One object that two functions reach through two different pointers (the limit of the alias
// analysis: rivus has no full pointer analysis, an accepted approximation, ADR-048). `store`
// writes a source into `h.S.Last`, `use` reads `s.Last` from the same `state` through its own
// pointer `s`. Where `use` gets the holder itself it is found (testdata/go/cases/paramshare/ctxstate);
// where the caller keeps its own pointer to the state and gives that to `use`, the flow is lost
// without a trace (testdata/go/cases/paramshare/ctxptr is the same shape with a param source).

type state struct{ Last string }
type holder struct{ S *state }

func store(h *holder, c *web.Context) {
	h.S.Last = c.Query("alias") // @source(alias-ptr) @miss(silent)
}

func use(s *state) {
	sys.Run(s.Last) // @sink(alias-ptr)
}

func aliasPtr(c *web.Context) {
	s := &state{}
	h := &holder{S: s}
	store(h, c)
	use(s)
}
