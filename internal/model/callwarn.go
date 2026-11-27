package model

import (
	"fmt"
	"strings"
)

// CallWarnings are the rows of calls whose `with` does not fit the callback: `with` fills the parameters of the
// callback from the first, so a callback of (ctx, m) given `with: [recv]` gets the receiver in ctx and nothing in
// m, and the flow is silently lost. The number of parameters is known from the row (`params`, which the built-in rows of the standard library of Go
// write; TestCallbackParamsKnown holds them to go/types); a row that does not say it is not checked (write the entries by
// number, `{param: 1, from: recv}`, to say which parameter is meant). The result is advice, not an error.
func (r *Rules) CallWarnings() []string {
	var out []string
	for _, rs := range r.Sets {
		for _, p := range rs.Propagators {
			for i, c := range p.Calls {
				if c.Fn == "" || len(c.With) == 0 {
					continue
				}
				if c.Params == nil {
					continue
				}
				n := *c.Params
				gap := false
				for _, w := range c.With {
					gap = gap || w == ""
				}
				where := fmt.Sprintf("%s: %s calls[%d]", rs.Origin, p.Symbol, i)
				switch {
				case len(c.With) > n:
					out = append(out, fmt.Sprintf("%s: with has %d entries but the callback has %d parameters: the entries after the last parameter are not read", where, len(c.With), n))
				case len(c.With) < n && !gap:
					out = append(out, fmt.Sprintf("%s: with has %d entries but the callback has %d parameters: with fills them from the first (%s), the others get nothing; to fill one other than the first, write {param: N, from: position} (and {param: N, from: none} for one left empty on purpose)", where, len(c.With), n, strings.Join(c.With, ", ")))
				}
			}
		}
	}
	return out
}
