// Package model loads the declarative YAML rules (sources, sinks,
// sanitizers and propagators) that describe code rivus cannot or should not
// analyse by itself. Built-in rules use exactly the same format as user rules.
//
// Symbols use the go/types FullName format:
//
//	os/exec.Command                      a function
//	(*net/http.Request).FormValue        a method with a pointer receiver
//	(net/url.Values).Get                 a method with a value receiver
//	(io.Writer).Write                    an interface method
package model

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Source kinds.
const (
	KindCallResult = "call_result" // the result of calling Symbol
	KindParamType  = "param_type"  // every parameter whose type is Type
	KindField      = "field"       // reads of the struct field Field
)

// Match modes of a symbol.
const (
	MatchExact           = "exact"
	MatchImplementations = "implementations" // an interface method: expands to its implementations
)

// RuleSet is one parsed rule file.
type RuleSet struct {
	Name        string       `yaml:"ruleset"`
	Sources     []Source     `yaml:"sources"`
	Sinks       []Sink       `yaml:"sinks"`
	Sanitizers  []Sanitizer  `yaml:"sanitizers"`
	Propagators []Propagator `yaml:"propagators"`

	// Origin is where the rule set came from (file name or "builtin:...").
	Origin string `yaml:"-"`
}

// Source introduces taint.
type Source struct {
	Kind   string   `yaml:"kind"`
	Symbol string   `yaml:"symbol"` // call_result
	Type   string   `yaml:"type"`   // param_type, e.g. "*net/http.Request"
	Field  string   `yaml:"field"`  // field, e.g. "net/http.Request.Body"
	Labels []string `yaml:"labels"`
	Match  string   `yaml:"match"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
}

// Sink is a dangerous operation.
type Sink struct {
	Symbol string `yaml:"symbol"`
	// Args are the argument positions (receiver excluded) that are dangerous.
	// Empty means every argument. Use "recv" in Selectors for the receiver.
	Args []int `yaml:"args"`
	// Recv marks the receiver as dangerous too.
	Recv bool `yaml:"recv"`
	// Accepts lists the labels the sink reacts to. Empty means any label.
	Accepts  []string `yaml:"accepts"`
	Category string   `yaml:"category"`
	Match    string   `yaml:"match"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
}

// Sanitizer removes labels (and optionally adds others) from what flows from
// its inputs to its result.
type Sanitizer struct {
	Symbol string `yaml:"symbol"`
	// Drops lists the removed labels. "*" removes every label.
	Drops []string `yaml:"drops"`
	// Adds lists labels attached to values that had a dropped label, e.g.
	// "html_escaped": still dangerous for other sinks.
	Adds []string `yaml:"adds"`
	// Flows overrides the default "all arguments flow to result0".
	Flows []Flow `yaml:"flows"`
	Match string `yaml:"match"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
	// AllowSanitizerExpansion must be set to expand a sanitizer over the
	// implementations of an interface. Over-matching a sanitizer makes taint
	// disappear (a missed flow), so it is off unless asked for (ADR-030).
	AllowSanitizerExpansion bool `yaml:"allow_sanitizer_expansion"`
}

// Propagator describes how taint moves through a function without a body.
type Propagator struct {
	Symbol string `yaml:"symbol"`
	Flows  []Flow `yaml:"flows"`
	Match  string `yaml:"match"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
}

// Flow moves taint between two positions of a call.
type Flow struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// Position kinds of a Selector.
const (
	SelRecv   = "recv"
	SelArg    = "arg"
	SelResult = "result"
)

// Selector addresses a position of a call.
type Selector struct {
	Kind  string // SelRecv, SelArg or SelResult
	Index int
	All   bool // "args" or "results": every argument/result
}

// ParseSelector parses "recv", "arg0", "args", "result", "result1", "results".
func ParseSelector(s string) (Selector, error) {
	switch s {
	case "recv":
		return Selector{Kind: SelRecv}, nil
	case "args":
		return Selector{Kind: SelArg, All: true}, nil
	case "result":
		return Selector{Kind: SelResult}, nil
	case "results":
		return Selector{Kind: SelResult, All: true}, nil
	}
	for _, p := range []struct{ prefix, kind string }{{"arg", SelArg}, {"result", SelResult}} {
		if rest, ok := strings.CutPrefix(s, p.prefix); ok {
			n, err := strconv.Atoi(rest)
			if err != nil || n < 0 {
				break
			}
			return Selector{Kind: p.kind, Index: n}, nil
		}
	}
	return Selector{}, fmt.Errorf("invalid selector %q (want recv, argN, args, result, resultN or results)", s)
}

// Parse reads one rule file.
func Parse(r io.Reader, origin string) (*RuleSet, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var rs RuleSet
	if err := dec.Decode(&rs); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	rs.Origin = origin
	if err := rs.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	return &rs, nil
}

