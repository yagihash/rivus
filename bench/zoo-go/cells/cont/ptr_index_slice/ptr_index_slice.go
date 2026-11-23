// Package ptr_index_slice is a cell of the zoo.
//
// @cell(F/cont/ptr-index-slice)
// @shape: a pointer read from a slice element, and a field written through it, is read from the slice again
package ptr_index_slice

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-index-slice/vuln", vuln)
	mark.Case("F/cont/ptr-index-slice/fixed", fixed)
	mark.Case("F/cont/ptr-index-slice/decoy", decoy)
}

func vuln() { // @vuln
	xs := []*box{{}}
	p := xs[0]
	p.v = mark.Src("cont-ptr-index-slice-vuln") // @source(cont-ptr-index-slice-vuln)
	mark.SinkCmd(xs[0].v)                       // @sink(cont-ptr-index-slice-vuln)
}

func fixed() { // @fixed
	xs := []*box{{}}
	p := xs[0]
	p.v = mark.CleanCmd(mark.Src("cont-ptr-index-slice-fixed"))
	mark.SinkCmd(xs[0].v) // @sink(none)
}

func decoy() { // @decoy
	xs, ys := []*box{{}}, []*box{{}}
	p := xs[0]
	p.v = mark.Src("cont-ptr-index-slice-decoy") // @source(cont-ptr-index-slice-decoy)
	mark.SinkCmd(ys[0].v)                        // @sink(none)
	mark.SinkSQL(xs[0].v)                        // @sink(cont-ptr-index-slice-decoy)
}
