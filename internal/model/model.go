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
	KindField      = "field"       // reads of the struct field Field ("pkg.Type.*": any field of the type)

	// BoundaryStoredData is a call that moves data between the code and a data
	// store that is not followed.
	BoundaryStoredData = "stored_data"
	KindParam          = "param" // parameters of the function Symbol (an entry point)
)

// Match modes of a symbol.
const (
	MatchExact           = "exact"
	MatchImplementations = "implementations" // an interface method: expands to its implementations
)

// Origins of a model (ADR-010): who decided that a rule is right.
const (
	OriginBuiltin = "builtin"
	OriginHuman   = "human"
	OriginLLM     = "llm"
)

// Provenance says where a rule comes from. A rule decided by an LLM must say
// when, why and by which model, so that it can be audited and, with
// --deterministic-only, left out.
type Provenance struct {
	Origin       string `yaml:"origin" json:"origin,omitempty"`
	DecidedAt    string `yaml:"decided_at" json:"decided_at,omitempty"`
	Rationale    string `yaml:"rationale" json:"rationale,omitempty"`
	ModelVersion string `yaml:"model_version" json:"model_version,omitempty"`
}

// RuleSet is one parsed rule file.
type RuleSet struct {
	Name string `yaml:"ruleset"`
	// ReadImpls says that the propagators of this set are for the methods of traits,
	// which the code of the project also implements (Iterator::next of a type of
	// its own): the model says what the implementations outside the code do, and the
	// code of the project's own implementations is read as well. Off, a model replaces
	// the analysis of every implementation (what a Go interface method with a model gets).
	ReadImpls bool `yaml:"read_impls"`
	// DefaultOrigin is the origin of the rules that do not say their own.
	DefaultOrigin string `yaml:"origin"`
	// The rules.
	Sources     []Source     `yaml:"sources"`
	Sinks       []Sink       `yaml:"sinks"`
	Sanitizers  []Sanitizer  `yaml:"sanitizers"`
	Propagators []Propagator `yaml:"propagators"`
	// ConfigReads are functions that read configuration: files, the
	// environment, flags. When rivus traces a value back to such a call, it
	// stops and says config_read: the value is in the repository's
	// configuration, not in its code.
	ConfigReads []ConfigRead `yaml:"config_reads"`

	// Origin is where the rule set came from (file name or "builtin:...").
	Origin string `yaml:"-"`
	// Project is true for a rule file that was found in the directory under analysis
	// (<dir>/.rivus/rules): it is part of the code being analysed, and says what that code wants
	// to have said about itself. The report counts what such a file removes (yagihash/rivus #57).
	Project bool `yaml:"-"`
}

// Source introduces taint.
type Source struct {
	Provenance `yaml:",inline"`
	Kind       string `yaml:"kind"`
	Symbol     string `yaml:"symbol"` // call_result
	Type       string `yaml:"type"`   // param_type, e.g. "*net/http.Request"
	Field      string `yaml:"field"`  // field, e.g. "net/http.Request.Body"
	// Params are the positions (receiver excluded) of the parameters of the
	// function Symbol that carry taint; Recv adds the receiver. It declares
	// an entry point of a framework: what the framework hands to the code.
	Params []int `yaml:"params"`
	Recv   bool  `yaml:"recv"`
	// Out are the argument positions (receiver excluded) that a call_result
	// source fills with the data, by writing through the pointer: the way a
	// database read hands rows back (rows.Scan(&x), db.Find(&users)). The
	// pointee is what is tainted. The result of the call is tainted too, unless
	// the rule says only out.
	Out []int `yaml:"out"`
	// Marker, with kind: param and match: implementations (and no symbol), picks
	// the interfaces that have a method of this name (a glob: for the code
	// protoc-gen-go-grpc writes, "mustEmbedUnimplemented*"), and declares as
	// entry points the implementations of each of their other methods: what the
	// framework hands to them in parameters that are messages is the input. The
	// generated code has to exist when the analysis runs.
	Marker string `yaml:"marker"`
	// Extends, with kind: param and no symbol, picks the methods of the
	// subtypes of this type (a type ID, as the extractor writes it) that take a
	// parameter of the type WithParamType, and declares them entry points: what
	// the framework hands to them in the other parameters is the input. It is
	// the marker of the languages that have a class hierarchy in the IR (Dart:
	// the subclasses of grpc's Service, whose handlers take a ServiceCall).
	Extends       string `yaml:"extends"`
	WithParamType string `yaml:"with_param_type"`
	// ExcludeMethods are globs of method names that extends: leaves out (the
	// wrappers the generated code makes around the handlers).
	ExcludeMethods []string `yaml:"exclude_methods"`
	// MarkerCall is the symbol of a function whose call, in the code of the
	// program, says that it serves what the marker is for (grpc.NewServer).
	// When it is called and no interface has the marker, the generated code is
	// probably missing, and the report says so.
	MarkerCall string   `yaml:"marker_call"`
	Labels     []string `yaml:"labels"`
	Match      string   `yaml:"match"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
}

// Sink is a dangerous operation.
type Sink struct {
	Provenance `yaml:",inline"`
	Symbol     string `yaml:"symbol"`
	// Args are the argument positions (receiver excluded) that are dangerous.
	// Empty means every argument. Use "recv" in Selectors for the receiver.
	Args []int `yaml:"args"`
	// Recv marks the receiver as dangerous too.
	Recv bool `yaml:"recv"`
	// Accepts lists the labels the sink reacts to. Empty means any label.
	Accepts  []string `yaml:"accepts"`
	Category string   `yaml:"category"`
	Match    string   `yaml:"match"`
	// WhenArgType makes the sink apply only to calls whose argument Arg has
	// one of Types as its static type (as the IR writes the type): fmt.Fprint
	// writes to a response only when its first argument is a
	// net/http.ResponseWriter, not when it is os.Stderr.
	WhenArgType *ArgTypeCond `yaml:"when_arg_type"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
}

