package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

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
}

// writeFuncValues runs the analysis once more with the function values as elements and writes the table. Nothing that
// the analysis reports depends on it.
func writeFuncValues(path string, graph *sg.Program, l *loaded, st *sites.Result, rules *model.Rules, opts taint.Options) error {
	st2, calls := taint.FuncValueSites(graph, st)
	opts.FuncValues = true
	eng := taint.New(graph, l.texts, st2, rules, opts)
	eng.SetLoadErrors(l.loadErrors())
	res := eng.Run()
	carried := map[string][]string{}
	for _, c := range res.FuncCandidates {
		carried[c.Site.ID] = c.Candidates
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
		}); err != nil {
			return err
		}
	}
	return nil
}
