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

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// SchemaVersion identifies the format of Report.
const SchemaVersion = "rivus.result/0"

// Report is the result of an analysis.
type Report struct {
	SchemaVersion string               `json:"schema_version"`
	Rules         []string             `json:"rules"`
	Sites         []*sites.Site        `json:"sites,omitempty"`
	Findings      []*taint.Finding     `json:"findings"`
	Uncertainties []*taint.Uncertainty `json:"uncertainties"`
	LoadErrors    []gossa.LoadError    `json:"load_errors,omitempty"`
	Stats         taint.Stats          `json:"stats"`
}

// Build assembles a report.
func Build(p *gossa.Program, st *sites.Result, res *taint.Result, rules *model.Rules) *Report {
	r := &Report{
		SchemaVersion: SchemaVersion,
		Sites:         st.Sites,
		Findings:      res.Findings,
		Uncertainties: res.Uncertainties,
		LoadErrors:    p.LoadErrors,
		Stats:         res.Stats,
	}
	for _, rs := range rules.Sets {
		name := rs.Name
		if name == "" {
			name = rs.Origin
		}
		r.Rules = append(r.Rules, name)
	}
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
