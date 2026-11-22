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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

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
	// SelfFirst says that the positions of the rules of this set count every operand of a call, the receiver
	// first (`arg0`): the models of Rust name the arguments with `self` first. A call through a trait (a
	// dynamic call), whose receiver is not one of its arguments for the engine, is then given the positions
	// the rule means: `from: args` is the receiver and the arguments.
	SelfFirst bool `yaml:"self_first"`
	// DefaultOrigin is the origin of the rules that do not say their own.
	DefaultOrigin string `yaml:"origin"`
	// The rules.
	Sources     []Source     `yaml:"sources"`
	Sinks       []Sink       `yaml:"sinks"`
	Sanitizers  []Sanitizer  `yaml:"sanitizers"`
	Propagators []Propagator `yaml:"propagators"`
	// Globals say what a package variable holds for the code that reads it.
	Globals []GlobalRule `yaml:"globals"`
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
	// Named are the names of named parameters (Dart) that are dangerous, whatever their position:
	// `named: [kind]`. A call that does not pass the name has no such argument. The site is made from
	// the call, which carries the names of the parameters (ir.Instr.ArgNames): sites.sinksAt turns them
	// into positions, and what is read after that is Args.
	Named []string `yaml:"named"`
	// NamedUnresolved are the names of Named that no call of the symbol in the program has: a
	// misspelling, or a parameter that was renamed. sinksAt sets it, and the sink then reads the whole
	// call (the taint engine says so: named_unresolved). Not a field of the rules.
	NamedUnresolved []string `yaml:"-"`
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
	// Parts say, for a position, which part of the value is dangerous: the field (a list of names) below
	// it. A position that Args or Recv name is dangerous whole, whatever Parts say of it. With Parts
	// and no Args, only the positions of Parts are read (docs/DESIGN-field-sinks.md).
	Parts []SinkPart `yaml:"parts"`
	// Field is the short form of one part: args: [1] with field: A.B, or recv: true with field: A.
	Field string `yaml:"field"`
}

// SinkPart is a position of a call and the field below it that is dangerous.
type SinkPart struct {
	// Arg is the position of an argument (the receiver is not one of them); Recv: true is the receiver.
	Arg  *int `yaml:"arg"`
	Recv bool `yaml:"recv"`
	// Named is the name of a named parameter (Dart) in place of Arg.
	Named string `yaml:"named"`
	// Field is the names of the fields from the value to the dangerous one, separated by dots. "[]" is
	// an element of a slice, an array or a map. The names are those the IR has; the steps that are
	// not names (the pointer behind a field, the pointer in an interface) are not written.
	Field string `yaml:"field"`
}

// PartList is the parts of the sink: Parts, or the one that the short form (field) makes.
func (s *Sink) PartList() []SinkPart {
	if s.Field == "" {
		return s.Parts
	}
	switch {
	case len(s.Args) == 1 && !s.Recv && len(s.Named) == 0:
		a := s.Args[0]
		return []SinkPart{{Arg: &a, Field: s.Field}}
	case len(s.Args) == 0 && !s.Recv && len(s.Named) == 1:
		return []SinkPart{{Named: s.Named[0], Field: s.Field}}
	case len(s.Args) == 0 && s.Recv:
		return []SinkPart{{Recv: true, Field: s.Field}}
	}
	return nil
}

// HasParts reports whether the sink names parts of its values.
func (s *Sink) HasParts() bool { return s.Field != "" || len(s.Parts) > 0 }

