// Package report builds, saves and queries the result of an analysis.
//
// The structured JSON is the primary output of rivus (ADR-009). Queries run
// on a Report, not on the program, so a saved report can answer questions
// without loading the source again (ADR-027).
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
	"github.com/yagihash/rivus/internal/vcs"
)

// SchemaVersion identifies the format of Report.
const SchemaVersion = "rivus.result/1"

// Report is the result of an analysis.
type Report struct {
	SchemaVersion string   `json:"schema_version"`
	Rules         []string `json:"rules"`
	// Target is the revision of the code that was analysed (absent outside a
	// git working tree).
	Target *vcs.Target `json:"target,omitempty"`
	// Packages lists the packages that were analysed, so that a site that
	// is missing can be told from a package that was not looked at.
	Packages      []string             `json:"packages"`
	Sites         []*sites.Site        `json:"sites,omitempty"`
	Findings      []*taint.Finding     `json:"findings"`
	Uncertainties []*taint.Uncertainty `json:"uncertainties"`
	// Suppressions are the places where a model decided by an LLM removed
	// taint: findings that exist only without the model.
	Suppressions []*taint.Suppression `json:"suppressions,omitempty"`
	LoadErrors   []ir.LoadError       `json:"load_errors,omitempty"`
	// Expansions says what happened to the rules with match: implementations.
	Expansions []model.Expansion `json:"expansions,omitempty"`
	// ProjectRules are the rule files that the analysed directory brought (<dir>/.rivus/rules), with what
	// each of them can remove from the result: a sanitizer, a model that says that nothing flows. They are
	// read without a flag, and they are written by whoever wrote the code under analysis.
	ProjectRules []ProjectRules `json:"project_rules,omitempty"`
	// DeterministicOnly is true when the rules decided by an LLM were left
	// out (IgnoredModels of them), so that the result depends on rivus and
	// on rules written or built in, and on nothing else.
	DeterministicOnly bool        `json:"deterministic_only,omitempty"`
	IgnoredModels     int         `json:"ignored_models,omitempty"`
	Stats             taint.Stats `json:"stats"`
	Summary           Summary     `json:"summary"`
}

// ProjectRules counts what a rule file of the analysed project holds.
type ProjectRules struct {
	File        string `json:"file"`
	Sources     int    `json:"sources"`
	Sinks       int    `json:"sinks"`
	Sanitizers  int    `json:"sanitizers"`
	Propagators int    `json:"propagators"`
	// PureModels are the models that say that nothing flows through the function.
	PureModels int `json:"pure_models"`
	// DropAll are the sanitizers that remove every label ("*").
	DropAll int `json:"drop_all_sanitizers"`
}

// projectRules summarizes the rule files that were found in the analysed directory.
func projectRules(rules *model.Rules) []ProjectRules {
	var out []ProjectRules
	for _, rs := range rules.Sets {
		if !rs.Project {
			continue
		}
		p := ProjectRules{File: rs.Origin, Sources: len(rs.Sources), Sinks: len(rs.Sinks), Sanitizers: len(rs.Sanitizers), Propagators: len(rs.Propagators)}
		for _, pr := range rs.Propagators {
			if pr.Pure {
				p.PureModels++
			}
		}
		for _, s := range rs.Sanitizers {
			for _, d := range s.Drops {
				if d == "*" {
					p.DropAll++
					break
				}
			}
		}
		out = append(out, p)
	}
	return out
}

// Summary counts the uncertainties. It is the feedback loop of ADR-008: the
// share of uncertainty attributable to rivus says where to invest, and the
// share of depth_limit is the number that should go down.
type Summary struct {
	Findings int `json:"findings"`
	// Program counts the uncertainties of the program, by kind and by
	// attributable_to.
	Program Counts `json:"program"`
	// OnFindings counts, by kind, the findings whose path has such an
	// uncertainty (a finding is counted once per kind).
	OnFindings map[string]int `json:"on_findings"`
	// RivusShare is the share of the program-level uncertainties
	// attributable to rivus, DepthLimitShare the share of depth_limit.
	RivusShare      float64 `json:"rivus_share"`
	DepthLimitShare float64 `json:"depth_limit_share"`
	// DispatchCandidates tells how many candidates the calls through
	// interfaces and function values have (0, 1, 2-5, 6-20, 21+). A call with
	// no candidate needs a model; many candidates call for a more precise
	// call graph.
	DispatchCandidates map[string]int `json:"dispatch_candidates"`
	// DispatchNarrowed counts the calls through Fn::call (Rust) whose candidates were narrowed to the
	// closures that flow to the receiver: how many, and the candidates they had before and after (summed).
	// The numbers of DispatchCandidates are those after. Absent when none was narrowed.
	DispatchNarrowed *DispatchNarrowed `json:"dispatch_narrowed,omitempty"`
	// TopSymbols lists the symbols behind the uncertainties rivus can
	// reduce itself: library calls without a model, and interface methods
	// without a known implementation. They show where a model helps most.
	TopSymbols map[string][]SymbolCount `json:"top_symbols"`
	// LoadDependent is true when the result depends on the load of the machine: a time limit
	// (-func-timeout, -timeout) stopped the analysis of a function or of the run, and what a clock
	// cuts is not the same twice. LoadDependentNote says it, with the numbers (stats has them).
	LoadDependent     bool   `json:"load_dependent,omitempty"`
	LoadDependentNote string `json:"load_dependent_note,omitempty"`
	// Unread is what was read and not built: the packages with a type error, and those that import
	// them (go/ssa builds nothing for them). Absent when every package was built. They stay in
	// packages; the functions they hold are not in the analysis, and no finding can come from them.
	Unread *Unread `json:"unread,omitempty"`
}