// ArgTypeCond is a condition on the static type of an argument.
type ArgTypeCond struct {
	Arg   int      `yaml:"arg"`
	Types []string `yaml:"types"`
}

// Holds reports whether the type satisfies the condition.
func (c *ArgTypeCond) Holds(t string) bool {
	if c == nil {
		return true
	}
	for _, x := range c.Types {
		if x == t {
			return true
		}
	}
	return false
}

// Sanitizer removes labels (and optionally adds others) from what flows from
// its inputs to its result.
type Sanitizer struct {
	Provenance `yaml:",inline"`
	Symbol     string `yaml:"symbol"`
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
	Provenance `yaml:",inline"`
	Symbol     string `yaml:"symbol"`
	Flows      []Flow `yaml:"flows"`
	// Pure says that no taint flows through the function: it neither
	// returns nor stores anything derived from its arguments (a mutex lock,
	// the clock). It replaces the guess that everything flows, and the
	// uncertainty that goes with the guess.
	Pure bool `yaml:"pure"`
	// Boundary marks a call that moves data between the code and a store that
	// rivus does not follow (a database, a cache): "stored_data". The call is
	// recorded as an uncertainty (stored_data_boundary), so that a place where
	// a second order flow may start or end is listed, not silent. A source
	// rule (field, out) says what comes back from it.
	Boundary string `yaml:"boundary"`
	Match    string `yaml:"match"`
	// MaxImplementations bounds the expansion of match: implementations.
	MaxImplementations int `yaml:"max_implementations"`
}

// ConfigRead names a function that reads configuration.
type ConfigRead struct {
	Symbol string `yaml:"symbol"`
}

// Flow moves taint between two positions of a call.
type Flow struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	// Alias says that the result is the same storage as the argument (a call
	// that hands back a reference to what it was given: Deref::deref, a lock
	// guard): what is written through the result is written to the argument.
	// Without it a flow copies the taint, and the result is a value of its own.
	Alias bool `yaml:"alias"`
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
	rs.fillOrigins()
	if err := rs.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	return &rs, nil
}

// fillOrigins gives every rule an origin: its own, else the rule set's, else
// builtin for the embedded rule sets and human for the others.
func (rs *RuleSet) fillOrigins() {
	def := rs.DefaultOrigin
	if def == "" {
		def = OriginHuman
		if strings.HasPrefix(rs.Origin, "builtin:") {
			def = OriginBuiltin
		}
		rs.DefaultOrigin = def
	}
	fill := func(p *Provenance) {
		if p.Origin == "" {
			p.Origin = def
		}
	}
	for i := range rs.Sources {
		fill(&rs.Sources[i].Provenance)
	}
	for i := range rs.Sinks {
		fill(&rs.Sinks[i].Provenance)
	}
	for i := range rs.Sanitizers {
		fill(&rs.Sanitizers[i].Provenance)
	}
	for i := range rs.Propagators {
		fill(&rs.Propagators[i].Provenance)
	}
}

func (p Provenance) validate(what string) error {
	switch p.Origin {
	case OriginBuiltin, OriginHuman:
		return nil
	case OriginLLM:
		var missing []string
		if p.DecidedAt == "" {
			missing = append(missing, "decided_at")
		}
		if p.Rationale == "" {
			missing = append(missing, "rationale")
		}
		if p.ModelVersion == "" {
			missing = append(missing, "model_version")
		}
		if len(missing) > 0 {
			return fmt.Errorf("%s: a rule decided by an LLM needs %s", what, strings.Join(missing, ", "))
		}
		return nil
	}
	return fmt.Errorf("%s: unknown origin %q (want %s, %s or %s)", what, p.Origin, OriginBuiltin, OriginHuman, OriginLLM)
}

