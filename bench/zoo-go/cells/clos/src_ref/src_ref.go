// Package src_ref is a cell of the zoo.
//
// @cell(F/clos/src-ref)
// @shape: a function that the rules declare a source, named as a value (not written as a closure), is given to a function of the standard library that calls it and puts what it returns in its result: the call is the place of the source
package src_ref

import (
	"os"
	"regexp"
	"slices"
	"strconv"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/src-ref/vuln", vuln)
	mark.Case("F/clos/src-ref/fixed", fixed)
	mark.Case("F/clos/src-ref/decoy", decoy)
}

func vuln() { // @vuln
	re := regexp.MustCompile(".+")
	{
		t := re.ReplaceAllStringFunc("clos-src-ref-vuln-replace", mark.Src) // @source(clos-src-ref-vuln-replace)
		mark.SinkCmd(t)                                                     // @sink(clos-src-ref-vuln-replace)
	}
	{
		t := os.Expand("${clos-src-ref-vuln-expand}", mark.Src) // @source(clos-src-ref-vuln-expand)
		mark.SinkCmd(t)                                         // @sink(clos-src-ref-vuln-expand)
	}
	{
		f := mark.Src
		t := re.ReplaceAllStringFunc("clos-src-ref-vuln-variable", f) // @source(clos-src-ref-vuln-variable)
		mark.SinkCmd(t)                                               // @sink(clos-src-ref-vuln-variable)
	}
	{
		// written as a closure: the source is the call in it, and the call of the library has none of its own
		t := re.ReplaceAllStringFunc("x", func(s string) string {
			return mark.Src("clos-src-ref-vuln-closure") // @source(clos-src-ref-vuln-closure)
		})
		mark.SinkCmd(t) // @sink(clos-src-ref-vuln-closure)
	}
}

func fixed() { // @fixed
	re := regexp.MustCompile(".+")
	t := mark.CleanCmd(re.ReplaceAllStringFunc("clos-src-ref-fixed-replace", mark.Src))
	mark.SinkCmd(t) // @sink(none)
	u := mark.CleanCmd(os.Expand("${clos-src-ref-fixed-expand}", mark.Src))
	mark.SinkCmd(u) // @sink(none)
}

func decoy() { // @decoy
	re := regexp.MustCompile(".+")
	t := mark.Src("clos-src-ref-decoy") // @source(clos-src-ref-decoy)
	// a function that is no source: what it returns is not tainted
	a := re.ReplaceAllStringFunc("constant", plain)
	mark.SinkCmd(a) // @sink(none) @nosink(clos-src-ref-decoy)
	// a source that the call puts nowhere (the result of the call is a position)
	n := slices.IndexFunc([]string{"constant"}, mark.SrcB)
	mark.SinkCmd(strconv.Itoa(n)) // @sink(none) @nosink(clos-src-ref-decoy)
	mark.SinkSQL(t)               // @sink(clos-src-ref-decoy)
}

func plain(s string) string { return s + "!" }
