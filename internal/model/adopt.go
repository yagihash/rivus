package model

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// AdoptRule is one decision of "bulk adoption" (docs/DESIGN-package-models.md, 6.12; docs/MODELING.md, "Bulk adoption: adopt_rules:"): a human
// decides once, for a rule of layer 1 and a scope, and the command `make model` expands it into rows (`origin: generated`, `decision: <id>`) in
// `<lang>-<package>.adopt.gen.yaml`. This file only reads and checks the decision, as `rejected:`: the analysis does not read it, and it changes
// no row. The expansion is not here.
type AdoptRule struct {
	// ID is unique in the file. A generated row names it in `decision:`.
	ID string `yaml:"id" json:"id"`
	// Kind is sink or source.
	Kind string `yaml:"kind" json:"kind"`
	// Rule is the stable id of a rule of layer 1 (AdoptRuleIDs).
	Rule string `yaml:"rule" json:"rule"`
	// Words are the words of the vocabulary of the rule that the decision takes (the command checks them against the table).
	Words []string `yaml:"words" json:"words,omitempty"`
	// ArgTypes are the full names of the types of the arguments that the rule sink.arg-type takes (one type each; no pattern).
	ArgTypes []string `yaml:"arg_types" json:"arg_types,omitempty"`
	// Category is the category of the rows (sink only).
	Category string `yaml:"category" json:"category,omitempty"`
	// Labels are the labels of the rows (source only).
	Labels []string `yaml:"labels" json:"labels,omitempty"`
	// Scope says which candidates the decision is about. Only what can be chosen without a pattern over symbols.
	Scope AdoptScope `yaml:"scope" json:"scope"`
	// Why is the reason, as a fact. Required.
	Why string `yaml:"why" json:"why"`
	// Covers is how many rows the decision expanded into when it was written (the command stops, exit 70, when there are more). Required.
	Covers *int `yaml:"covers" json:"covers,omitempty"`
	// Representatives are symbols of the generated rows that have a test cell.
	Representatives []string `yaml:"representatives" json:"representatives,omitempty"`
	// DiffChecked is the record of the last run of the diff test (bench/model-diff) over the generated rows. Optional until verified.
	DiffChecked *AdoptDiffChecked `yaml:"diff_checked" json:"diff_checked,omitempty"`
	// ExtraOK is the one reason for the EXTRA rows of the diff test of the whole decision (a row of a generated file has no comment of its own).
	ExtraOK string `yaml:"extra_ok" json:"extra_ok,omitempty"`
	// ExtraCovers is how many EXTRA rows ExtraOK covered when it was written. Required with ExtraOK; ExtraWarning says when there are more.
	ExtraCovers *int `yaml:"extra_covers" json:"extra_covers,omitempty"`

	// Line is the line of the entry in the file (for the messages). 0 when not known.
	Line int `yaml:"-" json:"-"`
}

// AdoptScope is the scope of a decision. Several keys are all required together (AND).
type AdoptScope struct {
	// Package is the exact path of the package (Go: `/v*` for the major; ADR-110).
	Package string `yaml:"package" json:"package"`
	// ReceiverTypes is a list of full names of types (exact).
	ReceiverTypes []string `yaml:"receiver_types" json:"receiver_types,omitempty"`
	// ReceiverSuffix and ReceiverPrefix are the end or the start of the name of the type (one of them at most).
	ReceiverSuffix string `yaml:"receiver_suffix" json:"receiver_suffix,omitempty"`
	ReceiverPrefix string `yaml:"receiver_prefix" json:"receiver_prefix,omitempty"`
	// Functions is a list of names of functions (exact).
	Functions []string `yaml:"functions" json:"functions,omitempty"`
}

// AdoptDiffChecked records a run of the diff test: the version of the package, and how many generated rows it checked (MISS 0).
type AdoptDiffChecked struct {
	Version string `yaml:"version" json:"version"`
	Count   int    `yaml:"count" json:"count"`
}

// Rules of layer 1 that a decision can name. The table is `bench/model-author/rules.go` (a test there keeps the two lists equal); an id is never
// renamed. Sink rules and source rules; the sanitizer and calls rules of the table are not adoptable.
const (
	RuleSinkArgWord        = "sink.arg-word"
	RuleSinkNameOfResource = "sink.name-of-resource"
	RuleSinkFileName       = "sink.file-name"
	RuleSinkBytesWord      = "sink.bytes-word"
	RuleSinkStructField    = "sink.struct-field"
	RuleSinkFormatValues   = "sink.format-values"
	RuleSinkArgType        = "sink.arg-type"
	RuleSourceFieldWord    = "source.field-word"
	RuleSourceResultWord   = "source.result-word"
)

