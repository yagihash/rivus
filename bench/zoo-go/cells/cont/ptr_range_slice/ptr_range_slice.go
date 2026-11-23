// Package ptr_range_slice is a cell of the zoo.
//
// @cell(F/cont/ptr-range-slice)
// @shape: a pointer taken as the value of a range over a slice, and a field written through it, is read from the slice again
package ptr_range_slice

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-range-slice/vuln", vuln)
	mark.Case("F/cont/ptr-range-slice/fixed", fixed)
	mark.Case("F/cont/ptr-range-slice/decoy", decoy)
}

func vuln() { // @vuln
	xs := []*box{{}}
	for _, p := range xs {
		p.v = mark.Src("cont-ptr-range-slice-vuln") // @source(cont-ptr-range-slice-vuln)
	}
	mark.SinkCmd(xs[0].v) // @sink(cont-ptr-range-slice-vuln)
}

func fixed() { // @fixed
	xs := []*box{{}}
	for _, p := range xs {
		p.v = mark.CleanCmd(mark.Src("cont-ptr-range-slice-fixed"))
	}
	mark.SinkCmd(xs[0].v) // @sink(none)
}

func decoy() { // @decoy
	xs, ys := []*box{{}}, []*box{{}}
	for _, p := range xs {
		p.v = mark.Src("cont-ptr-range-slice-decoy") // @source(cont-ptr-range-slice-decoy)
	}
	mark.SinkCmd(ys[0].v) // @sink(none)
	mark.SinkSQL(xs[0].v) // @sink(cont-ptr-range-slice-decoy)
}
