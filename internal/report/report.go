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

	"github.com/yagihash/rivus/internal/frontend/gossa"
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
	LoadErrors   []gossa.LoadError    `json:"load_errors,omitempty"`
	// Expansions says what happened to the rules with match: implementations.
	Expansions []model.Expansion `json:"expansions,omitempty"`
	// DeterministicOnly is true when the rules decided by an LLM were left
	// out (IgnoredModels of them), so that the result depends on rivus and
	// on rules written or built in, and on nothing else.
	DeterministicOnly bool        `json:"deterministic_only,omitempty"`
	IgnoredModels     int         `json:"ignored_models,omitempty"`
	Stats             taint.Stats `json:"stats"`
	Summary           Summary     `json:"summary"`
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
	// TopSymbols lists the symbols behind the uncertainties rivus can
	// reduce itself: library calls without a model, and interface methods
	// without a known implementation. They show where a model helps most.
	TopSymbols map[string][]SymbolCount `json:"top_symbols"`
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
	if s.Program.Total > 0 {
		s.RivusShare = float64(s.Program.ByAttribution[taint.AttributableRivus]) / float64(s.Program.Total)
		s.DepthLimitShare = float64(s.Program.ByKind[taint.KindDepthLimit]) / float64(s.Program.Total)
	}
	return s
}

// Build assembles a report.
func Build(p *gossa.Program, st *sites.Result, res *taint.Result, rules *model.Rules) *Report {
	r := &Report{
		SchemaVersion: SchemaVersion,
		Sites:         st.Sites,
		Findings:      res.Findings,
		Uncertainties: res.Uncertainties,
		Suppressions:  res.Suppressions,
		LoadErrors:    p.LoadErrors,
		Stats:         res.Stats,
		Expansions:    rules.Expansions,

		DeterministicOnly: rules.DeterministicOnly,
		IgnoredModels:     rules.IgnoredModels,
	}
	// A rule that was not expanded because it had too many implementations
	// leaves flows through those implementations unseen: say so.
	for _, e := range rules.Expansions {
		if e.Limited {
			r.Uncertainties = append(r.Uncertainties, taint.NewUncertainty(taint.KindDynamicDispatch,
				fmt.Sprintf("expansion_limit: %d implementations exceed the limit of %d; the rule applies to the interface method only", e.Count, e.Max), e.Symbol))
		}
	}
	for _, pkg := range p.Initial {
		r.Packages = append(r.Packages, pkg.PkgPath)
	}
	sort.Strings(r.Packages)
	for _, rs := range rules.Sets {
		name := rs.Name
		if name == "" {
			name = rs.Origin
		}
		r.Rules = append(r.Rules, name)
	}
	r.Summary = summarize(r)
	return r
}

// Detail levels of the output.
const (
	DetailSummary = "summary" // findings without paths and snippets
	DetailSnippet = "snippet" // everything (default)
)

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
