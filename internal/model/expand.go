package model

import (
	"fmt"
	"sort"
)

// DefaultMaxImplementations is the default bound of match: implementations.
// It is provisional and is meant to be settled by measurement (ADR-030).
const DefaultMaxImplementations = 100

// Resolver finds the implementations of an interface method.
type Resolver interface {
	// Implementations returns the symbols of the concrete methods that
	// implement the interface method symbol, e.g. "(net/http.Handler).ServeHTTP".
	Implementations(symbol string) ([]string, error)
}

// Expansion records what happened to one rule with match: implementations.
type Expansion struct {
	Symbol string `json:"symbol"`
	Role   string `json:"role"` // source, sink, sanitizer, propagator
	// Count is the number of implementations found.
	Count int `json:"count"`
	Max   int `json:"max"`
	// Limited is true when the expansion was not done because Count > Max.
	// The rule then matches the interface method only, as with match: exact.
	Limited bool `json:"limited,omitempty"`
	// Note explains a rule that could not be expanded at all.
	Note string `json:"note,omitempty"`
}

// Expand replaces every rule with match: implementations by rules for the
// implementations of the interface method. A rule whose expansion would
// exceed its limit is not expanded at all: a partial expansion would hide
// which part was left out. The outcome of every such rule is returned.
func (r *Rules) Expand(res Resolver, defaultMax int) []Expansion {
	if defaultMax <= 0 {
		defaultMax = DefaultMaxImplementations
	}
	var out []Expansion
	expand := func(role, symbol string, max int, add func(concrete string)) {
		if max <= 0 {
			max = defaultMax
		}
		e := Expansion{Symbol: symbol, Role: role, Max: max}
		syms, err := res.Implementations(symbol)
		switch {
		case err != nil:
			e.Note = err.Error()
		case len(syms) > max:
			e.Count, e.Limited = len(syms), true
		default:
			e.Count = len(syms)
			for _, c := range syms {
				add(c)
			}
		}
		out = append(out, e)
	}
	for _, rs := range r.Sets {
		for _, s := range rs.Sources {
			if s.Match == MatchImplementations && s.Kind == KindCallResult {
				s := s
				expand("source", s.Symbol, s.MaxImplementations, func(c string) {
					cp := s
					cp.Symbol, cp.Match = c, MatchExact
					r.CallSources[c] = append(r.CallSources[c], cp)
				})
			}
		}
		for _, s := range rs.Sinks {
			if s.Match == MatchImplementations {
				s := s
				expand("sink", s.Symbol, s.MaxImplementations, func(c string) {
					cp := s
					cp.Symbol, cp.Match = c, MatchExact
					r.Sinks[c] = append(r.Sinks[c], cp)
				})
			}
		}
		for _, s := range rs.Sanitizers {
			if s.Match == MatchImplementations {
				s := s
				expand("sanitizer", s.Symbol, s.MaxImplementations, func(c string) {
					cp := s
					cp.Symbol, cp.Match = c, MatchExact
					r.Sanitizers[c] = append(r.Sanitizers[c], cp)
				})
			}
		}
		for _, p := range rs.Propagators {
			if p.Match == MatchImplementations {
				p := p
				expand("propagator", p.Symbol, p.MaxImplementations, func(c string) {
					cp := p
					cp.Symbol, cp.Match = c, MatchExact
					r.Propagators[c] = append(r.Propagators[c], cp)
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Symbol != out[j].Symbol {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].Role < out[j].Role
	})
	r.Expansions = out
	return out
}

// String summarises an expansion for messages.
func (e Expansion) String() string {
	switch {
	case e.Note != "":
		return fmt.Sprintf("%s %s: not expanded (%s)", e.Role, e.Symbol, e.Note)
	case e.Limited:
		return fmt.Sprintf("%s %s: %d implementations exceed the limit %d, not expanded", e.Role, e.Symbol, e.Count, e.Max)
	}
	return fmt.Sprintf("%s %s: expanded to %d implementations", e.Role, e.Symbol, e.Count)
}
