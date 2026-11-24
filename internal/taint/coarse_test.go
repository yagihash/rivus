package taint

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/testutil"
)

// runCoarse analyses testdata/scccoarse (one component of 12 functions, with global variables and writes below two
// steps of a parameter) with the functions for which dep says so taken as functions of a dependency.
func runCoarse(t *testing.T, dep func(id string) bool, mod func(*Options)) (*Engine, *Result) {
	t.Helper()
	dir := filepath.Join(testutil.Testdata(), "..", "scccoarse")
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rs, err := model.Parse(f, "rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rules := model.Merge(append(sets, rs)...)
	if err := rules.Validate(); err != nil {
		t.Fatal(err)
	}
	l := loadIR(t, dir, "./...")
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

// elemSet is the set of (kind, index, global, path, element) of a summary; cut: the paths are cut at depth 0.
func elemSet(sum *Summary, cut bool) map[string]bool {
	m := map[string]bool{}
	for _, o := range sum.Outputs {
		g := ""
		if o.Global != nil {
			g = o.Global.Name
		}
		for _, oe := range o.Elems {
			p, e := o.Path, oe.E
			if cut && o.Kind == OutParam && e.Kind == Param && int(e.Param) == o.Index && e.Mods() == "" {
				continue // the parameter's own taint in itself: no flow
			}
			if cut {
				p = ""
				if e.Kind == Param {
					e = e.WithPath("")
				}
			}
			m[o.Kind+"|"+itoa(o.Index)+"|"+g+"|"+p+"|"+e.String()] = true
		}
	}
	return m
}

// canon is the meaning of a summary, without the order of its outputs (the first visit of a function gives them in
// the order of its cells, a later one in the sorted order: the same meaning, another signature): the elements with
// their guards, and the sinks.
func canon(sum *Summary) string {
	var l []string
	for _, o := range sum.Outputs {
		g := ""
		if o.Global != nil {
			g = o.Global.Name
		}
		for _, oe := range o.Elems {
			l = append(l, "out|"+o.Kind+"|"+itoa(o.Index)+"|"+g+"|"+o.Path+"|"+oe.E.String()+"|"+guardsKey(oe.Guards))
		}
	}
	for _, sr := range sum.Sinks {
		l = append(l, "sink|"+sr.Site+"|"+sr.Elem.String()+"|"+sr.Field+"|"+guardsKey(sr.Guards))
	}
	sort.Strings(l)
	return strings.Join(l, "\n")
}

func records(res *Result) []*Uncertainty {
	var out []*Uncertainty
	for _, u := range res.Uncertainties {
		if strings.HasPrefix(u.Detail, "scc_coarsened") {
			out = append(out, u)
		}
	}
	return out
}

// With the budget off, nothing is moved and the output has no record of it.
func TestCoarseBudgetOffChangesNothing(t *testing.T) {
	e, res := runCoarse(t, isF, nil)
	if len(e.coarseList) != 0 || res.Stats.SCCCoarsened != 0 || len(records(res)) != 0 {
		t.Fatalf("coarsened with no budget")
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
	e, res := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsen = 1 })
	if len(e.coarseList) != 1 || res.Stats.SCCCoarsened != 1 {
		t.Fatalf("moved %d times, want once", len(e.coarseList))
	}
	c := e.coarseList[0]
	if c.size != 12 || len(c.funcs) != 11 || c.round < 1 || c.cause != "deep_param_keys" {
		t.Errorf("the record: size %d, deps %d, round %d, cause %q", c.size, len(c.funcs), c.round, c.cause)
	}
	rs := records(res)
	if len(rs) != 1 || rs[0].Kind != KindDepthLimit || rs[0].AttributableTo != AttributableRivus || len(rs[0].Candidates) != 11 {
		t.Fatalf("the records: %v", rs)
	}
	for fn, sum := range e.summaries {
		if !isF(fn.ID) {
			continue
		}
		if (sum.Coarse != nil) != (e.coarseOf(fn) != nil) || (sum.Coarse != nil && !fn.Dep) {
			t.Errorf("%s: dependency %v, coarse %v", fn.ID, fn.Dep, sum.Coarse != nil)
		}
		if sum.Coarse != nil {
			// not undone: nothing is deeper than the first step in a dependency function
			for _, o := range sum.Outputs {
				if o.Kind == OutParam && pathDepth(o.Path) >= 1 {
					t.Errorf("%s: a path %q is left", fn.ID, o.Path)
				}
			}
		}
	}
}

// -workers 1 and the default give the same summaries, the same records and the same stats, with the move.
func TestCoarseSameForAnyWorkers(t *testing.T) {
	var first, firstRec string
	for _, w := range []int{1, 2, 0} {
		e, res := runCoarse(t, isF, func(o *Options) { o.SCCCoarsen = 2; o.Workers = w })
		rs := records(res)
		if len(rs) != 1 {
			t.Fatalf("workers %d: %d records", w, len(rs))
		}
		d, r := dumpOf(t, e), rs[0].Detail+strings.Join(rs[0].Candidates, ",")
		if first == "" {
			first, firstRec = d, r
		} else if d != first || r != firstRec {
			t.Fatalf("workers %d differ", w)
		}
	}
}

// Solved again from the start, the summaries (elements and guards) are the ones of a run that had the dependency
// functions at depth 0 from the start (before the first round), whatever the round the budget was hit at. They contain
// those of the run that was not moved, with the paths cut.
func TestCoarseSolvedAgainIsFromStart(t *testing.T) {
	plain, _ := runCoarse(t, isF, nil)
	start, _ := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenAtRound = -1 })
	if len(start.coarseList) != 1 {
		t.Fatal("not moved at the start")
	}
	want := map[string]string{}
	for id, sum := range byID(start) {
		want[id] = canon(sum)
	}
	for id, sum := range byID(plain) {
		if !isF(id) {
			continue
		}
		got := elemSet(byID(start)[id], false)
		for k := range elemSet(sum, true) {
			if !got[k] {
				t.Errorf("%s: %s of the run at depth 2 is not in the run at depth 0", id, k)
			}
		}
	}
	for _, round := range []int{1, 2, 3, 5} {
		e, _ := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenAtRound = round })
		if len(e.coarseList) != 1 {
			t.Fatalf("round %d: moved %d times", round, len(e.coarseList))
		}
		got := byID(e)
		if len(got) != len(want) {
			t.Errorf("round %d: %d summaries, want %d", round, len(got), len(want))
		}
		for id, sig := range want {
			if got[id] == nil || canon(got[id]) != sig {
				t.Errorf("round %d: %s is not the summary of the run from the start", round, id)
			}
		}
	}
	// by the budget (a count, not a forced round)
	e, _ := runCoarse(t, isF, func(o *Options) { o.SCCCoarsen = 2 })
	for id, sig := range want {
		if byID(e)[id] == nil || canon(byID(e)[id]) != sig {
			t.Errorf("budget: %s is not the summary of the run from the start", id)
		}
	}
}