// AdoptRuleIDs are the ids a decision can name, in the order of the table.
var AdoptRuleIDs = []string{
	RuleSinkArgWord, RuleSinkNameOfResource, RuleSinkFileName, RuleSinkBytesWord, RuleSinkStructField, RuleSinkFormatValues, RuleSinkArgType,
	RuleSourceFieldWord, RuleSourceResultWord,
}

// AdoptKinds are the values of kind of a decision.
var AdoptKinds = []string{"sink", "source"}

// adoptRuleUsesWords: the rules whose decision says `words` (the others have a fixed vocabulary).
var adoptRuleUsesWords = map[string]bool{
	RuleSinkArgWord: true, RuleSinkBytesWord: true, RuleSinkStructField: true, RuleSourceFieldWord: true, RuleSourceResultWord: true,
}

var (
	adoptIDRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	adoptWordRe = regexp.MustCompile(`^[a-z0-9]+$`)
	adoptCatRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// noGlob is an error when s has a pattern in it. `/v*` (the major of a Go path) is the only one allowed.
func noGlob(s, what string) error {
	if strings.ContainsAny(strings.ReplaceAll(s, "/v*", "/v"), "*?[]{}") || strings.TrimSpace(s) != s || s == "" {
		return fmt.Errorf("%s: %q is one exact name (a pattern over symbols is not allowed; `/v*` for a major is)", what, s)
	}
	return CheckMajorWildcard(s, what)
}

func (a AdoptRule) label(i int) string {
	s := fmt.Sprintf("adopt_rules[%d]", i)
	if a.ID != "" {
		s += fmt.Sprintf(" (%s)", a.ID)
	}
	if a.Line > 0 {
		s += fmt.Sprintf(" (line %d)", a.Line)
	}
	return s
}

func (rs *RuleSet) validateAdoptRules() error {
	seen := map[string]bool{}
	for i, a := range rs.AdoptRules {
		what := a.label(i)
		if a.ID == "" || !adoptIDRe.MatchString(a.ID) {
			return fmt.Errorf("%s: id is required (lower case, digits and -; got %q)", what, a.ID)
		}
		if seen[a.ID] {
			return fmt.Errorf("%s: id %q is used twice in the file", what, a.ID)
		}
		seen[a.ID] = true
		if a.Kind != "sink" && a.Kind != "source" {
			return fmt.Errorf("%s: kind is required and is sink or source (got %q)", what, a.Kind)
		}
		known := false
		for _, id := range AdoptRuleIDs {
			known = known || id == a.Rule
		}
		if !known {
			return fmt.Errorf("%s: unknown rule %q (one of %s)", what, a.Rule, strings.Join(AdoptRuleIDs, ", "))
		}
		if !strings.HasPrefix(a.Rule, a.Kind+".") {
			return fmt.Errorf("%s: rule %s is not a rule of kind %s", what, a.Rule, a.Kind)
		}
		if strings.TrimSpace(a.Why) == "" {
			return fmt.Errorf("%s: why is required", what)
		}
		if a.Covers == nil || *a.Covers < 1 {
			return fmt.Errorf("%s: covers is required (how many rows the decision expands into now; at least 1)", what)
		}
		if err := a.validateShape(what); err != nil {
			return err
		}
		if err := a.Scope.validate(what + ".scope"); err != nil {
			return err
		}
		if a.Rule == RuleSinkArgType && len(a.Scope.ReceiverTypes) == 0 && a.Scope.ReceiverSuffix == "" && a.Scope.ReceiverPrefix == "" && len(a.Scope.Functions) == 0 {
			return fmt.Errorf("%s: %s needs a scope narrower than a package: receiver_types, receiver_suffix, receiver_prefix or functions", what, a.Rule)
		}
		reps := map[string]bool{}
		for _, r := range a.Representatives {
			if err := checkSymbol(r, what+".representatives"); err != nil {
				return err
			}
			if reps[r] {
				return fmt.Errorf("%s: representative %s is listed twice", what, r)
			}
			reps[r] = true
		}
		if d := a.DiffChecked; d != nil {
			if strings.TrimSpace(d.Version) == "" || d.Count < 1 {
				return fmt.Errorf("%s: diff_checked needs version and count (at least 1)", what)
			}
		}
		if (a.ExtraOK != "") != (a.ExtraCovers != nil) {
			return fmt.Errorf("%s: extra_ok (one reason) and extra_covers (how many EXTRA rows it covers now) go together", what)
		}
		if a.ExtraOK != "" && strings.TrimSpace(a.ExtraOK) == "" || a.ExtraCovers != nil && *a.ExtraCovers < 0 {
			return fmt.Errorf("%s: extra_ok is a reason and extra_covers is not negative", what)
		}
	}
	return nil
}

// validateShape checks the keys that the rule and the kind take.
func (a AdoptRule) validateShape(what string) error {
	if adoptRuleUsesWords[a.Rule] {
		if len(a.Words) == 0 {
			return fmt.Errorf("%s: words is required with %s (the words of the vocabulary that the decision takes)", what, a.Rule)
		}
		seen := map[string]bool{}
		for _, w := range a.Words {
			if !adoptWordRe.MatchString(w) {
				return fmt.Errorf("%s: word %q is one lower-case word", what, w)
			}
			if seen[w] {
				return fmt.Errorf("%s: word %q is listed twice", what, w)
			}
			seen[w] = true
		}
	} else if len(a.Words) > 0 {
		return fmt.Errorf("%s: words is not for %s", what, a.Rule)
	}
	if a.Rule == RuleSinkArgType {
		if len(a.ArgTypes) == 0 {
			return fmt.Errorf("%s: arg_types is required with %s (the full names of the types)", what, a.Rule)
		}
		for _, t := range a.ArgTypes {
			if t == "" || strings.ContainsAny(t, "*? \t,") {
				return fmt.Errorf("%s: arg_types has the full name of one type each (no pattern, no list): %q", what, t)
			}
		}
	} else if len(a.ArgTypes) > 0 {
		return fmt.Errorf("%s: arg_types is for %s", what, RuleSinkArgType)
	}
	if a.Kind == "sink" {
		if !adoptCatRe.MatchString(a.Category) {
			return fmt.Errorf("%s: category is required for a sink (got %q)", what, a.Category)
		}
		if len(a.Labels) > 0 {
			return fmt.Errorf("%s: labels is for a source", what)
		}
		return nil
	}
	if a.Category != "" {
		return fmt.Errorf("%s: category is for a sink", what)
	}
	if len(a.Labels) == 0 {
		return fmt.Errorf("%s: labels is required for a source", what)
	}
	return nil
}

func (s AdoptScope) validate(what string) error {
	if s.Package == "" {
		return fmt.Errorf("%s: package is required (the exact path)", what)
	}
	if err := noGlob(s.Package, what+".package"); err != nil {
		return err
	}
	for _, t := range s.ReceiverTypes {
		if err := noGlob(t, what+".receiver_types"); err != nil {
			return err
		}
	}
	for _, f := range s.Functions {
		if err := noGlob(f, what+".functions"); err != nil {
			return err
		}
	}
	if s.ReceiverSuffix != "" && s.ReceiverPrefix != "" {
		return fmt.Errorf("%s: receiver_suffix and receiver_prefix: one of them", what)
	}
	for _, p := range []string{s.ReceiverSuffix, s.ReceiverPrefix} {
		if strings.ContainsAny(p, "*?[]{}/. \t()") {
			return fmt.Errorf("%s: a prefix or a suffix of a type name is plain letters (got %q)", what, p)
		}
	}
	return nil
}

// ExtraWarning is a warning when the diff test has more EXTRA rows now than extra_covers recorded: a new API entered the reason unseen. "" when
// there is nothing to say (no extra_ok, or not more).
func (a AdoptRule) ExtraWarning(now int) string {
	if a.ExtraCovers == nil || now <= *a.ExtraCovers {
		return ""
	}
	return fmt.Sprintf("adopt_rules %s: %d EXTRA rows, extra_ok covered %d: look at the new ones, then update extra_covers", a.ID, now, *a.ExtraCovers)
}

// ---- generated rows and the files they live in ----

// AdoptGenSuffix ends the name of the file of the rows of the decisions of a package (`go-gcp.adopt.gen.yaml` for `go-gcp.yaml`).
const AdoptGenSuffix = ".adopt.gen.yaml"

// adoptStem is the name of the package's file without the suffix ("" when name is not a file of the rows of decisions).
func adoptStem(name string) string {
	name = strings.TrimPrefix(name, "builtin:")
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if !strings.HasSuffix(name, AdoptGenSuffix) {
		return ""
	}
	return strings.TrimSuffix(name, AdoptGenSuffix)
}

// IsAdoptGen says whether a file name (or the origin of a rule set) is that of a file of generated rows of decisions.
func IsAdoptGen(name string) bool { return adoptStem(name) != "" }

func handStem(name string) string {
	name = strings.TrimPrefix(name, "builtin:")
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if !strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".gen.yaml") {
		return ""
	}
	return strings.TrimSuffix(name, ".yaml")
}

