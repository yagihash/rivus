package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yagihash/rivus/internal/callgraph"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// analyzeOpts are the flags common to the commands that run the analysis.
type analyzeOpts struct {
	common
	k int
}

func (o *analyzeOpts) register(fs *flag.FlagSet) {
	o.common.register(fs)
	fs.IntVar(&o.k, "k", taint.DefaultOptions().K, "maximum access path depth")
}

// analyze runs the whole pipeline. extra rule sets are added to the ones
// loaded from the flags.
func (o *analyzeOpts) analyze(patterns []string, extra ...*model.RuleSet) (*report.Report, error) {
	rules, err := o.loadRules()
	if err != nil {
		return nil, err
	}
	if len(extra) > 0 {
		rules = model.Merge(append(append([]*model.RuleSet{}, rules.Sets...), extra...)...)
	}
	prog, err := o.load(patterns)
	if err != nil {
		return nil, err
	}
	st := sites.Enumerate(prog, rules)
	opts := taint.DefaultOptions()
	opts.K = o.k
	eng := taint.New(prog, st, rules, callgraph.BuildGo(prog), opts)
	return report.Build(prog, st, eng.Run(), rules), nil
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	detail := fs.String("detail", report.DetailSnippet, "level of detail: summary or snippet")
	withSites := fs.Bool("sites", false, "also list the sites")
	fs.Parse(args)

	rep, err := o.analyze(fs.Args())
	if err != nil {
		return err
	}
	if !*withSites {
		rep.Sites = nil
	}
	rep, err = rep.WithDetail(*detail)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, rep)
}

func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	out := fs.String("o", "", "file to write (default: .rivus/index.json below -C)")
	fs.Parse(args)

	rep, err := o.analyze(fs.Args())
	if err != nil {
		return err
	}
	path := *out
	if path == "" {
		path = filepath.Join(o.dir, ".rivus", "index.json")
	}
	if err := rep.Save(path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rivus: wrote %s (%d sites, %d findings, %d uncertainties, %d ms)\n",
		path, len(rep.Sites), len(rep.Findings), len(rep.Uncertainties), rep.Stats.DurationMilli)
	return nil
}

func runQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	sink := fs.String("sink", "", "which sources can reach this sink symbol")
	source := fs.String("source", "", "which sinks can be reached from this source symbol")
	site := fs.String("site", "", "ID of a site (see \"rivus sites\")")
	index := fs.String("index", "", "answer from a saved index instead of analysing the source")
	detail := fs.String("detail", report.DetailSnippet, "level of detail: summary or snippet")
	fs.Parse(args)

	q := report.Query{Sink: *sink, Source: *source, Site: *site}
	var rep *report.Report
	var err error
	if *index != "" {
		rep, err = report.Load(*index)
	} else {
		rep, err = o.analyze(fs.Args(), adHocRules(q)...)
	}
	if err != nil {
		return err
	}
	res, err := rep.Run(q)
	if err != nil {
		return err
	}
	if *detail != "" && *detail != report.DetailSnippet {
		d, err := (&report.Report{SchemaVersion: report.SchemaVersion, Findings: res.Findings}).WithDetail(*detail)
		if err != nil {
			return err
		}
		res.Findings = d.Findings
	}
	return writeJSON(os.Stdout, res)
}

// adHocRules lets a one-sided query name a symbol that no rule mentions: a
// sink query makes the symbol a sink for any label, a source query makes
// its result a source.
func adHocRules(q report.Query) []*model.RuleSet {
	rs := &model.RuleSet{Name: "query", Origin: "query"}
	if q.Sink != "" {
		rs.Sinks = append(rs.Sinks, model.Sink{Symbol: q.Sink, Category: "query"})
	}
	if q.Source != "" {
		rs.Sources = append(rs.Sources, model.Source{Kind: model.KindCallResult, Symbol: q.Source, Labels: []string{"queried"}})
	}
	if len(rs.Sinks) == 0 && len(rs.Sources) == 0 {
		return nil
	}
	return []*model.RuleSet{rs}
}
