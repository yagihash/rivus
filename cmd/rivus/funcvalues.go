package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// funcValueRow is a line of the file that -func-values writes: a call of a function value, what the elements that the
// analysis carries say it may call (carried), and what the rest of the analysis has (valuecalls: what the calls of
// the function's callers say; extractor: what the extractor wrote).
type funcValueRow struct {
	Func       string   `json:"func"`
	Pos        string   `json:"pos"`
	Carried    []string `json:"carried"`
	ValueCalls []string `json:"valuecalls,omitempty"`
	Extractor  []string `json:"extractor,omitempty"`
	Limit      string   `json:"limit,omitempty"`
	// DroppedAtModel are the symbols of the calls, in the function that holds this call, that have a model and were given a
	// function value that the model says nothing of (the function value is lost there: FV-2, docs/DESIGN-callouts.md 5.6).
	DroppedAtModel []string `json:"dropped_at_model,omitempty"`
}

// writeFuncValues runs the analysis once more with the function values as elements and writes the table. Nothing that
// the analysis reports depends on it.
func writeFuncValues(path string, graph *sg.Program, l *loaded, st *sites.Result, rules *model.Rules, opts taint.Options) error {
	res, calls := runFuncValues(graph, l, st, rules, opts)
	carried := map[string][]string{}
	for _, c := range res.FuncCandidates {
		carried[c.Site.ID] = c.Candidates
	}
	dropped := map[string][]string{}
	for _, d := range res.FuncDropped {
		for _, fn := range d.Funcs {
			dropped[fn] = append(dropped[fn], d.Symbol)
		}
	}
	for _, syms := range dropped {
		sort.Strings(syms)
	}
	if n := len(res.FuncDropped); n > 0 {
		top := res.FuncDropped[:min(n, 10)]
		var parts []string
		for _, d := range top {
			parts = append(parts, fmt.Sprintf("%s (%d)", d.Symbol, d.Calls))
		}
		fmt.Fprintf(os.Stderr, "rivus: -func-values: %d symbol(s) with a model were given a function value they say nothing of; most: %s\n", n, strings.Join(parts, ", "))
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sort.Slice(calls, func(i, j int) bool { return calls[i].Site.ID < calls[j].Site.ID })
	enc := json.NewEncoder(f)
	for _, c := range calls {
		pos := c.Site.Position
		if err := enc.Encode(funcValueRow{
			Func: c.Site.Func, Pos: fmt.Sprintf("%s:%d:%d", pos.File, pos.Line, pos.Column),
			Carried: carried[c.Site.ID], ValueCalls: c.ValueCands, Extractor: c.Extractor, Limit: c.Limit,
			DroppedAtModel: dropped[c.Site.Func],
		}); err != nil {
			return err
		}
	}
	return nil
}

// runFuncValues is the run of the analysis of the function values (Options.FuncValues, with the synthetic sites): its
// result, and the calls of function values with the candidates that the rest of the analysis has for them.
func runFuncValues(graph *sg.Program, l *loaded, st *sites.Result, rules *model.Rules, opts taint.Options) (*taint.Result, []taint.FuncCall) {
	st2, calls := taint.FuncValueSites(graph, st)
	opts.FuncValues = true
	eng := taint.New(graph, l.texts, st2, rules, opts)
	eng.SetLoadErrors(l.loadErrors())
	return eng.Run(), calls
}
