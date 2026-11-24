package taint

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

// runCoarse analyses testdata/scccoarse (one component of 14 functions, with global variables and writes below two
// steps of a parameter) with the functions whose name matches dep taken as functions of a dependency.
func runCoarse(t *testing.T, dep func(id string) bool, mod func(*Options)) (*Engine, *Result) {
	t.Helper()
	rules := testutil.Rules(t)
	l := loadIR(t, filepath.Join(testutil.Testdata(), "..", "scccoarse"), "./...")
	for _, fn := range l.sg.Funcs {
		fn.Dep = dep(fn.ID)
	}
	opts := DefaultOptions()
	if mod != nil {
		mod(&opts)
	}
	e := l.engine(l.sites(rules), rules, opts)
	return e, e.Run()
}

func isF(id string) bool { return strings.Contains(id, ".f") }

func dumpOf(t *testing.T, e *Engine) string {
	t.Helper()
	var b bytes.Buffer
	if err := e.WriteSummaryDump(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func byID(e *Engine) map[string]*Summary {
	m := map[string]*Summary{}
	for fn, sum := range e.summaries {
		m[e.funcID(fn)] = sum
	}
	return m
}

// elemSet is the set of (kind, index, global, path, element) of a summary.
func elemSet(sum *Summary) map[string]bool {
	m := map[string]bool{}
	for _, o := range sum.Outputs {
		g := ""
		if o.Global != nil {
			g = o.Global.Name
		}
		for _, oe := range o.Elems {
			m[o.Kind+"|"+itoa(o.Index)+"|"+g+"|"+o.Path+"|"+oe.E.String()] = true
		}
	}
	return m
}

// With the budget off, nothing is moved and the output has no record of it.
func TestCoarseBudgetOffChangesNothing(t *testing.T) {
	e, res := runCoarse(t, isF, nil)
	if len(e.coarseList) != 0 || res.Stats.SCCCoarsened != 0 {
		t.Fatalf("coarsened with no budget")
	}
	for _, u := range res.Uncertainties {
		if strings.HasPrefix(u.Detail, "scc_coarsened") {
			t.Fatalf("a record without a budget: %s", u.Detail)
		}
	}
	for fn, sum := range e.summaries {
		if sum.Coarse != nil {
			t.Fatalf("%s is coarse", fn.ID)
		}
	}
}

// Over the budget, the component is moved once, only its dependency functions, and the record says why.
func TestCoarseOverBudgetMovesOnce(t *testing.T) {
	isDep := func(id string) bool { return isF(id) && !strings.HasSuffix(id, ".f02") }
	for name, b := range map[string]SCCCoarsenBudget{"global": {GlobalKeys: 1}, "deep": {DeepParamKeys: 1}} {
		t.Run(name, func(t *testing.T) {
			e, res := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsen = b })
			if len(e.coarseList) != 1 || res.Stats.SCCCoarsened != 1 {
				t.Fatalf("moved %d times, want once", len(e.coarseList))
			}
			c := e.coarseList[0]
			if c.size != 12 || len(c.funcs) != 11 || c.round < 1 || c.cause == "" {
				t.Errorf("the record: size %d, deps %d, round %d, cause %q", c.size, len(c.funcs), c.round, c.cause)
			}
			n := 0
			for _, u := range res.Uncertainties {
				if strings.HasPrefix(u.Detail, "scc_coarsened") {
					n++
					if u.Kind != KindDepthLimit || u.AttributableTo != AttributableRivus || len(u.Candidates) != 11 {
						t.Errorf("the record: %s %s %d", u.Kind, u.AttributableTo, len(u.Candidates))
					}
				}
			}
			if n != 1 {
				t.Errorf("%d records, want 1", n)
			}
			for fn, sum := range e.summaries {
				if !isF(fn.ID) {
					continue
				}
				inSCC := e.coarseOf(fn) != nil
				if inSCC != (sum.Coarse != nil) || (sum.Coarse != nil && !fn.Dep) {
					t.Errorf("%s: dependency %v, coarse %v", fn.ID, fn.Dep, sum.Coarse != nil)
				}
				if !fn.Dep && sum.Coarse != nil {
					t.Errorf("%s: a function of the project was made coarse", fn.ID)
				}
				if sum.Coarse == nil {
					continue
				}
				// it is not undone: the outputs have no path, and every global of the component is written
				for _, o := range sum.Outputs {
					if o.Path != "" {
						t.Errorf("%s: a path %q is left", fn.ID, o.Path)
					}
				}
				if sum.Sig != foldSummary(sum, c).Sig {
					t.Errorf("%s: the fold is not idempotent", fn.ID)
				}
			}
		})
	}
}