// The functions of the project in the component keep their depth: they are not at depth 0.
func TestCoarseProjectFunctionsKeepTheirDepth(t *testing.T) {
	isDep := func(id string) bool {
		return isF(id) && !strings.HasSuffix(id, ".f05") && !strings.HasSuffix(id, ".f11")
	}
	e, _ := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsenAtRound = 1 })
	if len(e.coarseList) != 1 {
		t.Fatal("not moved")
	}
	deep := false
	for fn, sum := range e.summaries {
		if fn.Dep || !isF(fn.ID) {
			continue
		}
		for _, o := range sum.Outputs {
			deep = deep || pathDepth(o.Path) >= 1
		}
	}
	if !deep {
		t.Error("the functions of the project have no path below the first step (f05 and f11 write to t.next.next)")
	}
}

// A hop of a call of a coarse summary has the note, so the flow through it is told to be coarser than the code.
func TestCoarseNoteOnHops(t *testing.T) {
	e, _ := runCoarse(t, func(id string) bool {
		return isF(id) && !strings.HasSuffix(id, ".f00") && !strings.HasSuffix(id, ".f01")
	}, func(o *Options) { o.SCCCoarsenAtRound = -1 })
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

// globalsDump is the global state: for each variable, path and element, the functions that wrote it.
func globalsDump(e *Engine) string {
	var l []string
	for v, cm := range e.globals.cells {
		for path, set := range cm {
			for el := range set {
				w := append([]string(nil), e.globals.writerFns[v][path][el]...)
				sort.Strings(w)
				l = append(l, v.Name+"|"+path+"|"+el.String()+"|"+strings.Join(w, ","))
			}
		}
	}
	for v, m := range e.globals.readers_ {
		for fn := range m {
			l = append(l, "reader|"+v.Name+"|"+fn.ID)
		}
	}
	sort.Strings(l)
	return strings.Join(l, "\n")
}

// wholeResult is the result of a run, without what says how much work was done (it counts the thrown-away rounds
// too: reanalyses, passes, the work counted in elements, the times).
func wholeResult(t *testing.T, res *Result) string {
	t.Helper()
	b, err := json.Marshal(res.Stats)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"reanalyses", "passes", "adds_total", "adds_max_single_run", "adds_max_function_total", "adds_per_function",
		"scc_restarts", "duration_ms", "dep_spent_ms", "own_spent_ms", "pass_changes", "summary_steps"} {
		delete(st, k)
	}
	sb, _ := json.Marshal(st)
	fb, _ := json.Marshal(res.Findings)
	var us []*Uncertainty // the records of the move say at which round: that is what differs
	for _, u := range res.Uncertainties {
		if !strings.HasPrefix(u.Detail, "scc_coarsened") {
			us = append(us, u)
		}
	}
	ub, _ := json.Marshal(us)
	return string(sb) + "\n" + string(fb) + "\n" + string(ub)
}