func (rs *RuleSet) validate() error {
	checkMatch := func(m, what string) error {
		switch m {
		case "", MatchExact:
			return nil
		case MatchImplementations:
			return nil
		}
		return fmt.Errorf("%s: unknown match %q", what, m)
	}
	for i := range rs.Sources {
		s := &rs.Sources[i]
		if s.Kind == "" {
			s.Kind = KindCallResult
		}
		what := fmt.Sprintf("sources[%d]", i)
		if err := checkMatch(s.Match, what); err != nil {
			return err
		}
		if len(s.Labels) == 0 {
			return fmt.Errorf("%s: labels is required", what)
		}
		switch s.Kind {
		case KindCallResult:
			if s.Symbol == "" {
				return fmt.Errorf("%s: symbol is required for kind %s", what, s.Kind)
			}
		case KindParamType:
			if s.Type == "" {
				return fmt.Errorf("%s: type is required for kind %s", what, s.Kind)
			}
		case KindField:
			if s.Field == "" {
				return fmt.Errorf("%s: field is required for kind %s", what, s.Kind)
			}
		default:
			return fmt.Errorf("%s: unknown kind %q", what, s.Kind)
		}
	}
	for i, s := range rs.Sinks {
		what := fmt.Sprintf("sinks[%d]", i)
		if s.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if err := checkMatch(s.Match, what); err != nil {
			return err
		}
	}
	for i, s := range rs.Sanitizers {
		what := fmt.Sprintf("sanitizers[%d]", i)
		if s.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if len(s.Drops) == 0 {
			return fmt.Errorf("%s: drops is required", what)
		}
		if err := checkMatch(s.Match, what); err != nil {
			return err
		}
		if s.Match == MatchImplementations && !s.AllowSanitizerExpansion {
			return fmt.Errorf("%s: match: implementations on a sanitizer needs allow_sanitizer_expansion: true "+
				"(a sanitizer that matches too much hides real flows)", what)
		}
		if err := validateFlows(s.Flows, what); err != nil {
			return err
		}
	}
	for i, p := range rs.Propagators {
		what := fmt.Sprintf("propagators[%d]", i)
		if p.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if len(p.Flows) == 0 {
			return fmt.Errorf("%s: flows is required", what)
		}
		if err := checkMatch(p.Match, what); err != nil {
			return err
		}
		if err := validateFlows(p.Flows, what); err != nil {
			return err
		}
	}
	return nil
}

func validateFlows(flows []Flow, what string) error {
	for i, f := range flows {
		if _, err := ParseSelector(f.From); err != nil {
			return fmt.Errorf("%s.flows[%d].from: %w", what, i, err)
		}
		to, err := ParseSelector(f.To)
		if err != nil {
			return fmt.Errorf("%s.flows[%d].to: %w", what, i, err)
		}
		_ = to
	}
	return nil
}

// Rules is the merged, indexed view of several rule sets.
type Rules struct {
	Sets []*RuleSet

	CallSources map[string][]Source // by symbol
	ParamTypes  []Source            // param_type sources
	Fields      map[string][]Source // by "pkg.Type.Field"
	Sinks       map[string][]Sink
	Sanitizers  map[string][]Sanitizer
	Propagators map[string][]Propagator

	// Expansions records what Expand did with match: implementations rules.
	Expansions []Expansion
}

// Merge indexes the rule sets. Order does not matter; entries for the same
// symbol accumulate.
func Merge(sets ...*RuleSet) *Rules {
	r := &Rules{
		Sets:        sets,
		CallSources: map[string][]Source{},
		Fields:      map[string][]Source{},
		Sinks:       map[string][]Sink{},
		Sanitizers:  map[string][]Sanitizer{},
		Propagators: map[string][]Propagator{},
	}
	for _, rs := range sets {
		for _, s := range rs.Sources {
			switch s.Kind {
			case KindCallResult:
				r.CallSources[s.Symbol] = append(r.CallSources[s.Symbol], s)
			case KindParamType:
				r.ParamTypes = append(r.ParamTypes, s)
			case KindField:
				r.Fields[s.Field] = append(r.Fields[s.Field], s)
			}
		}
		for _, s := range rs.Sinks {
			r.Sinks[s.Symbol] = append(r.Sinks[s.Symbol], s)
		}
		for _, s := range rs.Sanitizers {
			r.Sanitizers[s.Symbol] = append(r.Sanitizers[s.Symbol], s)
		}
		for _, p := range rs.Propagators {
			r.Propagators[p.Symbol] = append(r.Propagators[p.Symbol], p)
		}
	}
	return r
}

// HasModel reports whether the symbol has a sanitizer or propagator model,
// which replaces the analysis of the function body.
func (r *Rules) HasModel(symbol string) bool {
	return len(r.Sanitizers[symbol]) > 0 || len(r.Propagators[symbol]) > 0
}

// Symbols returns every symbol mentioned by a rule, sorted. It is used to
// report which rules matched nothing.
func (r *Rules) Symbols() []string {
	set := map[string]bool{}
	for s := range r.CallSources {
		set[s] = true
	}
	for s := range r.Sinks {
		set[s] = true
	}
	for s := range r.Sanitizers {
		set[s] = true
	}
	for s := range r.Propagators {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
