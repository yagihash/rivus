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
	return runCoarsePre(t, dep, mod, nil)
}

// runCoarsePre is runCoarse with pre called on the engine before the run.
func runCoarsePre(t *testing.T, dep func(id string) bool, mod func(*Options), pre func(*Engine)) (*Engine, *Result) {
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
	if pre != nil {
		pre(e)
	}
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
		"scc_coarsened", "duration_ms", "dep_spent_ms", "own_spent_ms", "pass_changes", "summary_steps"} {
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

// The components of testdata/scccoarse: f02 to f13 call each other (12 functions); f00, f01 and main are outside it.
func inSCC(id string) bool {
	for _, n := range []string{"f02", "f03", "f04", "f05", "f06", "f07", "f08", "f09", "f10", "f11", "f12", "f13"} {
		if strings.HasSuffix(id, "."+n) {
			return true
		}
	}
	return false
}

// mixed: f04 and f08 (in the component) and f01 (outside) are functions of the project, the rest is a dependency.
func mixed(id string) bool {
	return isF(id) && !strings.HasSuffix(id, ".f04") && !strings.HasSuffix(id, ".f08") && !strings.HasSuffix(id, ".f01")
}

// With the setting off, or a component smaller than N, nothing is solved at depth 0 and the output has no record of it.
func TestCoarseOffChangesNothing(t *testing.T) {
	plain, pres := runCoarse(t, isF, nil)
	for _, n := range []int{0, 13} {
		e, res := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenN = n })
		if len(e.coarseList) != 0 || res.Stats.SCCCoarsened != 0 || len(records(res)) != 0 {
			t.Fatalf("N=%d: coarsened", n)
		}
		if wholeResult(t, pres) != wholeResult(t, res) || globalsDump(plain) != globalsDump(e) {
			t.Errorf("N=%d: the result is not the one of the run with the setting off", n)
		}
		for id, sum := range byID(plain) {
			if canon(byID(e)[id]) != canon(sum) {
				t.Errorf("N=%d: %s differs", n, id)
			}
		}
	}
}

// A component of N functions or more is solved at depth 0: only its dependency functions, once, and the record says
// how large it is. What it works out contains what the run at depth 2 has, with the paths cut.
func TestCoarseLargeComponent(t *testing.T) {
	plain, _ := runCoarse(t, isF, nil)
	e, res := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenN = 12 })
	if len(e.coarseList) != 1 || res.Stats.SCCCoarsened != 1 {
		t.Fatalf("%d components, want 1", len(e.coarseList))
	}
	c := e.coarseList[0]
	if c.size != 12 || len(c.funcs) != 12 {
		t.Errorf("the component: %d functions, %d dependency functions", c.size, len(c.funcs))
	}
	rs := records(res)
	if len(rs) != 1 || rs[0].Kind != KindDepthLimit || rs[0].AttributableTo != AttributableRivus || len(rs[0].Candidates) != 12 {
		t.Fatalf("the records: %v", rs)
	}
	for fn, sum := range e.summaries {
		if !isF(fn.ID) {
			continue
		}
		if (sum.Coarse != nil) != inSCC(fn.ID) {
			t.Errorf("%s: coarse %v, in the component %v (f00 and f01 are dependencies outside it)", fn.ID, sum.Coarse != nil, inSCC(fn.ID))
		}
		if sum.Coarse == nil {
			continue
		}
		for _, o := range sum.Outputs {
			if o.Kind == OutParam && pathDepth(o.Path) >= 1 {
				t.Errorf("%s: a path %q is left", fn.ID, o.Path)
			}
		}
		got := elemSet(sum, false)
		for k := range elemSet(byID(plain)[fn.ID], true) {
			if !got[k] {
				t.Errorf("%s: %s of the run at depth 2 is not in the run at depth 0", fn.ID, k)
			}
		}
	}
}

// N counts the functions of the component, or (SCCCoarsenDeps) only the dependency functions in it.
func TestCoarseWhatIsCounted(t *testing.T) {
	cases := []struct {
		n    int
		deps bool
		want int
	}{{12, false, 1}, {13, false, 0}, {10, true, 1}, {11, true, 0}}
	for _, c := range cases {
		e, _ := runCoarse(t, mixed, func(o *Options) { o.SCCCoarsenN = c.n; o.SCCCoarsenDeps = c.deps })
		if len(e.coarseList) != c.want {
			t.Errorf("N=%d deps=%v: %d components, want %d", c.n, c.deps, len(e.coarseList), c.want)
		}
	}
}

