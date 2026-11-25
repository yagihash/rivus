// Package to_sort is a cell of the zoo.
//
// @cell(F/clos/to-sort)
// @shape: a comparison function that is given to a sort of the standard library (sort.Slice, slices.SortFunc) is called with the elements: what the slice holds reaches the sink in the function
package to_sort

import (
	"slices"
	"sort"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-sort/vuln", vuln)
	mark.Case("F/clos/to-sort/vuln2", vuln2)
	mark.Case("F/clos/to-sort/fixed", fixed)
	mark.Case("F/clos/to-sort/decoy", decoy)
}

func vuln() { // @vuln
	t := mark.Src("clos-to-sort-vuln") // @source(clos-to-sort-vuln)
	s := []string{t, t}
	sort.Slice(s, func(i, j int) bool {
		mark.SinkCmd(s[i]) // @sink(clos-to-sort-vuln)
		return s[i] < s[j]
	})
}

// vuln2: the elements are the arguments of the function.
func vuln2() { // @vuln
	t := mark.Src("clos-to-sort-vuln2") // @source(clos-to-sort-vuln2)
	s := []string{t, t}
	slices.SortFunc(s, func(a, b string) int {
		mark.SinkCmd(a) // @sink(clos-to-sort-vuln2)
		return 0
	})
}

func fixed() { // @fixed
	t := mark.CleanCmd(mark.Src("clos-to-sort-fixed"))
	s := []string{t, t}
	sort.Slice(s, func(i, j int) bool {
		mark.SinkCmd(s[i]) // @sink(none)
		return s[i] < s[j]
	})
	slices.SortFunc(s, func(a, b string) int {
		mark.SinkCmd(a) // @sink(none)
		return 0
	})
}

func decoy() { // @decoy
	t := mark.Src("clos-to-sort-decoy") // @source(clos-to-sort-decoy)
	s := []string{"x", "y"}
	sort.Slice(s, func(i, j int) bool {
		mark.SinkCmd(s[i]) // @sink(none) @nosink(clos-to-sort-decoy)
		return s[i] < s[j]
	})
	slices.SortFunc(s, func(a, b string) int {
		mark.SinkCmd(a) // @sink(none) @nosink(clos-to-sort-decoy)
		return 0
	})
	mark.SinkSQL(t) // @sink(clos-to-sort-decoy)
}
