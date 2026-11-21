// Package func_map is a cell of the zoo.
//
// @cell(F/dyn/func-map)
// @shape: a function value looked up in a map by a constant key is called; the map has another entry with another sink
package func_map

import "zoo/mark"

func init() {
	mark.Case("F/dyn/func-map/vuln", vuln)
	mark.Case("F/dyn/func-map/fixed", fixed)
	mark.Case("F/dyn/func-map/decoy", decoy)
}

type fnV func(string)
type fnF func(string)
type fnD func(string)

func vuln() { // @vuln
	v := mark.Src("dyn-func-map-vuln") // @source(dyn-func-map-vuln)
	m := map[string]fnV{"cmd": func(x string) {
		mark.SinkCmd(x) // @sink(dyn-func-map-vuln)
	}}
	m["cmd"](v)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("dyn-func-map-fixed"))
	m := map[string]fnF{"cmd": func(x string) {
		mark.SinkCmd(x) // @sink(none)
	}}
	m["cmd"](v)
}

func decoy() { // @decoy
	v := mark.Src("dyn-func-map-decoy") // @source(dyn-func-map-decoy)
	m := map[string]fnD{
		"cmd": func(x string) {
			mark.SinkCmd(x) // @sink(none) @fp @nosink(dyn-func-map-decoy) @fp @ref(XTALK)
		},
		"sql": func(x string) {
			mark.SinkSQL(x) // @sink(dyn-func-map-decoy)
		},
	}
	m["cmd"]("constant")
	m["sql"](v)
}