type adoptRow struct {
	where string
	kind  string // sink | source
	p     Provenance
}

func (rs *RuleSet) adoptRows() (rows []adoptRow, others int) {
	for i, s := range rs.Sinks {
		rows = append(rows, adoptRow{fmt.Sprintf("sinks[%d] (%s)", i, s.Symbol), "sink", s.Provenance})
	}
	for i, s := range rs.Sources {
		rows = append(rows, adoptRow{fmt.Sprintf("sources[%d] (%s)", i, s.Symbol), "source", s.Provenance})
	}
	return rows, len(rs.Sanitizers) + len(rs.Propagators) + len(rs.Globals)
}

// CheckAdoption checks the rule sets that are loaded together (the built-in files, or the files of one package): a generated row is only in a
// file `<name>.adopt.gen.yaml`, carries `decision:` (checked for the form when the file is read), and the decision exists in `adopt_rules:` of the
// hand file `<name>.yaml` of the same package, with the kind of the row. The file of generated rows has sinks and sources only, all of origin
// generated, and no `adopt_rules:` of its own. A hand file is found by the name of the origin of the rule set.
func CheckAdoption(sets ...*RuleSet) error {
	hand := map[string]*RuleSet{}
	for _, rs := range sets {
		if st := handStem(rs.Origin); st != "" {
			hand[st] = rs
		}
	}
	for _, rs := range sets {
		stem := adoptStem(rs.Origin)
		rows, others := rs.adoptRows()
		if stem == "" {
			for _, r := range rows {
				if r.p.Origin == OriginGenerated {
					return fmt.Errorf("%s: %s: origin generated is for the rows of a file named *%s", rs.Origin, r.where, AdoptGenSuffix)
				}
			}
			continue
		}
		if len(rs.AdoptRules) > 0 {
			return fmt.Errorf("%s: adopt_rules: is written in the hand file %s.yaml, not in the file of the generated rows", rs.Origin, stem)
		}
		if others > 0 {
			return fmt.Errorf("%s: a file of generated rows of decisions has sinks and sources only", rs.Origin)
		}
		h := hand[stem]
		if h == nil {
			return fmt.Errorf("%s: the hand file %s.yaml of the package is not loaded with it: the decisions of its rows are not there", rs.Origin, stem)
		}
		decisions := map[string]AdoptRule{}
		for _, a := range h.AdoptRules {
			decisions[a.ID] = a
		}
		for _, r := range rows {
			if r.p.Origin != OriginGenerated {
				return fmt.Errorf("%s: %s: a row of this file is origin generated (got %q)", rs.Origin, r.where, r.p.Origin)
			}
			for _, id := range r.p.Decision {
				a, ok := decisions[id]
				if !ok {
					return fmt.Errorf("%s: %s: decision %q is not in adopt_rules of %s.yaml", rs.Origin, r.where, id, stem)
				}
				if a.Kind != r.kind {
					return fmt.Errorf("%s: %s: the row is a %s, decision %q is a %s", rs.Origin, r.where, r.kind, a.ID, a.Kind)
				}
			}
		}
	}
	return nil
}

