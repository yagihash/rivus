// Package sanitizer_own_impl is a cell of the zoo.
//
// @cell(F/dyn/sanitizer-own-impl)
// @shape: the project implements an interface whose method the rules call a sanitizer, and the implementation reaches a sink itself
package sanitizer_own_impl

import "zoo/mark"

func init() {
	mark.Case("F/dyn/sanitizer-own-impl/vuln", vuln)
	mark.Case("F/dyn/sanitizer-own-impl/fixed", fixed)
	mark.Case("F/dyn/sanitizer-own-impl/decoy", decoy)
}

// Cleaner is declared a sanitizer by rules.yaml: what Clean returns is clean.
type Cleaner interface {
	Clean(s string) string
}

type shellCleaner struct{}

func (shellCleaner) Clean(s string) string {
	mark.SinkCmd(s) // @sink(dyn-sanitizer-own-impl-vuln)
	return mark.CleanCmd(s)
}

func vuln() { // @vuln
	var c Cleaner = shellCleaner{}
	_ = c.Clean(mark.Src("dyn-sanitizer-own-impl-vuln")) // @source(dyn-sanitizer-own-impl-vuln)
}

type fixedCleaner struct{}

func (fixedCleaner) Clean(s string) string {
	clean := mark.CleanCmd(s)
	mark.SinkCmd(clean) // @sink(none)
	return clean
}

func fixed() { // @fixed
	var c Cleaner = fixedCleaner{}
	_ = c.Clean(mark.Src("dyn-sanitizer-own-impl-fixed"))
}

type quietCleaner struct{}

func (quietCleaner) Clean(s string) string {
	mark.SinkCmd(s) // @sink(none) @nosink(dyn-sanitizer-own-impl-decoy)
	return mark.CleanCmd(s)
}

type loudCleaner struct{}

func (loudCleaner) Clean(s string) string {
	mark.SinkSQL(s) // @sink(dyn-sanitizer-own-impl-decoy)
	return mark.CleanSQL(s)
}

func decoy() { // @decoy
	var quiet Cleaner = quietCleaner{}
	var loud Cleaner = loudCleaner{}
	_ = quiet.Clean("constant")
	_ = loud.Clean(mark.Src("dyn-sanitizer-own-impl-decoy")) // @source(dyn-sanitizer-own-impl-decoy)
}
