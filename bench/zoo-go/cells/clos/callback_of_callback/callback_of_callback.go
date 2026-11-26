// Package callback_of_callback is a cell of the zoo.
//
// @cell(F/clos/callback-of-callback)
// @shape: a closure takes a function as its parameter and calls it (g(a)): the callback of a callback
package callback_of_callback

import "zoo/mark"

func init() {
	mark.Case("F/clos/callback-of-callback/vuln", vuln)
	mark.Case("F/clos/callback-of-callback/fixed", fixed)
	mark.Case("F/clos/callback-of-callback/decoy", decoy)
}

func viaInner(s string, outer func(func(string) string) string) string {
	return outer(func(x string) string { return x + s })
}

func vuln() { // @vuln
	a := mark.Src("clos-callback-of-callback-vuln")                                     // @source(clos-callback-of-callback-vuln) @miss(assumed_call,depth_limit) @ref(#81)
	mark.SinkCmd(viaInner("const", func(g func(string) string) string { return g(a) })) // @sink(clos-callback-of-callback-vuln)
}

func fixed() { // @fixed
	a := mark.CleanCmd(mark.Src("clos-callback-of-callback-fixed"))
	mark.SinkCmd(viaInner("const", func(g func(string) string) string { return g(a) })) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("clos-callback-of-callback-decoy")                                             // @source(clos-callback-of-callback-decoy) @miss(assumed_call,depth_limit) @ref(#81)
	mark.SinkCmd(viaInner("const", func(g func(string) string) string { return g("constant") })) // @sink(none) @nosink(clos-callback-of-callback-decoy)
	mark.SinkSQL(viaInner("const", func(g func(string) string) string { return g(a) }))          // @sink(clos-callback-of-callback-decoy)
}
