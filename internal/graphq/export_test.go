package graphq

import "github.com/yagihash/rivus/internal/graphdb"

// ReceiverNames is for the table test of the type a symbol names.
var ReceiverNames = receiverNames

// OnlyImplAfterClose asks the --only-impl filter about an interface edge of a database that was closed (every read fails): it says
// what the filter answered, the warnings it made, and how many answers it remembers.
func OnlyImplAfterClose(e *Engine, types []string) (passed bool, warnings []string, remembered int) {
	e.db.Close()
	c := &Common{Filters: []string{""}}
	f := &implFilter{e: e, types: types, c: c, method: map[int64]bool{}, site: map[int64]bool{}}
	passed = f.pass(&graphdb.Edge{Kind: graphdb.KindInterface, CallSite: 1, To: 1})
	return passed, c.Warnings, len(f.site) + len(f.method)
}