// Unread counts the packages that were read and not built.
type Unread struct {
	// Packages is how many, Functions the functions (with a body) that they hold.
	Packages  int `json:"packages"`
	Functions int `json:"functions"`
	// Top lists the packages with the most functions (at most 10).
	Top []UnreadPackage `json:"top"`
}

// UnreadPackage is one package that was read and not built.
type UnreadPackage struct {
	Package   string `json:"package"`
	Functions int    `json:"functions"`
	Why       string `json:"why"`
}

func unreadOf(errs []ir.LoadError) *Unread {
	var u *Unread
	for _, e := range errs {
		if !e.Unread {
			continue
		}
		if u == nil {
			u = &Unread{Top: []UnreadPackage{}}
		}
		u.Packages++
		u.Functions += e.Functions
		u.Top = append(u.Top, UnreadPackage{Package: e.Package, Functions: e.Functions, Why: e.Message})
	}
	if u == nil {
		return nil
	}
	sort.SliceStable(u.Top, func(i, j int) bool {
		a, b := u.Top[i], u.Top[j]
		if a.Functions != b.Functions {
			return a.Functions > b.Functions
		}
		return a.Package < b.Package
	})
	if len(u.Top) > 10 {
		u.Top = u.Top[:10]
	}
	return u
}

// SymbolCount is a symbol and how often it was uncertain.
type SymbolCount struct {
	Symbol string `json:"symbol"`
	Count  int    `json:"count"`
}

// Counts is a tally of uncertainties.
type Counts struct {
	Total         int            `json:"total"`
	ByKind        map[string]int `json:"by_kind"`
	ByAttribution map[string]int `json:"by_attribution"`
}

func bucket(n int) string {
	switch {
	case n == 0:
		return "0"
	case n == 1:
		return "1"
	case n <= 5:
		return "2-5"
	case n <= 20:
		return "6-20"
	}
	return "21+"
}

// DispatchNarrowed is what the narrowing of the candidates of Fn::call did (Summary.DispatchNarrowed).
type DispatchNarrowed struct {
	Calls  int `json:"calls"`
	Before int `json:"candidates_before"`
	After  int `json:"candidates_after"`
}

func summarize(r *Report) Summary {
	s := Summary{Findings: len(r.Findings), OnFindings: map[string]int{},
		Program:            Counts{ByKind: map[string]int{}, ByAttribution: map[string]int{}},
		DispatchCandidates: map[string]int{}, TopSymbols: map[string][]SymbolCount{}}
	tally := map[string]map[string]int{}
	count := func(group, symbol string) {
		if tally[group] == nil {
			tally[group] = map[string]int{}
		}
		tally[group][symbol]++
	}
	for _, u := range r.Uncertainties {
		s.Program.Total++
		s.Program.ByKind[u.Kind]++
		s.Program.ByAttribution[u.AttributableTo]++
		switch {
		case u.Kind == taint.KindDynamicDispatch && !strings.HasPrefix(u.Detail, "expansion_limit"):
			n := len(u.Candidates)
			s.DispatchCandidates[bucket(n)]++
			if u.NarrowedFrom > 0 {
				if s.DispatchNarrowed == nil {
					s.DispatchNarrowed = &DispatchNarrowed{}
				}
				s.DispatchNarrowed.Calls++
				s.DispatchNarrowed.Before += u.NarrowedFrom
				s.DispatchNarrowed.After += n
			}
			if n == 0 {
				count("dynamic_dispatch_without_implementation", u.Symbol)
			}
		case u.Kind == taint.KindUnmodeledLibraryCall:
			count(u.Kind, u.Symbol)
		}
	}
	for group, m := range tally {
		for sym, n := range m {
			s.TopSymbols[group] = append(s.TopSymbols[group], SymbolCount{sym, n})
		}
		sort.Slice(s.TopSymbols[group], func(i, j int) bool {
			a, b := s.TopSymbols[group][i], s.TopSymbols[group][j]
			if a.Count != b.Count {
				return a.Count > b.Count
			}
			return a.Symbol < b.Symbol
		})
		if len(s.TopSymbols[group]) > 10 {
			s.TopSymbols[group] = s.TopSymbols[group][:10]
		}
	}
	for _, f := range r.Findings {
		seen := map[string]bool{}
		for _, u := range f.Uncertainties {
			if !seen[u.Kind] {
				seen[u.Kind] = true
				s.OnFindings[u.Kind]++
			}
		}
	}
	if st := r.Stats; st.LoadDependent {
		s.LoadDependent = true
		s.LoadDependentNote = fmt.Sprintf("this result depends on the load of the machine: %d function(s) were stopped by the time limit of one function (-func-timeout), %d by the time limit of the run (-timeout), and the run itself %s; the same run on a quieter machine may find more (the work limit alone, %d function(s), is the same everywhere)",
			st.FuncTimeouts, st.RunTimeouts, map[bool]string{true: "was stopped", false: "was not"}[st.TimedOut], st.BudgetExhausted)
	}
	s.Unread = unreadOf(r.LoadErrors)
	if s.Program.Total > 0 {
		s.RivusShare = float64(s.Program.ByAttribution[taint.AttributableRivus]) / float64(s.Program.Total)
		s.DepthLimitShare = float64(s.Program.ByKind[taint.KindDepthLimit]) / float64(s.Program.Total)
	}
	return s
}