// WithoutOrigin returns a copy of the rule set without the rules of the
// given origin, and the number of rules left out.
func (rs *RuleSet) WithoutOrigin(origin string) (*RuleSet, int) {
	out := *rs
	out.Sources, out.Sinks, out.Sanitizers, out.Propagators = nil, nil, nil, nil
	n := 0
	for _, r := range rs.Sources {
		if r.Origin == origin {
			n++
		} else {
			out.Sources = append(out.Sources, r)
		}
	}
	for _, r := range rs.Sinks {
		if r.Origin == origin {
			n++
		} else {
			out.Sinks = append(out.Sinks, r)
		}
	}
	for _, r := range rs.Sanitizers {
		if r.Origin == origin {
			n++
		} else {
			out.Sanitizers = append(out.Sanitizers, r)
		}
	}
	for _, r := range rs.Propagators {
		if r.Origin == origin {
			n++
		} else {
			out.Propagators = append(out.Propagators, r)
		}
	}
	return &out, n
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
		if err := s.Provenance.validate(what); err != nil {
			return err
		}
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
			for _, o := range s.Out {
				if o < 0 {
					return fmt.Errorf("%s: out: %d is not an argument position", what, o)
				}
			}
		case KindParamType:
			if s.Type == "" {
				return fmt.Errorf("%s: type is required for kind %s", what, s.Kind)
			}
		case KindField:
			if s.Field == "" {
				return fmt.Errorf("%s: field is required for kind %s", what, s.Kind)
			}
		case KindParam:
			if s.Extends != "" {
				if s.Symbol != "" || s.Marker != "" || s.WithParamType == "" {
					return fmt.Errorf("%s: extends goes with with_param_type, and no symbol or marker", what)
				}
				break
			}
			if s.Marker != "" {
				if s.Symbol != "" || s.Match != MatchImplementations {
					return fmt.Errorf("%s: marker goes with match: implementations and no symbol", what)
				}
				break
			}
			if s.Symbol == "" {
				return fmt.Errorf("%s: symbol is required for kind %s", what, s.Kind)
			}
			if len(s.Params) == 0 && !s.Recv {
				return fmt.Errorf("%s: kind %s needs params or recv", what, s.Kind)
			}
		default:
			return fmt.Errorf("%s: unknown kind %q", what, s.Kind)
		}
	}
	for i, s := range rs.Sinks {
		what := fmt.Sprintf("sinks[%d]", i)
		if err := s.Provenance.validate(what); err != nil {
			return err
		}
		if s.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if err := checkMatch(s.Match, what); err != nil {
			return err
		}
	}
	for i, s := range rs.Sanitizers {
		what := fmt.Sprintf("sanitizers[%d]", i)
		if err := s.Provenance.validate(what); err != nil {
			return err
		}
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
	for i, c := range rs.ConfigReads {
		if c.Symbol == "" {
			return fmt.Errorf("config_reads[%d]: symbol is required", i)
		}
	}
	aliased := map[string]bool{}
	for i, p := range rs.Propagators {
		what := fmt.Sprintf("propagators[%d]", i)
		if err := p.Provenance.validate(what); err != nil {
			return err
		}
		if p.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if len(p.Flows) == 0 && !p.Pure {
			return fmt.Errorf("%s: flows is required (or pure: true when nothing flows)", what)
		}
		if len(p.Flows) > 0 && p.Pure {
			return fmt.Errorf("%s: pure: true and flows contradict each other", what)
		}
		if p.Boundary != "" && p.Boundary != BoundaryStoredData {
			return fmt.Errorf("%s: unknown boundary %q (stored_data)", what, p.Boundary)
		}
		if err := checkMatch(p.Match, what); err != nil {
			return err
		}
		if err := validateFlows(p.Flows, what); err != nil {
			return err
		}
		for _, f := range p.Flows {
			if f.Alias {
				if aliased[p.Symbol] {
					return fmt.Errorf("%s: %s has another model with an alias flow: a function has one", what, p.Symbol)
				}
				aliased[p.Symbol] = true
			}
		}
	}
	return nil
}

