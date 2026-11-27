package taint

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// Diagnostics (ADR-061). They say what the run did, and change nothing in what it finds: they are
// asked for (Options.PassStats, Options.TraceElement), and the results are the same with and without them.

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// PassChange is the summaries that changed in one pass of the run. A run that does not converge is
// understood from it: new flows keep coming (the analysis is still finding things), or only the
// guards change, or the elements go up and down.
type PassChange struct {
	Pass int `json:"pass"`
	// Changed is the number of functions whose summary changed (the first summary of a function is not a change).
	Changed int `json:"changed"`
	// NewFlow: an element was added (Added counts them, without their guards). Of them, New were in no
	// summary of the function before (the analysis found something), and Returned were in an earlier one and
	// not in the last (the elements go up and down).
	NewFlow  int `json:"new_flow"`
	Added    int `json:"added_elems"`
	New      int `json:"new_elems"`
	Returned int `json:"returned_elems"`
	// GuardOnly: the same elements, other guards.
	GuardOnly int `json:"guard_only"`
	// Shrunk: an element was taken away (a summary is joined with the earlier one, so it should not happen).
	Shrunk int `json:"shrunk,omitempty"`
}

func summaryPairs(sum *Summary, withGuards bool) map[string]bool {
	m := map[string]bool{}
	if sum == nil {
		return m
	}
	for _, o := range sum.Outputs {
		g := ""
		if o.Global != nil {
			g = o.Global.Name
		}
		for _, e := range o.Elems {
			k := o.Kind + "|" + itoa(o.Index) + "|" + g + "|" + o.Path + "|" + e.E.String()
			if withGuards {
				k += "|" + guardsKey(e.Guards)
			}
			m[k] = true
		}
	}
	for _, k := range sum.Sinks {
		key := "sink|" + k.Site + "|" + k.Elem.String()
		if withGuards {
			key += "|" + guardsKey(k.Guards)
		}
		m[key] = true
	}
	return m
}

// notePassChange records that the summary of a function changed in the current pass. It is called with e.mu held.
func (e *Engine) notePassChange(fn *sg.Func, old, cur *Summary) {
	if !e.opts.PassStats || old == nil {
		return
	}
	pc := e.passChange(e.pass)
	pc.Changed++
	a, b := summaryPairs(old, false), summaryPairs(cur, false)
	added, removed := 0, 0
	for k := range b {
		if !a[k] {
			added++
		}
	}
	for k := range a {
		if !b[k] {
			removed++
		}
	}
	ever := e.seenEver[fn]
	if ever == nil {
		if e.seenEver == nil {
			e.seenEver = map[*sg.Func]map[uint64]struct{}{}
		}
		ever = map[uint64]struct{}{}
		e.seenEver[fn] = ever
		for k := range a {
			ever[hash64(k)] = struct{}{}
		}
	}
	for k := range b {
		h := hash64(k)
		if _, seen := ever[h]; !seen {
			ever[h] = struct{}{}
			if !a[k] {
				pc.New++
			}
		} else if !a[k] {
			pc.Returned++
		}
	}
	switch {
	case added > 0:
		pc.NewFlow++
		pc.Added += added
	case removed > 0:
		pc.Shrunk++
	default:
		ag, bg := summaryPairs(old, true), summaryPairs(cur, true)
		same := len(ag) == len(bg)
		for k := range bg {
			if !ag[k] {
				same = false
			}
		}
		if !same {
			pc.GuardOnly++
		}
	}
}

func (e *Engine) passChange(pass int) *PassChange {
	for i := range e.passChanges {
		if e.passChanges[i].Pass == pass {
			return &e.passChanges[i]
		}
	}
	e.passChanges = append(e.passChanges, PassChange{Pass: pass})
	return &e.passChanges[len(e.passChanges)-1]
}

// Following one source (-trace-element <site id>): prints, to stderr, where the elements of that
// source first got out of the function that holds them: into the summary of a function (the first time
// for each function), and into the store of a package variable. The order of the lines is the
// order of the work, which differs from run to run when the functions are analysed together.

