package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// Recall: of the rows of the models that are adopted (sinks and sources, by symbol), how many a rule of layer 1 finds. It says how well
// the table covers what people chose; a row that no rule finds is either a rule that is missing (a generic one is added to rules.go)
// or a hand row (said in the pull request).

// RecallResult is the numbers.
type RecallResult struct {
	SinkRows, SinkFound, SinkPosFound int
	SinkMissed                        []string
	SinkPosMissed                     []string // found by symbol, but a position of the row is not one of the candidate's
	SourceRows, SourceFound           int      // by symbol (a call_result, param or callback_param source) or by field
	SourceMissed                      []string
	SourceTypeRows                    int // param_type rows (a type, not a symbol): not counted
}

// ComputeRecall counts. A sink row is found when the symbol is a sink candidate; it is found at its positions when every `args` number
// of the row is a position of the candidate.
func ComputeRecall(api *API, f *Facts) RecallResult {
	var r RecallResult
	cand := map[string]Item{}
	for _, it := range api.Items {
		cand[it.Kind+" "+it.Symbol] = it
	}
	for _, s := range f.SinkRows {
		r.SinkRows++
		it, ok := cand[KindSink+" "+s.Symbol]
		if !ok {
			r.SinkMissed = append(r.SinkMissed, s.Symbol)
			continue
		}
		r.SinkFound++
		// a candidate at a field of an argument (arg0.path) covers a row that names the whole argument (arg0)
		pos := map[string]bool{}
		for _, p := range it.Positions {
			pos[p] = true
			if i := strings.Index(p, "."); i > 0 {
				pos[p[:i]] = true
			}
		}
		all := true
		for _, a := range s.Args {
			if !pos["arg"+strconv.Itoa(a)] {
				all = false
			}
		}
		for _, n := range s.Named {
			if !pos["named:"+n] {
				all = false
			}
		}
		for _, p := range s.Parts {
			if p.Named != "" && !pos["named:"+p.Named+"."+p.Field] && !pos["named:"+p.Named] {
				all = false
			}
			if p.Arg != nil && !pos["arg"+strconv.Itoa(*p.Arg)+"."+p.Field] && !pos["arg"+strconv.Itoa(*p.Arg)] {
				all = false
			}
		}
		if all {
			r.SinkPosFound++
		} else {
			r.SinkPosMissed = append(r.SinkPosMissed, fmt.Sprintf("%s (row %v%v%v, candidate %s)", s.Symbol, s.Args, s.Named, partsText(s), strings.Join(it.Positions, ",")))
		}
	}
	for _, s := range f.SourceRows {
		switch {
		case s.Symbol != "":
			r.SourceRows++
			if _, ok := cand[KindSource+" "+s.Symbol]; ok {
				r.SourceFound++
			} else {
				r.SourceMissed = append(r.SourceMissed, s.Symbol)
			}
		case s.Field != "":
			r.SourceRows++
			if _, ok := cand[KindSource+" "+s.Field]; ok {
				r.SourceFound++
			} else {
				r.SourceMissed = append(r.SourceMissed, s.Field)
			}
		default:
			r.SourceTypeRows++
		}
	}
	return r
}

// PrintRecall is the report mode.
func PrintRecall(w io.Writer, pkg string, l *loaded, api *API, f *Facts) {
	r := ComputeRecall(api, f)
	fmt.Fprintf(w, "recall of layer 1: %s %s\n", pkg, l.Version)
	fmt.Fprintf(w, "  layer 1 candidates: %d (layer 2: %d)\n", len(api.Items), len(api.Layer2))
	fmt.Fprintf(w, "  sink rows:   %d of %d in layer 1 (%d with all of their positions)\n", r.SinkFound, r.SinkRows, r.SinkPosFound)
	fmt.Fprintf(w, "  source rows: %d of %d in layer 1 (by symbol or field; %d param_type rows are not counted: they name a type)\n", r.SourceFound, r.SourceRows, r.SourceTypeRows)
	if len(r.SinkPosMissed) > 0 {
		fmt.Fprintf(w, "  sink rows found by symbol but not at every position:\n    %s\n", strings.Join(r.SinkPosMissed, "\n    "))
	}
	if len(r.SinkMissed) > 0 {
		fmt.Fprintf(w, "  sink rows that no rule finds:\n    %s\n", strings.Join(r.SinkMissed, "\n    "))
	}
	if len(r.SourceMissed) > 0 {
		fmt.Fprintf(w, "  source rows that no rule finds:\n    %s\n", strings.Join(r.SourceMissed, "\n    "))
	}
}

func partsText(s model.Sink) string {
	var out []string
	for _, p := range s.Parts {
		if p.Arg != nil {
			out = append(out, fmt.Sprintf("arg%d.%s", *p.Arg, p.Field))
		} else if p.Named != "" {
			out = append(out, "named:"+p.Named+"."+p.Field)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return " parts " + strings.Join(out, ",")
}
