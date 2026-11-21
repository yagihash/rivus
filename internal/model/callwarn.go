package model

import (
	"fmt"
	"strings"
)

// knownCallbackParams are the numbers of parameters of the callbacks of functions of the Go standard library
// that a row of calls may name: "symbol/fn" (fn as the row writes it) -> parameters. TestCallbackParamsKnown
// holds the table to the signatures of the standard library (go/types).
var knownCallbackParams = map[string]int{
	"(*sync.Once).Do/arg0":       0,
	"sort.Slice/arg1":            2,
	"sort.SliceStable/arg1":      2,
	"sort.Search/arg1":           1,
	"slices.SortFunc/arg1":       2,
	"slices.SortStableFunc/arg1": 2,
	"slices.IndexFunc/arg1":      1,
	"slices.ContainsFunc/arg1":   1,
	"strings.IndexFunc/arg1":     1,
	"strings.Map/arg0":           1,
	"strings.TrimFunc/arg1":      1,
	"strings.FieldsFunc/arg1":    1,
}

// CallWarnings are the rows of calls whose `with` does not fit the callback: `with` fills the parameters of the
// callback from the first, so a callback of (ctx, m) given `with: [recv]` gets the receiver in ctx and nothing in
// m, and the flow is silently lost. The number of parameters is known from the row (`params`) or, for the standard
// library of Go, from the table above; a row whose callback is not known is not checked (write the entries by
// number, `{param: 1, from: recv}`, to say which parameter is meant). The result is advice, not an error.
func (r *Rules) CallWarnings() []string {
	var out []string
	for _, rs := range r.Sets {
		for _, p := range rs.Propagators {
			for i, c := range p.Calls {
				if c.Fn == "" || len(c.With) == 0 {
					continue
				}
				n, known := c.Params, c.Params > 0
				if !known {
					n, known = knownCallbackParams[p.Symbol+"/"+c.Fn]
				}
				if !known {
					continue
				}
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
