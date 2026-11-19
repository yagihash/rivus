package report

import (
	"fmt"
	"sort"

	"github.com/yagihash/rivus/internal/taint"
	"github.com/yagihash/rivus/internal/vcs"
)

// QuerySchemaVersion identifies the format of QueryResult.
const QuerySchemaVersion = "rivus.query/1"

// Query is a one-sided question (ADR-004 makes it possible: the summaries of
// all functions already exist, so no seed is needed).
type Query struct {
	// Sink asks which sources can reach sink sites of this symbol.
	Sink string `json:"sink,omitempty"`
	// Source asks which sinks the source sites of this symbol can reach.
	Source string `json:"source,omitempty"`
	// Site asks about one site, by ID, in whichever role it has.
	Site string `json:"site,omitempty"`
}

// Group collects the findings that share one end.
type Group struct {
	// Site is the other end: the source of a sink query, the sink of a
	// source query.
	Site     taint.SiteRef `json:"site"`
	Findings []string      `json:"findings"`
	Labels   []string      `json:"labels"`
	// Uncertain counts the findings whose path has uncertainty.
	Uncertain int `json:"uncertain"`
}

// QueryResult answers a Query.
type QueryResult struct {
	SchemaVersion string `json:"schema_version"`
	Query         Query  `json:"query"`
	// Target is the revision of the code the answer is about.
	Target *vcs.Target `json:"target,omitempty"`
	// Matched are the sites the question is about.
	Matched  []taint.SiteRef  `json:"matched"`
	Groups   []Group          `json:"groups"`
	Findings []*taint.Finding `json:"findings"`
	Stats    taint.Stats      `json:"stats"`
}

// Run answers the query from the report.
func (r *Report) Run(q Query) (*QueryResult, error) {
	n := 0
	for _, s := range []string{q.Sink, q.Source, q.Site} {
		if s != "" {
			n++
		}
	}
	if n != 1 {
		return nil, fmt.Errorf("exactly one of sink, source and site is required")
	}
	out := &QueryResult{SchemaVersion: QuerySchemaVersion, Query: q, Target: r.Target, Stats: r.Stats, Matched: []taint.SiteRef{}, Groups: []Group{}, Findings: []*taint.Finding{}}

	matched := map[string]bool{}
	note := func(s taint.SiteRef) {
		if !matched[s.ID] {
			matched[s.ID] = true
			out.Matched = append(out.Matched, s)
		}
	}
	groups := map[string]*Group{}
	add := func(f *taint.Finding, other taint.SiteRef) {
		g := groups[other.ID]
		if g == nil {
			g = &Group{Site: other}
			groups[other.ID] = g
		}
		g.Findings = append(g.Findings, f.ID)
		g.Labels = union(g.Labels, f.Labels)
		if len(f.Uncertainties) > 0 {
			g.Uncertain++
		}
	}

	for _, f := range r.Findings {
		switch {
		case q.Sink != "" && f.Sink.Symbol == q.Sink:
			note(f.Sink)
			add(f, f.Source)
		case q.Source != "" && f.Source.Symbol == q.Source:
			note(f.Source)
			add(f, f.Sink)
		case q.Site != "" && f.Sink.ID == q.Site:
			note(f.Sink)
			add(f, f.Source)
		case q.Site != "" && f.Source.ID == q.Site:
			note(f.Source)
			add(f, f.Sink)
		default:
			continue
		}
		out.Findings = append(out.Findings, f)
	}
	// A site that matches but has no finding is still worth reporting: the
	// question was answered, and the answer is "nothing reaches it".
	for _, s := range r.Sites {
		if (q.Sink != "" && s.Role == "sink" && s.Symbol == q.Sink) ||
			(q.Source != "" && s.Role == "source" && s.Symbol == q.Source) ||
			(q.Site != "" && s.ID == q.Site) {
			note(ref(s))
		}
	}
	for _, g := range groups {
		sort.Strings(g.Findings)
		out.Groups = append(out.Groups, *g)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i].Site.Position, out.Groups[j].Site.Position
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Offset != b.Offset {
			return a.Offset < b.Offset
		}
		return out.Groups[i].Site.ID < out.Groups[j].Site.ID
	})
	sort.Slice(out.Matched, func(i, j int) bool {
		a, b := out.Matched[i].Position, out.Matched[j].Position
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Offset != b.Offset {
			return a.Offset < b.Offset
		}
		return out.Matched[i].ID < out.Matched[j].ID
	})
	return out, nil
}

func union(a, b []string) []string {
	set := map[string]bool{}
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
