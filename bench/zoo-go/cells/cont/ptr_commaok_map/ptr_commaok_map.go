// Package ptr_commaok_map is a cell of the zoo.
//
// @cell(F/cont/ptr-commaok-map)
// @shape: a pointer taken out of a map by a comma-ok index, and a field written through it, is read from the map again
package ptr_commaok_map

import "zoo/mark"

type box struct{ v string }

func init() {
	mark.Case("F/cont/ptr-commaok-map/vuln", vuln)
	mark.Case("F/cont/ptr-commaok-map/fixed", fixed)
	mark.Case("F/cont/ptr-commaok-map/decoy", decoy)
}

func vuln() { // @vuln
	m := map[string]*box{"a": {}}
	p, _ := m["a"]
	p.v = mark.Src("cont-ptr-commaok-map-vuln") // @source(cont-ptr-commaok-map-vuln)
	mark.SinkCmd(m["a"].v)                      // @sink(cont-ptr-commaok-map-vuln)
}

func fixed() { // @fixed
	m := map[string]*box{"a": {}}
	p, _ := m["a"]
	p.v = mark.CleanCmd(mark.Src("cont-ptr-commaok-map-fixed"))
	mark.SinkCmd(m["a"].v) // @sink(none)
}

func decoy() { // @decoy
	m := map[string]*box{"a": {}}
	n := map[string]*box{"a": {}}
	p, _ := m["a"]
	p.v = mark.Src("cont-ptr-commaok-map-decoy") // @source(cont-ptr-commaok-map-decoy)
	mark.SinkCmd(n["a"].v)                       // @sink(none)
	mark.SinkSQL(m["a"].v)                       // @sink(cont-ptr-commaok-map-decoy)
}