// -workers 1 and the default give the same summaries, the same records and the same stats, with the move.
func TestCoarseSameForAnyWorkers(t *testing.T) {
	var first string
	var firstRec string
	for _, w := range []int{1, 2, 0} {
		e, res := runCoarse(t, isF, func(o *Options) { o.SCCCoarsen = SCCCoarsenBudget{GlobalKeys: 2}; o.Workers = w })
		var recs []string
		for _, u := range res.Uncertainties {
			if strings.HasPrefix(u.Detail, "scc_coarsened") {
				recs = append(recs, u.Detail+strings.Join(u.Candidates, ","))
			}
		}
		d, r := dumpOf(t, e), strings.Join(recs, "\n")
		if len(recs) != 1 {
			t.Fatalf("workers %d: %d records", w, len(recs))
		}
		if first == "" {
			first, firstRec = d, r
		} else if d != first || r != firstRec {
			t.Fatalf("workers %d differ", w)
		}
	}
}

// Moved at any round (and before the first), the final summaries are the same: the fold is a closure upwards, so
// where it is applied does not matter. And they are above the summaries of the run that is not moved (the elements
// of it, with the paths cut, are all there).
func TestCoarseAnyRoundGivesTheSameSummaries(t *testing.T) {
	plain, _ := runCoarse(t, isF, nil)
	var first map[string]string
	var firstName string
	for _, round := range []int{-1, 1, 2, 3, 5} {
		e, _ := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenAtRound = round })
		if len(e.coarseList) != 1 {
			t.Fatalf("round %d: moved %d times", round, len(e.coarseList))
		}
		c := e.coarseList[0]
		sigs := map[string]string{}
		for id, sum := range byID(e) {
			if sum.Coarse == nil {
				continue
			}
			sigs[id] = sum.Sig
			cut := elemSet(foldSummary(byID(plain)[id], c))
			got := elemSet(sum)
			for k := range cut {
				if !got[k] {
					t.Errorf("round %d, %s: %s of the plain run is missing", round, id, k)
				}
			}
		}
		if len(sigs) != 12 {
			t.Fatalf("round %d: %d coarse functions", round, len(sigs))
		}
		if first == nil {
			first, firstName = sigs, itoa(round)
			continue
		}
		for id, sg := range sigs {
			if first[id] != sg {
				t.Errorf("%s: round %d and round %s give different summaries", id, round, firstName)
			}
		}
	}
}

// A hop of a call of a coarse summary has the note, so the flow through it is told to be coarser than the code.
func TestCoarseNoteOnHops(t *testing.T) {
	e, _ := runCoarse(t, func(id string) bool { return isF(id) && !strings.HasSuffix(id, ".f00") && !strings.HasSuffix(id, ".f01") },
		func(o *Options) { o.SCCCoarsenAtRound = -1 })
	found := false
	for _, sum := range byID(e) {
		if sum.Coarse != nil {
			continue
		}
		if txt, _, _ := summaryText(sum); strings.Contains(txt, "scc_coarsened:") {
			found = true
		}
	}
	if !found {
		t.Error("no summary of a function of the project has the note on a hop")
	}
}
