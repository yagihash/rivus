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

// An alias inside a type (ADR-083): in a map, in a type argument, under a slice of pointers. The rule
// for E names the type and the code the alias, the rule for F the other way round (the same for the others).
func ERealInMap(m map[string]*real.E) { // @source(e1)
	for _, v := range m {
		lib.Sink(v.P) // @sink(e1)
	}
}

func EAliasInMap(m map[string]*api.AliasE) { // @source(e2)
	for _, v := range m {
		lib.Sink(v.P) // @sink(e2)
	}
}

func FRealInMap(m map[string]*real.F) { // @source(f1)
	for _, v := range m {
		lib.Sink(v.P) // @sink(f1)
	}
}

func FAliasInMap(m map[string]*api.AliasF) { // @source(f2)
	for _, v := range m {
		lib.Sink(v.P) // @sink(f2)
	}
}

func GRealInBox(b *real.Box[real.G]) { // @source(g1)
	lib.Sink(b.V.P) // @sink(g1)
}

func GAliasInBox(b *real.Box[api.AliasG]) { // @source(g2)
	lib.Sink(b.V.P) // @sink(g2)
}

func HRealInSlice(hs []*real.H) { // @source(h1)
	for _, h := range hs {
		lib.Sink(h.P) // @sink(h1)
	}
}

func HAliasInSlice(hs []*api.AliasH) { // @source(h2)
	for _, h := range hs {
		lib.Sink(h.P) // @sink(h2)
	}
}

// F8: a type defined in a function can have the name of an alias of the package. The rule for real.S is
// for the alias target, not for the other type of that name.
func SRealSpelling(r *real.S) { // @source(s1)
	lib.Sink(r.P) // @sink(s1)
}

type Shadow = real.S

func SShadowAlias(r *Shadow) { // @source(s2)
	lib.Sink(r.P) // @sink(s2)
}

// The decoy: the Shadow of the function is not the alias, so the rule for real.S is not for its parameter.
func SShadowDecoy() {
	type Shadow struct{ P string }
	f := func(r *Shadow) {
		lib.Sink(r.P)
	}
	f(&Shadow{})
}