// setAdoptLines gives the entries of adopt_rules the line they are written at.
func setAdoptLines(data []byte, rules []AdoptRule) {
	if len(rules) == 0 {
		return
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return
	}
	m := doc.Content[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "adopt_rules" && m.Content[i+1].Kind == yaml.SequenceNode {
			for j, n := range m.Content[i+1].Content {
				if j < len(rules) {
					rules[j].Line = n.Line
				}
			}
		}
	}
}

// DecisionIDs is the `decision:` of a generated row: one id (`decision: a`) or a list (`decision: [a, b]`) for a row that decisions that overlap
// were merged into. A list is sorted and has no duplicates; a single id is the same as a list of one.
type DecisionIDs []string

// UnmarshalYAML reads a string or a list of strings.
func (d *DecisionIDs) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		var s string
		if err := n.Decode(&s); err != nil {
			return err
		}
		*d = DecisionIDs{s}
	case yaml.SequenceNode:
		var l []string
		if err := n.Decode(&l); err != nil {
			return err
		}
		*d = DecisionIDs(l)
	default:
		return fmt.Errorf("line %d: decision is an id or a list of ids", n.Line)
	}
	return nil
}

// MarshalYAML writes one id as a string and several as a list.
func (d DecisionIDs) MarshalYAML() (any, error) {
	if len(d) == 1 {
		return d[0], nil
	}
	return []string(d), nil
}

func (d DecisionIDs) validate(what string) error {
	if len(d) == 0 {
		return fmt.Errorf("%s: a generated row needs decision: the id of its decision in adopt_rules (or a list of ids)", what)
	}
	for i, id := range d {
		if !adoptIDRe.MatchString(id) {
			return fmt.Errorf("%s: decision %q is not an id of adopt_rules", what, id)
		}
		if i > 0 && d[i-1] >= id {
			return fmt.Errorf("%s: decision is a list of ids in dictionary order, with no duplicates (%q is after %q)", what, id, d[i-1])
		}
	}
	return nil
}
