// Package to_lib_map is a cell of the zoo.
//
// @cell(F/clos/to-lib-map)
// @shape: a function that is given to a function of the standard library that tests the entries of a map (maps.DeleteFunc) is called with the key and the value: what the map holds reaches the sink in the function. A function that uses a value that is not an entry is not a finding
package to_lib_map

import (
	"maps"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-lib-map/vuln", vuln)
	mark.Case("F/clos/to-lib-map/fixed", fixed)
	mark.Case("F/clos/to-lib-map/decoy", decoy)
}

func vuln() { // @vuln
	{
		t := mark.Src("clos-to-lib-map-vuln-key") // @source(clos-to-lib-map-vuln-key)
		m := map[string]string{t: "v"}
		maps.DeleteFunc(m, func(k, v string) bool {
			mark.SinkCmd(k) // @sink(clos-to-lib-map-vuln-key)
			return false
		})
	}
	{
		t := mark.Src("clos-to-lib-map-vuln-value") // @source(clos-to-lib-map-vuln-value)
		m := map[string]string{"k": t}
		maps.DeleteFunc(m, func(k, v string) bool {
			mark.SinkCmd(v) // @sink(clos-to-lib-map-vuln-value)
			return false
		})
	}
}

func fixed() { // @fixed
	m := map[string]string{"k": mark.CleanCmd(mark.Src("clos-to-lib-map-fixed"))}
	maps.DeleteFunc(m, func(k, v string) bool {
		mark.SinkCmd(v) // @sink(none)
		return false
	})
}

func decoy() { // @decoy
	t := mark.Src("clos-to-lib-map-decoy") // @source(clos-to-lib-map-decoy)
	m := map[string]string{"k": "constant"}
	maps.DeleteFunc(m, func(k, v string) bool {
		mark.SinkCmd(v) // @sink(none) @nosink(clos-to-lib-map-decoy)
		return false
	})
	mark.SinkSQL(t) // @sink(clos-to-lib-map-decoy)
}