func validateFlows(flows []Flow, what string) error {
	aliases := 0
	for i, f := range flows {
		if f.Alias {
			if aliases++; aliases > 1 {
				return fmt.Errorf("%s.flows[%d]: a model has at most one flow with alias (the result is the same storage as one argument)", what, i)
			}
		}
		if _, err := ParseSelector(f.From); err != nil {
			return fmt.Errorf("%s.flows[%d].from: %w", what, i, err)
		}
		to, err := ParseSelector(f.To)
		if err != nil {
			return fmt.Errorf("%s.flows[%d].to: %w", what, i, err)
		}
		if f.Alias {
			if to.Kind != SelResult || to.All {
				return fmt.Errorf("%s.flows[%d]: alias goes with to: result", what, i)
			}
			from, _ := ParseSelector(f.From)
			if from.Kind == SelResult {
				return fmt.Errorf("%s.flows[%d]: alias needs an argument to be the same storage as", what, i)
			}
		}
	}
	return nil
}

// AliasArgs gives, for the functions that have a flow with alias, the position
// of the argument their result is the same storage as (the receiver is the
// argument 0 for the languages that pass it first).
func (r *Rules) AliasArgs() (map[string]int, error) {
	out := map[string]int{}
	for sym, ps := range r.Propagators {
		for _, p := range ps {
			for _, f := range p.Flows {
				if !f.Alias {
					continue
				}
				sel, err := ParseSelector(f.From)
				if err != nil {
					continue
				}
				idx := 0
				if sel.Kind == SelArg && !sel.All {
					idx = sel.Index
				}
				// a function has one argument that its result is the same storage as: a second one (in another
				// model of the same symbol, from another rule set) would take the place of the first without a word
				if _, ok := out[sym]; ok {
					return nil, fmt.Errorf("%s: more than one model has an alias flow", sym)
				}
				out[sym] = idx
			}
		}
	}
	return out, nil
}

// Validate checks what only the merged rules can show: what the rule sets do not see of one another.
func (r *Rules) Validate() error {
	_, err := r.AliasArgs()
	return err
}

// Rules is the merged, indexed view of several rule sets.
type Rules struct {
	Sets []*RuleSet

	CallSources map[string][]Source // by symbol
	ParamTypes  []Source            // param_type sources
	ParamFuncs  map[string][]Source // param sources, by function symbol
	Fields      map[string][]Source // by "pkg.Type.Field"
	Sinks       map[string][]Sink
	Sanitizers  map[string][]Sanitizer
	Propagators map[string][]Propagator
	// ReadImpls are the symbols of the propagators of a set with read_impls.
	ReadImpls   map[string]bool
	ConfigReads map[string]bool

	// Expansions records what Expand did with match: implementations rules.
	Expansions []Expansion

	// DeterministicOnly is set when the rules decided by an LLM were left
	// out, and IgnoredModels says how many (ADR-010).
	DeterministicOnly bool
	IgnoredModels     int
}

// WithoutOrigin merges the rule sets after leaving out the rules of one
// origin. It is how --deterministic-only works: with the rules decided by an
// LLM removed, rivus alone is deterministic.
func WithoutOrigin(origin string, sets ...*RuleSet) *Rules {
	var kept []*RuleSet
	ignored := 0
	for _, rs := range sets {
		k, n := rs.WithoutOrigin(origin)
		ignored += n
		kept = append(kept, k)
	}
	r := Merge(kept...)
	r.DeterministicOnly = origin == OriginLLM
	r.IgnoredModels = ignored
	return r
}

// Merge indexes the rule sets. Order does not matter; entries for the same
// symbol accumulate.
func Merge(sets ...*RuleSet) *Rules {
	r := &Rules{
		Sets:        sets,
		CallSources: map[string][]Source{},
		Fields:      map[string][]Source{},
		ParamFuncs:  map[string][]Source{},
		Sinks:       map[string][]Sink{},
		Sanitizers:  map[string][]Sanitizer{},
		Propagators: map[string][]Propagator{},
		ReadImpls:   map[string]bool{},
		ConfigReads: map[string]bool{},
	}
	for _, rs := range sets {
		for _, s := range rs.Sources {
			switch s.Kind {
			case KindCallResult:
				r.CallSources[s.Symbol] = append(r.CallSources[s.Symbol], s)
			case KindParamType:
				r.ParamTypes = append(r.ParamTypes, s)
			case KindParam:
				if s.Marker != "" || s.Extends != "" {
					break // expanded over what the program has (Expand, ExpandSubtypes)
				}
				r.ParamFuncs[s.Symbol] = append(r.ParamFuncs[s.Symbol], s)
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
			if rs.ReadImpls {
				r.ReadImpls[p.Symbol] = true
			}
		}
		for _, c := range rs.ConfigReads {
			r.ConfigReads[c.Symbol] = true
		}
	}
	return r
}

// ReadsImpls reports whether the code of the project's own implementations of the
// method is read as well as the model applied (a set with read_impls).
func (r *Rules) ReadsImpls(symbol string) bool { return r.ReadImpls[symbol] }

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
