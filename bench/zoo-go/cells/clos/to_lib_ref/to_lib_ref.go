// Package to_lib_ref is a cell of the zoo.
//
// @cell(F/clos/to-lib-ref)
// @shape: a function that the rules declare a sink, named as a value (not written as a closure), is given to a function of the standard library that calls it with the elements of a slice: the call is the place of the sink
package to_lib_ref

import (
	"slices"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-lib-ref/vuln", vuln)
	mark.Case("F/clos/to-lib-ref/fixed", fixed)
	mark.Case("F/clos/to-lib-ref/decoy", decoy)
}

func vuln() { // @vuln
	{
		t := mark.Src("clos-to-lib-ref-vuln-indexfunc") // @source(clos-to-lib-ref-vuln-indexfunc)
		xs := []string{t}
		slices.IndexFunc(xs, mark.SinkCmdB) // @sink(clos-to-lib-ref-vuln-indexfunc)
	}
	{
		t := mark.Src("clos-to-lib-ref-vuln-containsfunc") // @source(clos-to-lib-ref-vuln-containsfunc)
		xs := []string{t}
		slices.ContainsFunc(xs, mark.SinkCmdB) // @sink(clos-to-lib-ref-vuln-containsfunc)
	}
	{
		t := mark.Src("clos-to-lib-ref-vuln-variable") // @source(clos-to-lib-ref-vuln-variable)
		xs := []string{t}
		f := mark.SinkCmdB
		slices.IndexFunc(xs, f) // @sink(clos-to-lib-ref-vuln-variable)
	}
	{
		// written as a closure: the sink is the call in it, and the call of the library has none of its own
		t := mark.Src("clos-to-lib-ref-vuln-closure") // @source(clos-to-lib-ref-vuln-closure)
		xs := []string{t}
		slices.IndexFunc(xs, func(e string) bool {
			return mark.SinkCmdB(e) // @sink(clos-to-lib-ref-vuln-closure)
		})
	}
}

func fixed() { // @fixed
	xs := []string{mark.CleanCmd(mark.Src("clos-to-lib-ref-fixed"))}
	slices.IndexFunc(xs, mark.SinkCmdB)    // @sink(none)
	slices.ContainsFunc(xs, mark.SinkCmdB) // @sink(none)
	f := mark.SinkCmdB
	slices.IndexFunc(xs, f) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("clos-to-lib-ref-decoy") // @source(clos-to-lib-ref-decoy)
	// the elements are constants: the value of the source is not what the library hands to the function
	xs := []string{"constant"}
	slices.IndexFunc(xs, mark.SinkCmdB)    // @sink(none) @nosink(clos-to-lib-ref-decoy)
	slices.ContainsFunc(xs, mark.SinkCmdB) // @sink(none) @nosink(clos-to-lib-ref-decoy)
	f := mark.SinkCmdB
	slices.IndexFunc(xs, f) // @sink(none) @nosink(clos-to-lib-ref-decoy)
	// a function that is no sink, given the tainted elements
	ys := []string{t}
	slices.IndexFunc(ys, isEmpty)
	mark.SinkSQL(t) // @sink(clos-to-lib-ref-decoy)
}

func isEmpty(s string) bool { return s == "" }
