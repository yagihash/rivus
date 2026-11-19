package model

import (
	"fmt"
	"sort"
)

// DefaultMaxImplementations is the default bound of match: implementations.
// It protects against a rule that expands over an interface implemented
// everywhere. In the code measured so far, the interfaces that taint rules
// are written for have at most about twenty implementations; fmt.Stringer
// had 165 (ADR-030).
const DefaultMaxImplementations = 300

// Resolver finds the implementations of an interface method.
type Resolver interface {
	// Implementations returns the symbols of the concrete methods that
	// implement the interface method symbol, e.g. "(net/http.Handler).ServeHTTP".
	Implementations(symbol string) ([]string, error)
}

// MarkedMethod is an interface method found by a marker: the symbol, and the
// positions (receiver excluded) of the parameters that are messages.
type MarkedMethod struct {
	Symbol string
	Params []int
}

// MarkerResolver is a Resolver that can also list the methods of the
// interfaces that have a method named by a marker (a glob).
type MarkerResolver interface {
	MarkedMethods(marker string) []MarkedMethod
}

// CallResolver tells whether the code of the program calls a function.
type CallResolver interface {
	CallsFunction(symbol string) bool
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
	// Missing is true when a marker rule found nothing although the program
	// uses the library it is for: the generated code is probably absent.
	Missing bool `json:"missing,omitempty"`
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
			if s.Marker != "" && s.Kind == KindParam {
				mr, ok := res.(MarkerResolver)
				if !ok {
					continue
				}
				marked := mr.MarkedMethods(s.Marker)
				if cr, ok := res.(CallResolver); ok && len(marked) == 0 && s.MarkerCall != "" && cr.CallsFunction(s.MarkerCall) {
					out = append(out, Expansion{Symbol: s.Marker, Role: "source", Missing: true,
						Note: fmt.Sprintf("the program calls %s, but no interface has a method matching %q: the code generated from the service definitions (protoc) has to exist before the analysis (a service generated without the marker is declared in the rules of the project)", s.MarkerCall, s.Marker)})
				}
				for _, m := range marked {
					m, s := m, s
					expand("source", m.Symbol, s.MaxImplementations, func(c string) {
						cp := s
						cp.Symbol, cp.Match, cp.Marker, cp.Params = c, MatchExact, "", m.Params
						r.ParamFuncs[c] = append(r.ParamFuncs[c], cp)
					})
				}
				continue
			}
			if s.Match == MatchImplementations && (s.Kind == KindCallResult || s.Kind == KindParam) {
				s := s
				expand("source", s.Symbol, s.MaxImplementations, func(c string) {
					cp := s
					cp.Symbol, cp.Match = c, MatchExact
					if s.Kind == KindParam {
						r.ParamFuncs[c] = append(r.ParamFuncs[c], cp)
					} else {
						r.CallSources[c] = append(r.CallSources[c], cp)
					}
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
