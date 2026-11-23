// Package ptr_value_copy is a cell of the zoo.
//
// @cell(F/cont/ptr-value-copy)
// @shape: a value (not a pointer) taken by a comma-ok assertion or a range is a copy: a field written in the copy is not written back to the original
package ptr_value_copy

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-value-copy/vuln", vuln)
	mark.Case("F/cont/ptr-value-copy/fixed", fixed)
	mark.Case("F/cont/ptr-value-copy/decoy", decoy)
}

// the copy itself is tainted and read: that is a finding
func vuln() { // @vuln
	var x any = box{}
	c, _ := x.(box)
	c.v = mark.Src("cont-ptr-value-copy-vuln") // @source(cont-ptr-value-copy-vuln)
	mark.SinkCmd(c.v)                          // @sink(cont-ptr-value-copy-vuln)
}

func fixed() { // @fixed
	var x any = box{}
	c, _ := x.(box)
	c.v = mark.CleanCmd(mark.Src("cont-ptr-value-copy-fixed"))
	mark.SinkCmd(c.v) // @sink(none)
}

// the write goes to the copy only: the original (read by assertion or by range) is clean
func decoy() { // @decoy
	var x any = box{}
	m := map[string]box{"k": {}}
	c, _ := x.(box)
	c.v = mark.Src("cont-ptr-value-copy-decoy") // @source(cont-ptr-value-copy-decoy)
	for _, e := range m {
		e.v = mark.Src("cont-ptr-value-copy-decoy-range") // @source(cont-ptr-value-copy-decoy-range)
		mark.SinkCmd(e.v)                                 // @sink(cont-ptr-value-copy-decoy-range)
	}
	mark.SinkCmd(x.(box).v) // @sink(none)
	for _, e := range m {
		mark.SinkSQL(e.v) // @sink(none)
	}
	mark.SinkHTML(c.v) // @sink(cont-ptr-value-copy-decoy)
}