// Build assembles a report.
func Build(packages []string, loadErrors []ir.LoadError, st *sites.Result, res *taint.Result, rules *model.Rules) *Report {
	r := &Report{
		SchemaVersion: SchemaVersion,
		Sites:         st.Sites,
		Findings:      res.Findings,
		Uncertainties: res.Uncertainties,
		Suppressions:  res.Suppressions,
		LoadErrors:    loadErrors,
		Stats:         res.Stats,
		Expansions:    rules.Expansions,
		ProjectRules:  projectRules(rules),

		DeterministicOnly: rules.DeterministicOnly,
		IgnoredModels:     rules.IgnoredModels,
	}
	// A rule that was not expanded because it had too many implementations
	// leaves flows through those implementations unseen: say so.
	for _, e := range rules.Expansions {
		if e.Missing {
			r.Uncertainties = append(r.Uncertainties, taint.NewUncertainty(taint.KindMissingSource, "generated_code_missing: "+e.Note, e.Symbol))
		}
		if e.Limited {
			r.Uncertainties = append(r.Uncertainties, taint.NewUncertainty(taint.KindDynamicDispatch,
				fmt.Sprintf("expansion_limit: %d implementations exceed the limit of %d; the rule applies to the interface method only", e.Count, e.Max), e.Symbol))
		}
	}
	r.Packages = append(r.Packages, packages...)
	sort.Strings(r.Packages)
	for _, rs := range rules.Sets {
		name := rs.Name
		if name == "" {
			name = rs.Origin
		}
		r.Rules = append(r.Rules, name)
	}
	// The arrays of the output are arrays when they are empty, so that a reader
	// does not have to take null for one (#15).
	if r.Findings == nil {
		r.Findings = []*taint.Finding{}
	}
	if r.Uncertainties == nil {
		r.Uncertainties = []*taint.Uncertainty{}
	}
	if r.Packages == nil {
		r.Packages = []string{}
	}
	if r.Rules == nil {
		r.Rules = []string{}
	}
	r.Summary = summarize(r)
	return r
}

// Detail levels of the output.
const (
	DetailSummary = "summary" // findings without paths and snippets
	DetailSnippet = "snippet" // everything (default)
)

// ValidDetail reports whether level is a detail level.
func ValidDetail(level string) error {
	switch level {
	case "", DetailSnippet, DetailSummary:
		return nil
	}
	return fmt.Errorf("unknown detail level %q (want %s or %s)", level, DetailSummary, DetailSnippet)
}

// WithDetail returns a copy of the report reduced to the detail level. It
// keeps the agent's context small when only an overview is needed.
func (r *Report) WithDetail(level string) (*Report, error) {
	switch level {
	case "", DetailSnippet:
		return r, nil
	case DetailSummary:
		out := *r
		out.Sites = nil
		out.Findings = make([]*taint.Finding, len(r.Findings))
		for i, f := range r.Findings {
			c := *f
			c.Paths = nil
			c.Source.Snippet = ""
			c.Sink.Snippet = ""
			c.Uncertainties = nil
			for _, u := range f.Uncertainties {
				cu := *u
				cu.Snippet = ""
				c.Uncertainties = append(c.Uncertainties, &cu)
			}
			out.Findings[i] = &c
		}
		return &out, nil
	}
	return nil, fmt.Errorf("unknown detail level %q (want %s or %s)", level, DetailSummary, DetailSnippet)
}

// Save writes the report as JSON.
func (r *Report) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads a saved report.
func Load(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if r.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%s: unsupported schema %q (want %q)", path, r.SchemaVersion, SchemaVersion)
	}
	return &r, nil
}