// With functions of the project in the component (they write to the global variables, and keep their depth), and a
// function outside it that reads them, solved again is still the run from the start: the summaries, the global state,
// the findings, the uncertainties and the stats (but the work that was done).
func TestCoarseSolvedAgainWithProjectFunctions(t *testing.T) {
	isDep := func(id string) bool {
		return isF(id) && !strings.HasSuffix(id, ".f04") && !strings.HasSuffix(id, ".f08") && !strings.HasSuffix(id, ".f01")
	}
	start, sres := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsenAtRound = -1 })
	if len(sres.Findings) == 0 || !strings.Contains(globalsDump(start), "scc.G") {
		t.Logf("findings %d, globals %q", len(sres.Findings), globalsDump(start))
		t.Fatal("the program has no findings or no global state: the test shows nothing")
	}
	for _, round := range []int{1, 3} {
		e, res := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsenAtRound = round })
		if len(e.coarseList) != 1 {
			t.Fatalf("round %d: not moved", round)
		}
		for id, sum := range byID(start) {
			if got := byID(e)[id]; got == nil || canon(got) != canon(sum) {
				t.Errorf("round %d: %s differs from the run from the start", round, id)
			}
		}
		if a, b := globalsDump(start), globalsDump(e); a != b {
			t.Errorf("round %d: the global state differs from the run from the start\n%s\n---\n%s", round, a, b)
		}
		if a, b := wholeResult(t, sres), wholeResult(t, res); a != b {
			t.Errorf("round %d: the result differs from the run from the start\n%s\n---\n%s", round, a, b)
		}
	}
}

// Hit in a later pass (the component has written to the global state by then), the whole analysis is started again
// with the component at depth 0 from the start: the answer is the one of the run from the start (the summaries, the
// global state, the findings, the uncertainties, the stats but the work that was done), and the record says so.
func TestCoarseLaterPassStartsAgain(t *testing.T) {
	isDep := func(id string) bool {
		return isF(id) && !strings.HasSuffix(id, ".f04") && !strings.HasSuffix(id, ".f08") && !strings.HasSuffix(id, ".f01")
	}
	start, sres := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsenAtRound = -1 })
	e, res := runCoarse(t, isDep, func(o *Options) { o.SCCCoarsenAtRound = 9 })
	if len(e.coarseList) != 1 || !e.coarseList[0].restarted || e.coarseList[0].pass < 2 || res.Stats.SCCRestarts != 1 {
		t.Fatalf("not hit in a later pass: %d moves, restarts %d", len(e.coarseList), res.Stats.SCCRestarts)
	}
	if rs := records(res); len(rs) != 1 || !strings.Contains(rs[0].Detail, "started again") {
		t.Errorf("the record does not say that the analysis was started again: %v", rs)
	}
	for id, sum := range byID(start) {
		if got := byID(e)[id]; got == nil || canon(got) != canon(sum) {
			t.Errorf("%s differs from the run from the start", id)
		}
	}
	if a, b := globalsDump(start), globalsDump(e); a != b {
		t.Errorf("the global state differs from the run from the start\n%s\n---\n%s", a, b)
	}
	if a, b := wholeResult(t, sres), wholeResult(t, res); a != b {
		t.Errorf("the result differs from the run from the start\n%s\n---\n%s", a, b)
	}
}