func (e *Engine) tracing(el Elem) bool {
	return e.opts.TraceElement != "" && el.Kind == Real && el.Origin() == e.opts.TraceElement
}

func (e *Engine) traceSummary(fn *sg.Func, sum *Summary) {
	if e.opts.TraceElement == "" {
		return
	}
	if e.traced == nil {
		e.traced = map[*sg.Func]bool{}
	}
	if e.traced[fn] {
		return
	}
	for _, o := range sum.Outputs {
		for _, oe := range o.Elems {
			if e.tracing(oe.E) {
				e.traced[fn] = true
				first := ""
				if st := oe.Steps.flatten(2); len(st) > 0 {
					first = st[0].Desc
				}
				fmt.Fprintf(os.Stderr, "TRACE pass%d fn=%s out=%s/%d path=%q guards=%d first-step=%q\n", e.pass, fn.ID, o.Kind, o.Index, o.Path, len(oe.Guards), first)
				return
			}
		}
	}
}

func (e *Engine) traceGlobal(g *sg.Value, path string, el Elem) {
	if e.tracing(el) {
		fmt.Fprintf(os.Stderr, "TRACE pass%d global=%s path=%q\n", e.pass, g.Name, path)
	}
}

// traceWrite says what put the element into the package variable: the function that stored it, and the last steps of the path.
func (s *state) traceWrite(fn *sg.Func, g *sg.Value, root Root, path string, el Elem) {
	steps := s.trace(Loc{root, path}, el).flatten(1 << 20)
	from := len(steps) - 4
	if from < 0 {
		from = 0
	}
	fmt.Fprintf(os.Stderr, "TRACEWRITE fn=%s global=%s path=%q steps=%d\n", fn.ID, g.Name, path, len(steps))
	for _, st := range steps[from:] {
		fmt.Fprintf(os.Stderr, "    %s %s:%d  %s\n", st.Func, st.Position.File, st.Position.Line, st.Desc)
	}
}

// UnmodeledSymbol is a function without a model, and what its lack did: Reached is how many times a tainted
// value got to one of its arguments (summed over the analyses of the functions that call it; a call whose
// arguments hold nothing tainted does not count), and OnPath how many findings have it on the path shown.
type UnmodeledSymbol struct {
	Symbol  string `json:"symbol"`
	Reached int    `json:"reached"`
	OnPath  int    `json:"on_path"`
}

type unmodeledCount struct {
	mu      sync.Mutex
	reached map[string]int
}

func (e *Engine) noteUnmodeled(sym string, reached bool) {
	if !reached {
		return
	}
	e.unmodeled.mu.Lock()
	if e.unmodeled.reached == nil {
		e.unmodeled.reached = map[string]int{}
	}
	e.unmodeled.reached[sym]++
	e.unmodeled.mu.Unlock()
}

