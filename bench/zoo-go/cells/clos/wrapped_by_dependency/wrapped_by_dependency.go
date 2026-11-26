// Package wrapped_by_dependency is a cell of the zoo.
//
// @cell(F/clos/wrapped-by-dependency)
// @shape: a function of the project is given to a dependency (zoodep.WrapFn), which returns a closure that calls it; the project calls what it got back
package wrapped_by_dependency

import (
	"zoo/mark"
	"zoodep"
)

func init() {
	mark.Case("F/clos/wrapped-by-dependency/vuln", vuln)
	mark.Case("F/clos/wrapped-by-dependency/fixed", fixed)
	mark.Case("F/clos/wrapped-by-dependency/decoy", decoy)
}

func vuln() { // @vuln
	a := mark.Src("clos-wrapped-by-dependency-vuln") // @source(clos-wrapped-by-dependency-vuln)
	g := zoodep.WrapFn(func(s string) string { return s })
	mark.SinkCmd(g(a)) // @sink(clos-wrapped-by-dependency-vuln)
}

func fixed() { // @fixed
	a := mark.Src("clos-wrapped-by-dependency-fixed")
	g := zoodep.WrapFn(func(s string) string { return mark.CleanCmd(s) })
	mark.SinkCmd(g(a)) // @sink(none) @fp @ref(#78)
}

func decoy() { // @decoy
	a := mark.Src("clos-wrapped-by-dependency-decoy") // @source(clos-wrapped-by-dependency-decoy)
	quiet := zoodep.WrapFn(func(s string) string { return "constant" })
	loud := zoodep.WrapFn(func(s string) string { return s })
	mark.SinkCmd(quiet(a)) // @sink(none) @fp @nosink(clos-wrapped-by-dependency-decoy) @fp @ref(#78)
	mark.SinkSQL(loud(a))  // @sink(clos-wrapped-by-dependency-decoy)
}
