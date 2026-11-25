// Package to_nomodel_ref is a cell of the zoo.
//
// @cell(F/clos/to-nomodel-ref)
// @shape: a function that the rules declare a sink, named as a value (not written as a closure), is given to a function of the standard library that has no body and no row of calls (ring.Do): the call is the place of the sink
package to_nomodel_ref

import (
	"container/ring"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-nomodel-ref/vuln", vuln)
	mark.Case("F/clos/to-nomodel-ref/fixed", fixed)
	mark.Case("F/clos/to-nomodel-ref/decoy", decoy)
}

func vuln() { // @vuln
	{
		t := mark.Src("clos-to-nomodel-ref-vuln-direct") // @source(clos-to-nomodel-ref-vuln-direct)
		r := ring.New(1)
		r.Value = t
		r.Do(mark.SinkAny) // @sink(clos-to-nomodel-ref-vuln-direct)
	}
	{
		t := mark.Src("clos-to-nomodel-ref-vuln-variable") // @source(clos-to-nomodel-ref-vuln-variable)
		r := ring.New(1)
		r.Value = t
		f := mark.SinkAny
		r.Do(f) // @sink(clos-to-nomodel-ref-vuln-variable)
	}
	{
		// written as a closure: the sink is the call in it, and the call of the library has none of its own
		t := mark.Src("clos-to-nomodel-ref-vuln-closure") // @source(clos-to-nomodel-ref-vuln-closure)
		r := ring.New(1)
		r.Value = t
		r.Do(func(x any) {
			mark.SinkAny(x) // @sink(clos-to-nomodel-ref-vuln-closure)
		})
	}
}

func fixed() { // @fixed
	v := mark.CleanHTML(mark.CleanPath(mark.CleanSQL(mark.CleanCmd(mark.Src("clos-to-nomodel-ref-fixed")))))
	r := ring.New(1)
	r.Value = v
	r.Do(mark.SinkAny) // @sink(none)
	f := mark.SinkAny
	r.Do(f) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("clos-to-nomodel-ref-decoy") // @source(clos-to-nomodel-ref-decoy)
	// the value of the ring is a constant: the value of the source is not what the library hands to the function
	r := ring.New(1)
	r.Value = "constant"
	r.Do(mark.SinkAny) // @sink(none) @nosink(clos-to-nomodel-ref-decoy)
	f := mark.SinkAny
	r.Do(f) // @sink(none) @nosink(clos-to-nomodel-ref-decoy)
	// a function that is no sink, given the tainted value
	q := ring.New(1)
	q.Value = t
	q.Do(ignore)
	mark.SinkSQL(t) // @sink(clos-to-nomodel-ref-decoy)
}

func ignore(any) {}