// unmodeledTop is the 20 functions without a model that tainted values reached the most, with
// the number of findings that have the call on their path (the ones in the result).
func (e *Engine) unmodeledTop(findings []*Finding) []UnmodeledSymbol {
	e.unmodeled.mu.Lock()
	defer e.unmodeled.mu.Unlock()
	onPath := map[string]int{}
	for _, f := range findings {
		seen := map[string]bool{}
		for _, p := range f.Paths {
			for _, st := range p.Steps {
				d := st.Desc
				if strings.HasPrefix(d, "call ") && strings.HasSuffix(d, " (unmodeled)") {
					seen[strings.TrimSuffix(strings.TrimPrefix(d, "call "), " (unmodeled)")] = true
				}
			}
		}
		for sym := range seen {
			onPath[sym]++
		}
	}
	var out []UnmodeledSymbol
	for sym, n := range e.unmodeled.reached {
		out = append(out, UnmodeledSymbol{sym, n, onPath[sym]})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Reached != b.Reached {
			return a.Reached > b.Reached
		}
		if a.OnPath != b.OnPath {
			return a.OnPath > b.OnPath
		}
		return a.Symbol < b.Symbol
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// PackageStat is where the work of a run went, for one package (a Go import path, a Dart "package:", a
// Rust crate): how many analyses of its functions there were (Reanalyses; the first one of each counts), the
// time they took (SpentMilli: a clock, so it differs from run to run), the elements of the summaries
// of its functions, and the largest of them. It tells a package that is slow because it is
// large from one that is slow because it is joined with everything (a mock, a generated
// package, a function that takes any value).
type PackageStat struct {
	Package          string `json:"package"`
	Dep              bool   `json:"dep,omitempty"`
	Functions        int    `json:"functions"`
	Reanalyses       int    `json:"reanalyses"`
	SpentMilli       int64  `json:"spent_ms"`
	SummaryElems     int    `json:"summary_elems"`
	LargestSummary   string `json:"largest_summary,omitempty"`
	LargestSummaryEl int    `json:"largest_summary_elems,omitempty"`
}

type pkgAcc struct {
	reanalyses int
	spent      time.Duration
}

// notePackage counts one analysis of a function. It is called with e.mu held.
func (e *Engine) notePackage(fn *sg.Func, took time.Duration) {
	if !e.opts.PassStats {
		return
	}
	if e.pkgs == nil {
		e.pkgs = map[string]*pkgAcc{}
	}
	u := UnitOf(fn.ID)
	a := e.pkgs[u]
	if a == nil {
		a = &pkgAcc{}
		e.pkgs[u] = a
	}
	a.reanalyses++
	a.spent += took
}

// packagesTop is the packages (PassStatsTop of them, 20 by default) with the most analyses of their functions (the count
// does not depend on the clock, so the list does not, unless -pass-stats-sort is time; ties: the larger summaries, then
// the name).
func (e *Engine) packagesTop() []PackageStat {
	by := map[string]*PackageStat{}
	get := func(u string) *PackageStat {
		p := by[u]
		if p == nil {
			p = &PackageStat{Package: u}
			by[u] = p
		}
		return p
	}
	for _, fn := range e.prog.Funcs {
		p := get(UnitOf(fn.ID))
		p.Functions++
		if fn.Dep {
			p.Dep = true
		}
	}
	for fn, sum := range e.summaries {
		p := get(UnitOf(fn.ID))
		n := 0
		for _, o := range sum.Outputs {
			n += len(o.Elems)
		}
		p.SummaryElems += n
		if n > p.LargestSummaryEl || (n == p.LargestSummaryEl && n > 0 && fn.ID < p.LargestSummary) {
			p.LargestSummary, p.LargestSummaryEl = fn.ID, n
		}
	}
	for u, a := range e.pkgs {
		p := get(u)
		p.Reanalyses = a.reanalyses
		p.SpentMilli = a.spent.Milliseconds()
	}
	out := make([]PackageStat, 0, len(by))
	for _, p := range by {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch e.opts.PassStatsSort {
		case "time":
			if a.SpentMilli != b.SpentMilli {
				return a.SpentMilli > b.SpentMilli
			}
		case "functions":
			if a.Functions != b.Functions {
				return a.Functions > b.Functions
			}
		case "elems":
			if a.SummaryElems != b.SummaryElems {
				return a.SummaryElems > b.SummaryElems
			}
		default:
			if a.Reanalyses != b.Reanalyses {
				return a.Reanalyses > b.Reanalyses
			}
			if a.SummaryElems != b.SummaryElems {
				return a.SummaryElems > b.SummaryElems
			}
		}
		return a.Package < b.Package
	})
	if n := e.opts.PassStatsTop; n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// SCCPackage is the number of functions of the largest component that belong to one package.
type SCCPackage struct {
	Package   string `json:"package"`
	Functions int    `json:"functions"`
}

// largestSCCPackages is the 10 packages with the most functions in the largest component (ties: the name).
func (e *Engine) largestSCCPackages() []SCCPackage {
	count := map[string]int{}
	for _, fn := range e.largestSCC {
		count[UnitOf(fn.ID)]++
	}
	out := make([]SCCPackage, 0, len(count))
	for u, n := range count {
		out = append(out, SCCPackage{u, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Functions != out[j].Functions {
			return out[i].Functions > out[j].Functions
		}
		return out[i].Package < out[j].Package
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

// NotNarrowed counts the calls of closures (Rust, Fn::call) that kept every candidate of the extractor, by why.
type NotNarrowed struct {
	OpenCallers   int `json:"open_callers"`
	UnknownOrigin int `json:"unknown_origin"`
	Limit         int `json:"limit"`
}

// DispatchCandidates is how many functions the calls through an interface, a trait or a function value
// may reach: the calls (the call sites of the code, once each) by that number. A call with many is where the
// analysis joins many things, which is what makes a mock or a variadic "any" slow.
type DispatchCandidates struct {
	One       int    `json:"one"`
	To8       int    `json:"two_to_8"`
	To16      int    `json:"9_to_16"`
	Over16    int    `json:"over_16"`
	None      int    `json:"none,omitempty"`
	Largest   int    `json:"largest"`
	LargestAt string `json:"largest_at,omitempty"`
}

func (e *Engine) dispatchCandidates() *DispatchCandidates {
	d := &DispatchCandidates{}
	for _, fn := range e.prog.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Callee == nil || in.StaticID != "" {
					continue
				}
				n := len(in.Candidates)
				if in.Callee.Kind == ir.CalleeValue && len(in.Callee.ValueCands) > n {
					n = len(in.Callee.ValueCands)
				}
				if in.Callee.Kind != ir.CalleeDynamic && in.Callee.Kind != ir.CalleeValue {
					continue
				}
				switch {
				case n == 0:
					d.None++
				case n == 1:
					d.One++
				case n <= 8:
					d.To8++
				case n <= 16:
					d.To16++
				default:
					d.Over16++
				}
				if n > d.Largest || (n == d.Largest && n > 0 && fn.ID < d.LargestAt) {
					d.Largest, d.LargestAt = n, fn.ID
				}
			}
		}
	}
	return d
}

// ValueCandidateBuckets are the buckets of summary.value_candidates, in the order of the numbers. The edge is at
// ir.ValueEnumerateMost (32): a call that was found with 33 or more is the one that rivus leaves unresolved (ADR-050).
var ValueCandidateBuckets = []string{"0", "1", "2-5", "6-20", "21-32", "33+"}

// ValueCandidateBucket is the bucket of a call of a function value that was found with n functions.
func ValueCandidateBucket(n int) string {
	switch {
	case n == 0:
		return "0"
	case n == 1:
		return "1"
	case n <= 5:
		return "2-5"
	case n <= 20:
		return "6-20"
	case n <= ir.ValueEnumerateMost:
		return "21-32"
	}
	return "33+"
}

// valueCandidateCounts counts the calls of function values (the call sites of the code, once each) by how many
// functions were found for the value: the candidates when there are some, or, when rivus left them out (more than
// ir.ValueEnumerateMost, or only a part of the callers was followed), the number that was found. The IR, not the
// analysis: the same for every run.
func (e *Engine) valueCandidateCounts() map[string]int {
	out := map[string]int{}
	for _, fn := range e.prog.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Callee == nil || in.Callee.Kind != ir.CalleeValue || in.StaticID != "" {
					continue
				}
				n := len(in.Callee.ValueCands)
				if (in.Callee.ValueLimit == ir.ValueCandidateLimit || in.Callee.ValueLimit == ir.ValuePartial) && in.Callee.ValueFound > n {
					n = in.Callee.ValueFound
				}
				out[ValueCandidateBucket(n)]++
			}
		}
	}
	return out
}