// WholeArgs are the positions of the arguments (out of n) that are dangerous whole. A sink with parts
// reads whole only the positions that args names: its parts are for the rest, and no args does not
// mean every argument (as it does for a sink without parts).
func (s *Sink) WholeArgs(n int) []int {
	if s.Field != "" {
		return nil // the short form: its one position is the part
	}
	if len(s.Args) == 0 && !s.HasParts() && len(s.Named) == 0 {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	var out []int
	for _, i := range s.Args {
		if i >= 0 && i < n {
			out = append(out, i)
		}
	}
	return out
}

// WholeRecv reports whether the receiver is dangerous whole.
func (s *Sink) WholeRecv() bool { return s.Recv && s.Field == "" }

// ArgTypeCond is a condition on the static type of an argument.
//
// Arg counts the arguments as args does: the receiver of a method is not one of them. Recv: true
// makes the condition on the type of the receiver instead (Arg is then not read). A call that has
// no receiver does not satisfy a condition on one.
type ArgTypeCond struct {
	Arg   int      `yaml:"arg"`
	Recv  bool     `yaml:"recv"`
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

// TypeCanonicalizer gives the type behind the aliases of a type name (ir.TypeCanon).
type TypeCanonicalizer interface{ Canon(string) string }

// HoldsWith is Holds, where a type and the same type under another name (an
// alias) are the same: canon gives the name of the type behind the aliases.
func (c *ArgTypeCond) HoldsWith(t string, canon TypeCanonicalizer) bool {
	if c == nil || c.Holds(t) {
		return true
	}
	ct := canon.Canon(t)
	for _, x := range c.Types {
		if canon.Canon(x) == ct {
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
	// Calls say that the function calls a function it was given (a callback), and with what: the
	// engine applies the summary of the function that was passed, at the call, with the values this
	// row names (CallRow). A row that has a ret also has flows that stand for it when the function
	// that was passed is not one that can be told (see CallRow.DerivedFlows).
	Calls []CallRow `yaml:"calls"`
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

// Holds values of a global rule.
const (
	// HoldsNothing: what is written to the variable (and to what it points to) is not read back by anyone: it goes out of
	// the program (os.Stderr). Reading the variable gives no taint; the call that writes to it is still seen (a sink rule
	// may name it).
	HoldsNothing = "nothing"
	// HoldsEverything is the default, and undoes a rule that says nothing (a rule file of the project, over the built-in one).
	HoldsEverything = "everything"
)

// GlobalRule says what a package variable holds. It is for the variables that many functions use as the
// handle of a place the data goes out to: what was written to it is not data that others read.
type GlobalRule struct {
	Provenance `yaml:",inline"`
	// Symbol is the name of the variable with its package: os.Stderr.
	Symbol string `yaml:"symbol"`
	Holds  string `yaml:"holds"`
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
	// Approximate says that the flow is a guess that the model makes (the output of an external program:
	// whether it carries the arguments depends on the program): the text is the reason. A finding whose
	// path goes through the flow has one uncertainty of kind model_approximation, with the reason as its
	// detail, so that it is not reported as a path without any uncertainty (C-8).
	Approximate string `yaml:"approximate"`
	// Assumed is set on the flows that DerivedFlows makes (not read from a file): the use of the flow is
	// an assumed_call, and the text is its detail.
	Assumed string `yaml:"-"`
}

// Modes of a call row.
const (
	CallModeCall  = "call"  // the callback is called, and the caller waits for it (the default)
	CallModeGo    = "go"    // it runs on its own (a thread, a task): what it does counts, what it returns does not
	CallModeDefer = "defer" // it runs later: the same
)

// CallRow says that the function calls one of its arguments (or the receiver).
//
//	calls:
//	  - fn: arg0              # which function value is called (recv or argN; the receiver is not counted in argN)
//	    with: [recv]          # what it is called with, parameter by parameter (recv, argN; none: nothing flows in)
//	    ret: result0          # where its result goes (result, resultN, recv, argN; none: dropped)
//	    throw: result0        # where what it throws goes (the same; none: it is thrown out of the caller)
//	    writes: {0: recv}     # where what it writes through its parameter N goes, besides the value that was given
//	    mode: call            # call (default), go or defer
//
// The names are the positions of the call, as flows names them. Whole values only (no projection:
// `recv` is the receiver with what is inside it), which is what flows does too.
type CallRow struct {
	Fn string `yaml:"fn"`
	// Method is, instead of Fn, the method of the same trait that a provided (default) method of the library calls
	// on its receiver (`std::io::Write::write_all` calls `write`): the implementations of it that the program has are
	// called, with the receiver and With. The positions of a row with a method count every operand of the call, the
	// receiver first (arg0), as the models of Rust do.
	Method string `yaml:"method"`
	// With is what the callback is called with, parameter by parameter from the first: entry k fills
	// parameter k, and "" fills none. The YAML is a list of positions (`[arg1, recv]`), or of
	// `{param: 1, from: recv}` that say the parameter by number (`from: none` leaves it empty), so that
	// a row for a callback of (ctx, m) can fill m alone: `[{param: 1, from: recv}]`.
	With WithList `yaml:"with"`
	// Params is the number of parameters of the callback, when the row says it (optional): the load then
	// warns of a `with` that does not fill them (CallWarnings).
	Params int            `yaml:"params"`
	Ret    string         `yaml:"ret"`
	Throw  string         `yaml:"throw"`
	Writes map[int]string `yaml:"writes"`
	Mode   string         `yaml:"mode"`
}

// WithList is the `with` of a row of calls (see CallRow.With).
type WithList []string

// UnmarshalYAML reads the two forms of with: positions in order, or {param: N, from: position} entries.
func (w *WithList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: with is a list of positions (recv, argN) or of {param: N, from: position}", n.Line)
	}
	var out []string
	named, plain := false, false
	for _, e := range n.Content {
		switch e.Kind {
		case yaml.ScalarNode:
			plain = true
			if e.Value == "" {
				return fmt.Errorf("line %d: a with entry is a position (recv, argN); {param: N, from: none} leaves a parameter empty", e.Line)
			}
			out = append(out, e.Value)
		case yaml.MappingNode:
			named = true
			param, from := -1, ""
			seenFrom := false
			for i := 0; i+1 < len(e.Content); i += 2 {
				k, v := e.Content[i], e.Content[i+1]
				switch k.Value {
				case "param":
					if err := v.Decode(&param); err != nil || param < 0 || param > 31 {
						return fmt.Errorf("line %d: with.param is the number of the parameter of the callback, from 0", v.Line)
					}
				case "from":
					seenFrom = true
					from = v.Value
					if from == "none" {
						from = ""
					}
				default:
					return fmt.Errorf("line %d: with: %q is not a key of {param, from}", k.Line, k.Value)
				}
			}
			if param < 0 || !seenFrom {
				return fmt.Errorf("line %d: a with entry of the named form has param and from", e.Line)
			}
			for len(out) <= param {
				out = append(out, "")
			}
			if out[param] != "" {
				return fmt.Errorf("line %d: with: parameter %d is given twice", e.Line, param)
			}
			out[param] = from
		default:
			return fmt.Errorf("line %d: a with entry is a position or {param: N, from: position}", e.Line)
		}
	}
	if named && plain {
		return fmt.Errorf("line %d: with: write every entry as a position or every entry as {param, from}, not both", n.Line)
	}
	*w = out
	return nil
}

// DerivedFlows are the flows that stand for the rows of calls when the function that was passed
// cannot be told (it is a parameter, read from memory, one of more than four): what goes in
// (with) comes out (ret), whole, and each use says assumed_call. A flow that is written is added to
// these, not replaced by them. A row without a ret or without a with has none: nothing is carried.
func (p *Propagator) DerivedFlows(row int) []Flow {
	c := p.Calls[row]
	if c.Ret == "" || c.Mode == CallModeGo || c.Mode == CallModeDefer {
		return nil
	}
	var out []Flow
	for _, w := range c.With {
		if w == "" {
			continue
		}
		out = append(out, Flow{From: w, To: c.Ret, Assumed: "assumed_call: the function given to " + p.Symbol + " is not known here: what it is called with is taken to come out as what it returns"})
	}
	return out
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
	// Content is the written content of the position (`recv.content`, `arg0.content`): what a writer
	// holds of the data written to it, as a step of its own below the value, beside the fields of
	// the value. Only a destination (`to:`) of a flow can say it (ParseDest); ADR-082.
	Content bool
}

// ContentSuffix is how a model names the content step of a destination, and ContentStep is the step it
// is in an access path. A step that is no field name of any language: a field of the writer (a file name,
// a setting) is never a read of what was written to it.
const (
	ContentSuffix = ".content"
	ContentStep   = "#content"
)

// ParseDest parses the destination of a flow: a Selector, or `recv.content` / `argN.content`, the content
// of a writer (a destination that is not a result).
func ParseDest(s string) (Selector, error) {
	if base, ok := strings.CutSuffix(s, ContentSuffix); ok {
		sel, err := ParseSelector(base)
		if err != nil || sel.Kind == SelResult || sel.All {
			return Selector{}, fmt.Errorf("invalid destination %q (%s is for recv or argN)", s, ContentSuffix)
		}
		sel.Content = true
		return sel, nil
	}
	return ParseSelector(s)
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
	// A rule file is one document: what follows a `---` would be neither read nor checked, and a sink or a
	// sanitizer written there would be silently void (yagihash/rivus #52). An empty one (a trailing `---`) is fine.
	for {
		var next yaml.Node
		err := dec.Decode(&next)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: after the first YAML document: %w", origin, err)
		}
		if next.Kind != 0 && !(next.Kind == yaml.DocumentNode && len(next.Content) == 1 && next.Content[0].Kind == yaml.ScalarNode && next.Content[0].Tag == "!!null") {
			return nil, fmt.Errorf("%s: line %d: a rule file is one YAML document: what follows the second \"---\" is not read (put the rules in one document, or in another file)", origin, next.Line)
		}
	}
	rs.Origin = origin
	// `builtin` says that rivus itself embeds the rule: a file that comes from outside cannot say it (it would
	// make its own rules look vetted, and the filters of origin would take it for one of rivus')
	if !strings.HasPrefix(origin, "builtin:") {
		if err := rs.rejectBuiltinOrigin(); err != nil {
			return nil, fmt.Errorf("%s: %w", origin, err)
		}
	}
	rs.fillOrigins()
	if err := rs.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	return &rs, nil
}

// rejectBuiltinOrigin is an error for a rule set that claims the origin builtin (itself, or in one of its rules).
func (rs *RuleSet) rejectBuiltinOrigin() error {
	const msg = "origin: builtin is for the rules that rivus embeds; a rule file says human or llm (or says nothing: human)"
	if rs.DefaultOrigin == OriginBuiltin {
		return fmt.Errorf("origin: %s", msg)
	}
	check := func(what string, i int, p Provenance) error {
		if p.Origin == OriginBuiltin {
			return fmt.Errorf("%s[%d].origin: %s", what, i, msg)
		}
		return nil
	}
	for i, s := range rs.Sources {
		if err := check("sources", i, s.Provenance); err != nil {
			return err
		}
	}
	for i, s := range rs.Sinks {
		if err := check("sinks", i, s.Provenance); err != nil {
			return err
		}
	}
	for i, s := range rs.Sanitizers {
		if err := check("sanitizers", i, s.Provenance); err != nil {
			return err
		}
	}
	for i, s := range rs.Propagators {
		if err := check("propagators", i, s.Provenance); err != nil {
			return err
		}
	}
	for i, s := range rs.Globals {
		if err := check("globals", i, s.Provenance); err != nil {
			return err
		}
	}
	return nil
}

// RedactValues is the error without the text of the values in it: the YAML decoder quotes the value it could not
// read (`cannot unmarshal !!str `...` into ...`), and a rule file that is a link to another file would show the
// start of that file.
func RedactValues(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", backquoted.ReplaceAllString(err.Error(), "`…`"))
}

var backquoted = regexp.MustCompile("`[^`]*`")

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
	for i := range rs.Globals {
		fill(&rs.Globals[i].Provenance)
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

// checkFields refuses the fields that the kind of a source does not read.
func (s *Source) checkFields(what string) error {
	var bad []string
	has := func(name string, set bool) {
		if set {
			bad = append(bad, name)
		}
	}
	switch s.Kind {
	case KindCallResult:
		has("type", s.Type != "")
		has("field", s.Field != "")
		has("params", len(s.Params) > 0)
		has("marker", s.Marker != "")
		has("extends", s.Extends != "")
		has("with_param_type", s.WithParamType != "")
		has("exclude_methods", len(s.ExcludeMethods) > 0)
		has("marker_call", s.MarkerCall != "")
	case KindParamType, KindField:
		has("symbol", s.Symbol != "")
		has("params", len(s.Params) > 0)
		has("recv", s.Recv)
		has("out", len(s.Out) > 0)
		has("marker", s.Marker != "")
		has("extends", s.Extends != "")
		has("with_param_type", s.WithParamType != "")
		has("exclude_methods", len(s.ExcludeMethods) > 0)
		has("marker_call", s.MarkerCall != "")
		// only a call_result and a param are expanded to their implementations
		has("match: implementations", s.Match == MatchImplementations)
		if s.Kind == KindParamType {
			has("field", s.Field != "")
		} else {
			has("type", s.Type != "")
		}
	case KindParam:
		has("out", len(s.Out) > 0)
		has("type", s.Type != "")
		has("field", s.Field != "")
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s: kind %s does not use %s", what, s.Kind, strings.Join(bad, ", "))
	}
	return nil
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
		// a field that the kind does not read is a rule that does not do what it says: refused, not ignored
		if err := s.checkFields(what); err != nil {
			return err
		}
		switch s.Kind {
		case KindCallResult:
			if s.Symbol == "" {
				return fmt.Errorf("%s: symbol is required for kind %s", what, s.Kind)
			}
			if err := checkSymbol(s.Symbol, what); err != nil {
				return err
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
			if err := checkSymbol(s.Symbol, what); err != nil {
				return err
			}
			if len(s.Params) == 0 && !s.Recv {
				return fmt.Errorf("%s: kind %s needs params or recv", what, s.Kind)
			}
			for _, o := range s.Params {
				if o < 0 {
					return fmt.Errorf("%s: params: %d is not a parameter position", what, o)
				}
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
		if err := checkSymbol(s.Symbol, what); err != nil {
			return err
		}
		if err := checkMatch(s.Match, what); err != nil {
			return err
		}
		for _, a := range s.Args {
			if a < 0 {
				return fmt.Errorf("%s: args: %d is not an argument position (the receiver is recv: true)", what, a)
			}
		}
		for _, n := range s.Named {
			if err := checkParamName(n, what+": named"); err != nil {
				return err
			}
		}
		if err := validateSinkParts(&s, what); err != nil {
			return err
		}
		if c := s.WhenArgType; c != nil {
			if c.Arg < 0 {
				return fmt.Errorf("%s: when_arg_type: arg %d is not an argument position (the receiver is recv: true)", what, c.Arg)
			}
			if c.Recv && c.Arg != 0 {
				return fmt.Errorf("%s: when_arg_type: recv: true is the condition on the receiver, and arg %d is not read", what, c.Arg)
			}
			if len(c.Types) == 0 {
				return fmt.Errorf("%s: when_arg_type needs types: without them the condition never holds and the sink never applies", what)
			}
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
		if err := checkSymbol(s.Symbol, what); err != nil {
			return err
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
	for i, g := range rs.Globals {
		what := fmt.Sprintf("globals[%d]", i)
		if err := g.Provenance.validate(what); err != nil {
			return err
		}
		if g.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if g.Holds != HoldsNothing && g.Holds != HoldsEverything {
			return fmt.Errorf("%s: holds is %q (want %s or %s)", what, g.Holds, HoldsNothing, HoldsEverything)
		}
	}
	for i, p := range rs.Propagators {
		what := fmt.Sprintf("propagators[%d]", i)
		if err := p.Provenance.validate(what); err != nil {
			return err
		}
		if p.Symbol == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if err := checkSymbol(p.Symbol, what); err != nil {
			return err
		}
		if len(p.Flows) == 0 && !p.Pure && len(p.Calls) == 0 {
			return fmt.Errorf("%s: flows is required (or pure: true when nothing flows, or calls)", what)
		}
		if err := p.validateCalls(what); err != nil {
			return err
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

// validateCalls checks the rows of calls, and that a row that returns something has a way to carry it
// when the function that is called cannot be told: with and ret, or flows that say it.
func (p *Propagator) validateCalls(what string) error {
	for i, c := range p.Calls {
		w := fmt.Sprintf("%s.calls[%d]", what, i)
		pos := func(field, text string, results bool) (Selector, error) {
			sel, err := ParseSelector(text)
			if err != nil || sel.All || (sel.Kind == SelResult && !results) {
				return sel, fmt.Errorf("%s.%s: %q is not a position (recv or argN%s)", w, field, text, map[bool]string{true: ", resultN", false: ""}[results])
			}
			return sel, nil
		}
		if (c.Fn == "") == (c.Method == "") {
			return fmt.Errorf("%s: exactly one of fn (the argument that is called) and method (the method of the receiver's trait that is called) is required", w)
		}
		if c.Method != "" {
			for _, r := range c.Method {
				if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
					return fmt.Errorf("%s.method: %q is not a method name", w, c.Method)
				}
			}
			if c.Ret != "" || c.Throw != "" || len(c.Writes) > 0 || c.Mode != "" {
				return fmt.Errorf("%s: a row with a method calls it as a call is: ret, throw, writes and mode are not for it", w)
			}
		} else if _, err := pos("fn", c.Fn, false); err != nil {
			return err
		}
		for j, x := range c.With {
			if x == "" {
				continue
			}
			if _, err := pos(fmt.Sprintf("with[%d]", j), x, false); err != nil {
				return err
			}
		}
		if c.Ret != "" {
			if _, err := pos("ret", c.Ret, true); err != nil {
				return err
			}
		}
		if c.Throw != "" {
			if _, err := pos("throw", c.Throw, true); err != nil {
				return err
			}
		}
		for k, x := range c.Writes {
			if k < 0 {
				return fmt.Errorf("%s.writes: %d is not a parameter position", w, k)
			}
			if _, err := pos(fmt.Sprintf("writes[%d]", k), x, true); err != nil {
				return err
			}
		}
		switch c.Mode {
		case "", CallModeCall, CallModeGo, CallModeDefer:
		default:
			return fmt.Errorf("%s.mode: %q (want call, go or defer)", w, c.Mode)
		}
		if (c.Mode == CallModeGo || c.Mode == CallModeDefer) && c.Ret != "" {
			return fmt.Errorf("%s: a callback that runs on its own has no result to read (ret with mode %s)", w, c.Mode)
		}
		// a row that returns something is a flow when its function cannot be told: it has to have one,
		// from the row or written (an approximation that errs towards finding)
		if c.Ret != "" && len(p.DerivedFlows(i)) == 0 && len(p.Flows) == 0 {
			return fmt.Errorf("%s: ret needs with (what goes in comes out when the function is not known), or flows that say where the result comes from", w)
		}
	}
	return nil
}

// checkSymbol rejects a symbol with a space around it: it is a mistake that no call has the symbol of, and the
// rule would silently not apply (yagihash/rivus #53).
func checkSymbol(sym, what string) error {
	if sym != strings.TrimSpace(sym) {
		return fmt.Errorf("%s: symbol %q has a space around it", what, sym)
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
		to, err := ParseDest(f.To)
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
//
// A function that two models give different arguments is an error, and the map has the first
// of them (the others are not lost with it: a rule that is wrong costs its own symbol only).
func (r *Rules) AliasArgs() (map[string]int, error) {
	out := map[string]int{}
	var err0 error
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
				// (two that name the same argument say the same: the rules of two interfaces that one type implements
				// are both copied to its method)
				if was, ok := out[sym]; ok {
					if was != idx && err0 == nil {
						err0 = fmt.Errorf("%s: more than one model has an alias flow, and they name different arguments", sym)
					}
					continue
				}
				out[sym] = idx
			}
		}
	}
	return out, err0
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
	// Calls are the symbols that have a row of calls in a propagator.
	Calls map[string]bool
	// ReadImpls are the symbols of the propagators of a set with read_impls.
	ReadImpls map[string]bool
	// SelfFirst are the symbols of the propagators of a set with self_first.
	SelfFirst   map[string]bool
	ConfigReads map[string]bool
	// GlobalsHoldNothing are the package variables that hold nothing for the code that reads them (a rule that says
	// everything, in any set, takes the variable out).
	GlobalsHoldNothing map[string]bool

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
		Calls:       map[string]bool{},
		ReadImpls:   map[string]bool{},
		SelfFirst:   map[string]bool{},
		ConfigReads: map[string]bool{},

		GlobalsHoldNothing: map[string]bool{},
	}
	everything := map[string]bool{}
	for _, rs := range sets {
		for _, g := range rs.Globals {
			if g.Holds == HoldsNothing {
				r.GlobalsHoldNothing[g.Symbol] = true
			} else {
				everything[g.Symbol] = true
			}
		}
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
			if len(p.Calls) > 0 {
				r.Calls[p.Symbol] = true
			}
			if rs.ReadImpls {
				r.ReadImpls[p.Symbol] = true
			}
			if rs.SelfFirst {
				r.SelfFirst[p.Symbol] = true
			}
		}
		for _, c := range rs.ConfigReads {
			r.ConfigReads[c.Symbol] = true
		}
	}
	for sym := range everything {
		delete(r.GlobalsHoldNothing, sym)
	}
	return r
}

// HoldsNothing reports whether the package variable holds nothing for the code that reads it.
func (r *Rules) HoldsNothing(symbol string) bool { return r != nil && r.GlobalsHoldNothing[symbol] }

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

// validateSinkParts checks the parts of a sink and its short form.
func validateSinkParts(s *Sink, what string) error {
	if s.Field != "" {
		if len(s.Parts) > 0 {
			return fmt.Errorf("%s: field is the short form of one part: it cannot come with parts", what)
		}
		if !(len(s.Args)+len(s.Named) == 1 && !s.Recv) && !(len(s.Args)+len(s.Named) == 0 && s.Recv) {
			return fmt.Errorf("%s: field names the part of one position: give one position (args: [n], named: [name], or recv: true), or write parts", what)
		}
		return checkFieldPath(s.Field, what+": field")
	}
	for i, p := range s.Parts {
		w := fmt.Sprintf("%s: parts[%d]", what, i)
		switch {
		case p.Recv && (p.Arg != nil || p.Named != ""):
			return fmt.Errorf("%s: arg, named and recv: true are one or the other", w)
		case p.Arg != nil && p.Named != "":
			return fmt.Errorf("%s: arg and named are one or the other", w)
		case p.Named != "":
			if err := checkParamName(p.Named, w+": named"); err != nil {
				return err
			}
		case !p.Recv && p.Arg == nil:
			return fmt.Errorf("%s: arg (an argument position), named (the name of a named parameter) or recv: true is required", w)
		case p.Arg != nil && *p.Arg < 0:
			return fmt.Errorf("%s: arg: %d is not an argument position (the receiver is recv: true)", w, *p.Arg)
		}
		if p.Field == "" {
			return fmt.Errorf("%s: field is required (a position whole is args or recv)", w)
		}
		if err := checkFieldPath(p.Field, w+": field"); err != nil {
			return err
		}
	}
	return nil
}

// checkParamName checks the name of a named parameter: an identifier of the language.
func checkParamName(n, what string) error {
	if n == "" {
		return fmt.Errorf("%s: the name is empty", what)
	}
	for i, r := range n {
		if r == '_' || r == '$' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r) {
			continue
		}
		return fmt.Errorf("%s: %q is not the name of a parameter", what, n)
	}
	return nil
}

// FieldNames splits the field of a part into the names of the fields (and "[]" for an element).
func FieldNames(field string) []string { return strings.Split(field, ".") }

func checkFieldPath(field, what string) error {
	for _, n := range FieldNames(field) {
		if n == "" || n == "*" || strings.ContainsAny(n, "/ \t") {
			return fmt.Errorf("%s: %q is not a path of field names (names separated by dots; \"[]\" for an element; the steps of a pointer are not written)", what, field)
		}
	}
	return nil
}
