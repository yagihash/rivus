package taint

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/testutil"
)

// A construct that an extractor could not express leaves a trace in the
// result: without it the analysis answers "not tainted" about code it did not
// read (REVIEW-2026-11-19 CORE-1).
func TestUnsupportedInstructionIsRecorded(t *testing.T) {
	f := &ir.Func{
		ID:     "m#f",
		Symbol: "m#f",
		Locals: []ir.Local{{Name: "x"}},
		Blocks: []ir.Block{{
			ID: 0,
			Instrs: []ir.Instr{
				{Op: ir.OpUnsupported, Reason: "expression Whatever", Pos: ir.Pos{File: "a.dart", Line: 3}},
			},
			Term: ir.Term{Kind: ir.TermReturn},
		}},
		Roots: []ir.BlockID{0},
		Entry: true,
	}
	prog := &ir.Program{Funcs: []*ir.Func{f}}
	if err := prog.Validate(); err != nil {
		t.Fatal(err)
	}
	g, err := sg.Build(prog)
	if err != nil {
		t.Fatal(err)
	}
	rules := &model.Rules{}
	rules.Sinks = map[string][]model.Sink{}
	st := sites.Enumerate(prog, ir.NewStreamTexts(prog), rules)
	eng := New(g, ir.NewStreamTexts(prog), st, rules, DefaultOptions())
	res := eng.Run()
	for _, u := range res.Uncertainties {
		if u.Kind == KindUnsupported {
			if u.AttributableTo != AttributableRivus {
				t.Errorf("attributable_to = %q", u.AttributableTo)
			}
			if !strings.Contains(u.Detail, "expression Whatever") {
				t.Errorf("detail = %q", u.Detail)
			}
			return
		}
	}
	t.Errorf("no %s in %+v", KindUnsupported, res.Uncertainties)
}

// An access path that is cut at the depth k leaves a record (REVIEW-2026-11-19
// CORE-4): the flag was set and never read.
func TestCutAccessPathsAreRecorded(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/heap/...")
	rules := testutil.Rules(t)
	rules.Expand(prog.gossa, 0)
	st := prog.sites(rules)
	opts := DefaultOptions()
	opts.K = 1
	res := prog.engine(st, rules, opts).Run()
	for _, u := range res.Uncertainties {
		if u.Kind == KindDepthLimit && strings.HasPrefix(u.Detail, "path_limit:") {
			if len(u.Candidates) == 0 {
				t.Error("the functions are not listed")
			}
			return
		}
	}
	t.Error("no path_limit uncertainty with k = 1")
}

// A finding whose value went through a path that was cut carries a note on that hop (B-3): it is not "no uncertainty
// on the path". With a deep limit nothing is cut, and no finding has it.
func TestFindingThroughCutPathCarriesNote(t *testing.T) {
	run := func(k int) (cut, all int) {
		prog := loadIR(t, testutil.Testdata(), "./cases/heap/...")
		rules := testutil.Rules(t)
		rules.Expand(prog.gossa, 0)
		st := prog.sites(rules)
		opts := DefaultOptions()
		opts.K = k
		res := prog.engine(st, rules, opts).Run()
		for _, f := range res.Findings {
			all++
			for _, u := range f.Uncertainties {
				if u.Kind == KindAccessPathTruncated {
					cut++
					if u.AttributableTo != AttributableRivus || f.Confidence == ConfidenceClear || !strings.Contains(u.Detail, "cut at depth") || u.Func == "" {
						t.Errorf("k=%d: the note is not complete: %+v (confidence %s)", k, u, f.Confidence)
					}
					break
				}
			}
		}
		return
	}
	if cut, all := run(1); cut == 0 || all == 0 {
		t.Errorf("k = 1: %d of %d findings carry access_path_truncated", cut, all)
	}
	if cut, _ := run(8); cut != 0 {
		t.Errorf("k = 8: %d findings carry access_path_truncated, no path is that deep", cut)
	}
}

// The data of a source that becomes a number or a bool is not followed, and the place where it
// stops is recorded for that source (CORE-5): "no flow" and "stopped at a number" differ.
func TestNumericStopIsRecorded(t *testing.T) {
	run := func(numeric bool) []*Uncertainty {
		prog := loadIR(t, testutil.Testdata(), "./cases/numeric/...")
		rules := testutil.Rules(t)
		rules.Expand(prog.gossa, 0)
		st := prog.sites(rules)
		opts := DefaultOptions()
		opts.NumericTaint = numeric
		var out []*Uncertainty
		for _, u := range prog.engine(st, rules, opts).Run().Uncertainties {
			if u.Kind == KindNumericStop {
				out = append(out, u)
			}
		}
		return out
	}
	got := run(false)
	if len(got) != 1 {
		t.Fatalf("%d numeric_stop records, want 1 (one source site): %+v", len(got), got)
	}
	if got[0].Symbol == "" || got[0].Func == "" || got[0].Position.Line == 0 || got[0].AttributableTo != AttributableRivus {
		t.Errorf("the record does not say where: %+v", got[0])
	}
}

// A source that returns a flag beside its data (`md, ok := metadata.FromIncomingContext(ctx)`) is not a
// number that the input became: the flag gets no record, however the call puts the label on
// all of its results (yagihash/rivus #72).
func TestCommaOkSourceLeavesNoNumericStop(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/grpcsrv/...")
	rules := testutil.Rules(t)
	rules.Expand(prog.gossa, 0)
	st := prog.sites(rules)
	for _, u := range prog.engine(st, rules, DefaultOptions()).Run().Uncertainties {
		if u.Kind == KindNumericStop {
			t.Errorf("numeric_stop for %s at %s: %s", u.Symbol, u.Position.File, u.Detail)
		}
	}
}

// A string read rune by rune stops at the numeric rule inside the function that does it, and the
// count says where (summary.numeric_stops); a program that reads nothing as a number has no count.
func TestNumericStopsCountedPerFunction(t *testing.T) {
	counts := func(pat string) map[string]int {
		prog := loadIR(t, testutil.Testdata(), pat)
		rules := testutil.Rules(t)
		rules.Expand(prog.gossa, 0)
		st := prog.sites(rules)
		m := map[string]int{}
		for _, c := range prog.engine(st, rules, DefaultOptions()).Run().NumericStops {
			m[c.Func] = c.Char
		}
		return m
	}
	got := counts("./cases/numrune/...")
	n := 0
	for f, c := range got {
		if strings.HasSuffix(f, "PerRune") {
			n = c
		}
	}
	if n == 0 {
		t.Errorf("no numeric stop counted in PerRune: %v", got)
	}
	for f, c := range counts("./cases/numnone/...") {
		t.Errorf("numeric stop counted where nothing is read as a number: %s %d", f, c)
	}
}
