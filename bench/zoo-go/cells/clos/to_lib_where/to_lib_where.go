// Package to_lib_where is a cell of the zoo.
//
// @cell(F/clos/to-lib-where)
// @shape: a function that is given to a function of the standard library that tests the elements of a slice (slices.IndexFunc, slices.ContainsFunc, slices.DeleteFunc) is called with an element: what the slice holds reaches the sink in the function. A function that uses a value that is not an element is not a finding
package to_lib_where

import (
	"slices"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-lib-where/vuln", vuln)
	mark.Case("F/clos/to-lib-where/fixed", fixed)
	mark.Case("F/clos/to-lib-where/decoy", decoy)
}

func vuln() { // @vuln
	{
		t := mark.Src("clos-to-lib-where-vuln-indexfunc") // @source(clos-to-lib-where-vuln-indexfunc)
		l := []string{t}
		_ = slices.IndexFunc(l, func(e string) bool {
			mark.SinkCmd(e) // @sink(clos-to-lib-where-vuln-indexfunc)
			return true
		})
	}
	{
		t := mark.Src("clos-to-lib-where-vuln-containsfunc") // @source(clos-to-lib-where-vuln-containsfunc)
		l := []string{t}
		_ = slices.ContainsFunc(l, func(e string) bool {
			mark.SinkCmd(e) // @sink(clos-to-lib-where-vuln-containsfunc)
			return true
		})
	}
	{
		t := mark.Src("clos-to-lib-where-vuln-deletefunc") // @source(clos-to-lib-where-vuln-deletefunc)
		l := []string{t}
		_ = slices.DeleteFunc(l, func(e string) bool {
			mark.SinkCmd(e) // @sink(clos-to-lib-where-vuln-deletefunc)
			return false
		})
	}
}

func fixed() { // @fixed
	l := []string{mark.CleanCmd(mark.Src("clos-to-lib-where-fixed"))}
	_ = slices.IndexFunc(l, func(e string) bool {
		mark.SinkCmd(e) // @sink(none)
		return true
	})
	_ = slices.ContainsFunc(l, func(e string) bool {
		mark.SinkCmd(e) // @sink(none)
		return true
	})
	_ = slices.DeleteFunc(l, func(e string) bool {
		mark.SinkCmd(e) // @sink(none)
		return false
	})
}

func decoy() { // @decoy
	t := mark.Src("clos-to-lib-where-decoy") // @source(clos-to-lib-where-decoy)
	l := []string{"constant"}
	_ = slices.IndexFunc(l, func(e string) bool {
		mark.SinkCmd(e) // @sink(none) @nosink(clos-to-lib-where-decoy)
		return true
	})
	_ = slices.ContainsFunc(l, func(e string) bool {
		mark.SinkCmd(e) // @sink(none) @nosink(clos-to-lib-where-decoy)
		return true
	})
	_ = slices.DeleteFunc(l, func(e string) bool {
		mark.SinkCmd(e) // @sink(none) @nosink(clos-to-lib-where-decoy)
		return false
	})
	mark.SinkSQL(t) // @sink(clos-to-lib-where-decoy)
}
