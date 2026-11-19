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