// The answer is the one of a run in which the dependency functions of that component have access paths at depth 0 and
// nothing else does: the summaries, the global state, the findings, the uncertainties and the stats (but the work
// that was done). With functions of the project in the component that write to the global variables, and a function
// outside it that reads them.
func TestCoarseIsDepthZeroForThoseFunctions(t *testing.T) {
	ref, rres := runCoarsePre(t, mixed, nil, func(e *Engine) {
		c := &coarseSCC{}
		for _, fn := range e.prog.Funcs {
			if fn.Dep && inSCC(fn.ID) {
				e.coarse[fn] = c
			}
		}
	})
	if len(rres.Findings) == 0 || !strings.Contains(globalsDump(ref), "scc.G") {
		t.Fatalf("the program has no findings or no global state: the test shows nothing")
	}
	e, res := runCoarse(t, mixed, func(o *Options) { o.SCCCoarsenN = 12 })
	if len(e.coarseList) != 1 {
		t.Fatal("not solved at depth 0")
	}
	for id, sum := range byID(ref) {
		if got := byID(e)[id]; got == nil || canon(got) != canon(sum) {
			t.Errorf("%s differs from the run with those functions at depth 0", id)
		}
	}
	if a, b := globalsDump(ref), globalsDump(e); a != b {
		t.Errorf("the global state differs\n%s\n---\n%s", a, b)
	}
	if a, b := wholeResult(t, rres), wholeResult(t, res); a != b {
		t.Errorf("the result differs\n%s\n---\n%s", a, b)
	}
}

// -workers 1 and the default give the same summaries, the same records and the same stats.
func TestCoarseSameForAnyWorkers(t *testing.T) {
	var first, firstRec string
	for _, w := range []int{1, 2, 0} {
		e, res := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenN = 12; o.Workers = w })
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

// The functions of the project in the component keep their depth.
func TestCoarseProjectFunctionsKeepTheirDepth(t *testing.T) {
	// f05 and f11 write to t.next.next: as functions of the project they keep the paths below the first step
	e, _ := runCoarse(t, func(id string) bool {
		return isF(id) && !strings.HasSuffix(id, ".f05") && !strings.HasSuffix(id, ".f11")
	}, func(o *Options) { o.SCCCoarsenN = 12 })
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
		t.Error("the functions of the project have no path below the first step")
	}
}

// A hop of a call of a coarse summary has the note, so the flow through it is told to be coarser than the code.
func TestCoarseNoteOnHops(t *testing.T) {
	e, _ := runCoarse(t, func(id string) bool {
		return isF(id) && !strings.HasSuffix(id, ".f00") && !strings.HasSuffix(id, ".f01")
	}, func(o *Options) { o.SCCCoarsenN = 12 })
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

// The wording has the three things a reader needs to tell a real flow from an over-approximation: which function of
// which package the step went through, what depth 0 means, and how to check.
func TestCoarseWordingHasThreeItems(t *testing.T) {
	id := "example.com/dep/pkg.F"
	d := sccCoarsenedDetail(id)
	for name, want := range map[string]string{
		"function name":        id,
		"package":              "package " + UnitOf(id),
		"depth 0 meaning":      "cut at depth 0",
		"over-approximation":   "more flows than there are, never fewer",
		"lost distinction":     "which fields were told apart before the cut",
		"check by reading":     "read the body of " + id,
		"check by solving off": sccCoarsenFlagOff,
	} {
		if !strings.Contains(d, want) {
			t.Errorf("the note on a hop has no %s (%q): %s", name, want, d)
		}
	}
	_, res := runCoarse(t, isF, func(o *Options) { o.SCCCoarsenN = 12 })
	rs := records(res)
	if len(rs) == 0 {
		t.Fatal("no record")
	}
	for _, u := range rs {
		for _, want := range []string{"cut at depth 0", "more flows than there are, never fewer", "which fields were told apart before the cut", "read that function", sccCoarsenFlagOff} {
			if !strings.Contains(u.Detail, want) {
				t.Errorf("the record has no %q: %s", want, u.Detail)
			}
		}
	}
}
