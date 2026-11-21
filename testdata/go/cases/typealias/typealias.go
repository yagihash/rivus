// Package typealias: a rule names a type, and the code names it by another of
// its names. The rule for A is written with the alias, the rule for B with the
// type; both match both spellings (ADR-083).
package typealias

import (
	"example.com/cases/cases/typealias/api"
	"example.com/cases/cases/typealias/real"
	"example.com/cases/lib"
)

func ARealSpelling(r *real.A) { // @source(a1)
	lib.Sink(r.P) // @sink(a1)
}

func AAliasSpelling(r *api.AliasA) { // @source(a2)
	lib.Sink(r.P) // @sink(a2)
}

func BRealSpelling(r *real.B) { // @source(b1)
	lib.Sink(r.P) // @sink(b1)
}

func BAliasSpelling(r *api.AliasB) { // @source(b2)
	lib.Sink(r.P) // @sink(b2)
}

// The rules for the fields: C by the alias, D by the type.
func CRealSpelling(r *real.C) {
	v := r.F    // @source(c1)
	lib.Sink(v) // @sink(c1)
}

func CAliasSpelling(r *api.AliasC) {
	v := r.F    // @source(c2)
	lib.Sink(v) // @sink(c2)
}

func DRealSpelling(r *real.D) {
	v := r.F    // @source(d1)
	lib.Sink(v) // @sink(d1)
}

func DAliasSpelling(r *api.AliasD) {
	v := r.F    // @source(d2)
	lib.Sink(v) // @sink(d2)
}
