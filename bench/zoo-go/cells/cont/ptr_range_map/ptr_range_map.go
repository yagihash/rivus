// Package ptr_range_map is a cell of the zoo.
//
// @cell(F/cont/ptr-range-map)
// @shape: a pointer taken as the value of a range over a map, and a field written through it, is read from the map again
package ptr_range_map

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-range-map/vuln", vuln)
	mark.Case("F/cont/ptr-range-map/fixed", fixed)
	mark.Case("F/cont/ptr-range-map/decoy", decoy)
}

func vuln() { // @vuln
	m := map[string]*box{"a": {}}
	for _, p := range m {
		p.v = mark.Src("cont-ptr-range-map-vuln") // @source(cont-ptr-range-map-vuln)
	}
	mark.SinkCmd(m["a"].v) // @sink(cont-ptr-range-map-vuln)
}

func fixed() { // @fixed
	m := map[string]*box{"a": {}}
	for _, p := range m {
		p.v = mark.CleanCmd(mark.Src("cont-ptr-range-map-fixed"))
	}
	mark.SinkCmd(m["a"].v) // @sink(none)
}

func decoy() { // @decoy
	m := map[string]*box{"a": {}}
	n := map[string]*box{"a": {}}
	for _, p := range m {
		p.v = mark.Src("cont-ptr-range-map-decoy") // @source(cont-ptr-range-map-decoy)
	}
	mark.SinkCmd(n["a"].v) // @sink(none)
	mark.SinkSQL(m["a"].v) // @sink(cont-ptr-range-map-decoy)
}
