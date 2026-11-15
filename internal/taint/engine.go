package taint

import (
	"sort"
	"time"

	"github.com/yagihash/rivus/internal/sg"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// Options tunes the analysis.
type Options struct {
	// K is the maximum access path depth (ADR-014).
	K int
	// MaxPasses bounds the number of times the whole program is revisited
	// while summaries change.
	MaxPasses int
	// Timeout bounds the whole run. When it expires, the functions not yet
	// analysed are left out and the result says so (depth_limit). Zero means
	// no limit.
	Timeout time.Duration
	// FuncTimeout bounds the time of the analysis of one function, whatever
	// the budget; a function that exceeds it is treated like one that
	// exceeded the budget (depth_limit). Zero means the default.
	FuncTimeout time.Duration
	// Budget bounds the number of elements added while analysing one
	// function. A function that exceeds it is cut short and recorded as
	// depth_limit; its summary may then miss flows. Zero means the default.
	Budget int
	// SlowFunc, if set, is called for functions whose analysis took longer
	// than SlowThreshold. It is a development aid.
	SlowFunc      func(fn string, d time.Duration, outputs, roots int)
	SlowThreshold time.Duration
}

// DefaultOptions are the defaults.
func DefaultOptions() Options {
	return Options{K: 3, MaxPasses: 12, Budget: 400000, FuncTimeout: 30 * time.Second}
}

// Engine runs the whole-program analysis.
type Engine struct {
	prog  *sg.Program
	texts ir.Texts
	sites *sites.Result
	rules *model.Rules
	opts  Options

	summaries map[*sg.Func]*Summary
	states    map[*sg.Func]*state
	globals   *globalStore

	notConverged []string
	// degraded lists the functions whose analysis hit the budget. They are
	// analysed once and then left alone.
	degraded map[*sg.Func]bool
	// spent is the time each function has used so far.
	spent map[*sg.Func]time.Duration
	// guardLimit lists the functions with a flow that had more guards than
	// maxGuards: the flow is kept without guards (ADR-006).
	guardLimit map[*sg.Func]bool
	// writers lists, for each package variable, the functions that store to it.
	// deadline is when the whole run has to stop (zero: no limit).
	deadline time.Time
	writers  map[*sg.Value][]string
	// loadErrors are the problems the loading of the code had.
	loadErrors []ir.LoadError
	// callSites lists, for each function, the calls that may reach it.
	callSites map[*sg.Func][]*sg.Instr
}

// writersOf lists the functions that directly store to the package variable.
func (e *Engine) writersOf(g *sg.Value) []string {
	if e.writers == nil {
		e.writers = map[*sg.Value][]string{}
		seen := map[*sg.Value]map[string]bool{}
		for _, fn := range e.prog.Funcs {
			for _, b := range fn.Blocks {
				for _, in := range b.Instrs {
					var gl *sg.Value
					switch in.Op {
					case ir.OpGlobalStore:
						gl = e.prog.Global(in.IR.Global)
					case ir.OpStore, ir.OpFieldStore, ir.OpIndexStore:
						gl = globalRoot(in.Args[0])
					default:
						continue
					}
					if gl == nil {
						continue
					}
					id := e.funcID(fn)
					if seen[gl] == nil {
						seen[gl] = map[string]bool{}
					}
					if !seen[gl][id] {
						seen[gl][id] = true
						e.writers[gl] = append(e.writers[gl], id)
					}
				}
			}
		}
		for _, l := range e.writers {
			sort.Strings(l)
		}
	}
	return e.writers[g]
}

// globalRoot follows field and element addresses back to a package variable.
func globalRoot(v *sg.Value) *sg.Value {
	for v != nil {
		if v.Kind == sg.KGlobal {
			return v
		}
		if v.Instr != nil && v.Instr.Op == ir.OpAddrOf && len(v.Instr.Args) == 1 {
			v = v.Instr.Args[0]
			continue
		}
		return nil
	}
	return nil
}

// New creates an engine. graph may be nil: calls through interfaces and
// function values are then reported as unresolved.
func New(p *sg.Program, texts ir.Texts, st *sites.Result, rules *model.Rules, opts Options) *Engine {
	if opts.K <= 0 {
		opts.K = DefaultOptions().K
	}
	if opts.MaxPasses <= 0 {
		opts.MaxPasses = DefaultOptions().MaxPasses
	}
	if opts.Budget <= 0 {
		opts.Budget = DefaultOptions().Budget
	}
	if opts.FuncTimeout <= 0 {
		opts.FuncTimeout = DefaultOptions().FuncTimeout
	}
	return &Engine{
		prog: p, texts: texts, sites: st, rules: rules, opts: opts,
		summaries:  map[*sg.Func]*Summary{},
		states:     map[*sg.Func]*state{},
		degraded:   map[*sg.Func]bool{},
		spent:      map[*sg.Func]time.Duration{},
		guardLimit: map[*sg.Func]bool{},
		globals:    newGlobalStore(),
	}
}

func (e *Engine) funcID(fn *sg.Func) string { return fn.ID }

// SetLoadErrors tells the engine what the loading of the code could not see:
// they become missing_source uncertainties.
func (e *Engine) SetLoadErrors(errs []ir.LoadError) { e.loadErrors = errs }

func (e *Engine) noteGuardLimit(fn *sg.Func) { e.guardLimit[fn] = true }

// sanitizes reports whether the summary has a flow that passes a sanitizer
// or carries a label of a source.
func sanitizes(sum *Summary) bool {
	for _, o := range sum.Outputs {
		for _, oe := range o.Elems {
			if oe.E.Kind == Real || oe.E.Mods != "" {
				return true
			}
		}
	}
	for _, sr := range sum.Sinks {
		if sr.Elem.Mods != "" {
			return true
		}
	}
	return false
}

func (e *Engine) noteNotConverged(fn *sg.Func) {
	e.notConverged = append(e.notConverged, e.funcID(fn))
}

// Stats describes a run.
type Stats struct {
	Functions     int      `json:"functions"`
	Passes        int      `json:"passes"`
	Converged     bool     `json:"converged"`
	Reanalyses    int      `json:"reanalyses"`
	DurationMilli int64    `json:"duration_ms"`
	NotConverged  []string `json:"not_converged,omitempty"`
	TimedOut      bool     `json:"timed_out,omitempty"`
	// Unstable lists (up to ten) functions whose summary still changed in
	// the last pass, when the run did not converge.
	Unstable []string `json:"unstable,omitempty"`

	// Sizes of the summaries, to see where the time goes.
	SummaryElems   int    `json:"summary_elems"`
	SummarySteps   int    `json:"summary_steps"`
	LargestSummary string `json:"largest_summary,omitempty"`
	LargestElems   int    `json:"largest_summary_elems,omitempty"`
}

// Result is the outcome of a run.
type Result struct {
	Findings      []*Finding     `json:"findings"`
	Uncertainties []*Uncertainty `json:"uncertainties"`
	// Suppressions are the places where a model decided by an LLM removed
	// taint.
	Suppressions []*Suppression `json:"suppressions,omitempty"`
	Stats        Stats          `json:"stats"`
}

// Run analyses every function with a body, callees before callers, until the
// summaries stop changing.
func (e *Engine) Run() *Result {
	start := time.Now()
	fns := e.prog.Funcs
	callees := func(ci *sg.Instr) []*sg.Func {
		if ci.StaticID != "" {
			if ci.Static != nil {
				return []*sg.Func{ci.Static}
			}
			return nil
		}
		return ci.CandidateFns
	}
	sccs, callers := order(fns, callees)

	dirty := map[*sg.Func]bool{}
	for _, fn := range fns {
		dirty[fn] = true
	}
	var st Stats
	st.Functions = len(fns)
	var deadline time.Time
	if e.opts.Timeout > 0 {
		deadline = start.Add(e.opts.Timeout)
		e.deadline = deadline
	}
	expired := func() bool { return !deadline.IsZero() && time.Now().After(deadline) }
	for pass := 0; pass < e.opts.MaxPasses; pass++ {
		st.Passes = pass + 1
		any := false
		var changedNow []string
		for _, scc := range sccs {
			for iter := 0; iter < 8; iter++ {
				progress := false
				for _, fn := range scc {
					if expired() {
						st.TimedOut = true
						break
					}
					if !dirty[fn] || e.degraded[fn] {
						dirty[fn] = false
						continue
					}
					dirty[fn] = false
					any = true
					st.Reanalyses++
					if e.analyzeFunc(fn, dirty, callers) {
						progress = true
						if len(changedNow) < 10 {
							changedNow = append(changedNow, e.funcID(fn))
						}
					}
				}
				if !progress || len(scc) == 1 && !selfCalls(scc[0], callees) {
					break
				}
			}
		}
		st.Unstable = changedNow
		if st.TimedOut {
			break
		}
		if !any {
			st.Converged = true
			break
		}
		if !e.anyDirty(dirty) {
			st.Converged = true
			break
		}
	}
	st.NotConverged = e.notConverged
	for _, sum := range e.summaries {
		n, steps := 0, 0
		for _, o := range sum.Outputs {
			n += len(o.Elems)
			for _, oe := range o.Elems {
				steps += oe.Steps.len()
			}
		}
		st.SummaryElems += n
		st.SummarySteps += steps
		// The summaries are in a map: ties go to the smaller name, so that
		// the same code always gives the same answer.
		if n > st.LargestElems || (n == st.LargestElems && n > 0 && sum.Func < st.LargestSummary) {
			st.LargestElems, st.LargestSummary = n, sum.Func
		}
	}
	st.DurationMilli = time.Since(start).Milliseconds()
	return e.collect(st)
}

func (e *Engine) anyDirty(dirty map[*sg.Func]bool) bool {
	for _, d := range dirty {
		if d {
			return true
		}
	}
	return false
}

func selfCalls(fn *sg.Func, callees func(*sg.Instr) []*sg.Func) bool {
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			if in.Op == ir.OpCall {
				for _, c := range callees(in) {
					if c == fn {
						return true
					}
				}
			}
		}
	}
	return false
}

// analyzeFunc analyses one function and reports whether its summary or the
// global state changed. Dependents are marked dirty.
func (e *Engine) analyzeFunc(fn *sg.Func, dirty map[*sg.Func]bool, callers map[*sg.Func][]*sg.Func) bool {
	s := &state{
		eng: e, fn: fn,
		cells:       map[Root]map[string]Set{},
		paramRoot:   map[Root]int{},
		findings:    map[findingKey]*rawFinding{},
		sinkReaches: map[sinkKey]*SinkReach{},
	}
	began := time.Now()
	s.analyze()
	e.spent[fn] += time.Since(began)
	if s.exceeded {
		e.degraded[fn] = true
	}
	// A function that keeps costing time (its callees' summaries change and
	// it is visited again) is not visited again once it has used what one
	// analysis is allowed: its summary may miss flows, and says so.
	if e.spent[fn] > e.opts.FuncTimeout {
		e.degraded[fn] = true
	}
	sum := joinSummaries(e.summaries[fn], s.summarize())
	// Too many guards matter only where taint is told apart by them: where
	// something is sanitized, or a source's own label is returned.
	if s.guardOverflow && sanitizes(sum) {
		e.noteGuardLimit(fn)
	}
	if e.opts.SlowFunc != nil {
		if d := time.Since(began); d > e.opts.SlowThreshold {
			e.opts.SlowFunc(e.funcID(fn), d, len(sum.Outputs), len(s.cells))
		}
	}

	changed := false
	if old := e.summaries[fn]; old == nil || old.Sig != sum.Sig {
		changed = true
		for _, c := range callers[fn] {
			dirty[c] = true
		}
	}
	e.summaries[fn] = sum

	// Publish what the function writes to global variables.
	for r, cm := range s.cells {
		g := r.V
		if g == nil || g.Kind != sg.KGlobal {
			continue
		}
		for path, set := range cm {
			for el := range set {
				if el.Kind != Real {
					continue
				}
				if set[el] == nil {
					continue // read from the global state, not produced here
				}
				if e.globals.merge(g, path, el) {
					changed = true
					for _, other := range e.globals.readers(g) {
						dirty[other] = true
					}
				}
			}
		}
	}
	// Remember which functions read which globals.
	for r := range s.cells {
		if r.V != nil && r.V.Kind == sg.KGlobal {
			e.globals.noteReader(r.V, fn)
		}
	}
	// What the result needs of the analysis is the findings, the
	// suppressions and the uncertainties. The cells and the caches are the
	// bulk of the memory, and the function is analysed afresh if it is
	// visited again.
	e.states[fn] = &state{eng: e, fn: fn, findings: s.findings, suppressions: s.suppressions, uncertain: s.uncertain}
	return changed
}

// order computes the strongly connected components of the call graph with
// callees before callers, and the reverse edges.
func order(fns []*sg.Func, callees func(*sg.Instr) []*sg.Func) ([][]*sg.Func, map[*sg.Func][]*sg.Func) {
	inSet := map[*sg.Func]bool{}
	for _, fn := range fns {
		inSet[fn] = true
	}
	succ := map[*sg.Func][]*sg.Func{}
	callers := map[*sg.Func][]*sg.Func{}
	for _, fn := range fns {
		seen := map[*sg.Func]bool{}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall {
					continue
				}
				for _, c := range callees(in) {
					if inSet[c] && !seen[c] {
						seen[c] = true
						succ[fn] = append(succ[fn], c)
						callers[c] = append(callers[c], fn)
					}
				}
			}
		}
	}
	// Tarjan's algorithm yields components in reverse topological order,
	// that is, callees first.
	index := map[*sg.Func]int{}
	low := map[*sg.Func]int{}
	on := map[*sg.Func]bool{}
	var stack []*sg.Func
	var sccs [][]*sg.Func
	n := 0
	var visit func(v *sg.Func)
	visit = func(v *sg.Func) {
		index[v], low[v] = n, n
		n++
		stack = append(stack, v)
		on[v] = true
		for _, w := range succ[v] {
			if _, seen := index[w]; !seen {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if on[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var comp []*sg.Func
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sort.Slice(comp, func(i, j int) bool { return comp[i].ID < comp[j].ID })
			sccs = append(sccs, comp)
		}
	}
	for _, fn := range fns {
		if _, seen := index[fn]; !seen {
			visit(fn)
		}
	}
	return sccs, callers
}

// --- globals ---

type globalStore struct {
	cells    map[*sg.Value]map[string]Set
	readers_ map[*sg.Value]map[*sg.Func]bool
}

func newGlobalStore() *globalStore {
	return &globalStore{cells: map[*sg.Value]map[string]Set{}, readers_: map[*sg.Value]map[*sg.Func]bool{}}
}

func (g *globalStore) cellsOf(v *sg.Value) map[string]Set { return g.cells[v] }

func (g *globalStore) merge(v *sg.Value, path string, e Elem) bool {
	cm := g.cells[v]
	if cm == nil {
		cm = map[string]Set{}
		g.cells[v] = cm
	}
	set := cm[path]
	if set == nil {
		set = Set{}
		cm[path] = set
	}
	if _, ok := set[e]; ok {
		return false
	}
	set[e] = nil
	return true
}

func (g *globalStore) noteReader(v *sg.Value, fn *sg.Func) {
	m := g.readers_[v]
	if m == nil {
		m = map[*sg.Func]bool{}
		g.readers_[v] = m
	}
	m[fn] = true
}

func (g *globalStore) readers(v *sg.Value) []*sg.Func {
	var out []*sg.Func
	for fn := range g.readers_[v] {
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// --- results ---

func (e *Engine) collect(st Stats) *Result {
	res := &Result{Stats: st, Findings: []*Finding{}, Uncertainties: []*Uncertainty{}}

	type best struct {
		rf *rawFinding
	}
	merged := map[findingKey]*rawFinding{}
	fnList := make([]*sg.Func, 0, len(e.states))
	for fn := range e.states {
		fnList = append(fnList, fn)
	}
	sort.Slice(fnList, func(i, j int) bool { return fnList[i].ID < fnList[j].ID })

	seenU := map[string]bool{}
	for _, fn := range fnList {
		s := e.states[fn]
		keys := make([]findingKey, 0, len(s.findings))
		for k := range s.findings {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].origin != keys[j].origin {
				return keys[i].origin < keys[j].origin
			}
			return keys[i].sink < keys[j].sink
		})
		for _, k := range keys {
			rf := s.findings[k]
			cur := merged[k]
			if cur == nil {
				cp := *rf
				cp.labels = map[string]bool{}
				for l := range rf.labels {
					cp.labels[l] = true
				}
				merged[k] = &cp
				continue
			}
			for l := range rf.labels {
				cur.labels[l] = true
			}
			if rf.steps.len() < cur.steps.len() {
				cur.steps = rf.steps
				cur.guards = rf.guards
			}
		}
		if fn.Dep {
			continue
		}
		for _, sp := range s.suppressions {
			sp.Position = position(sp.pos)
			sp.Snippet = e.texts.Snippet(sp.pos)
			res.Suppressions = append(res.Suppressions, sp)
		}
		for _, u := range s.uncertain {
			key := u.Kind + "|" + u.Symbol + "|" + u.Func + "|" + posKey(u.pos)
			if seenU[key] {
				continue
			}
			seenU[key] = true
			e.fill(u)
			res.Uncertainties = append(res.Uncertainties, u)
		}
	}
	for _, pe := range e.loadErrors {
		u := newUncertainty(KindMissingSource, pe.Message, pe.Package)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	var limited []*sg.Func
	for fn := range e.guardLimit {
		limited = append(limited, fn)
	}
	sort.Slice(limited, func(i, j int) bool { return limited[i].ID < limited[j].ID })
	for _, fn := range limited {
		u := newUncertainty(KindDepthLimit, "guard_limit: a flow in this function depends on more than "+itoa(maxGuards)+
			" conditions on parameters; it is kept without them, as possible under any condition", "")
		u.Func = e.funcID(fn)
		u.pos = fn.Pos
		e.fill(u)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	var degraded []*sg.Func
	for fn := range e.degraded {
		degraded = append(degraded, fn)
	}
	sort.Slice(degraded, func(i, j int) bool { return degraded[i].ID < degraded[j].ID })
	for _, fn := range degraded {
		u := newUncertainty(KindDepthLimit, "the analysis budget of this function was exhausted; its summary may miss flows", "")
		u.Func = e.funcID(fn)
		u.pos = fn.Pos
		e.fill(u)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	if len(e.notConverged) > 0 {
		u := newUncertainty(KindRecursionApproximation, "the local analysis of these functions did not reach a fixed point", "")
		u.Candidates = append([]string(nil), e.notConverged...)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	switch {
	case st.TimedOut:
		res.Uncertainties = append(res.Uncertainties, newUncertainty(KindDepthLimit,
			"the time limit was reached; functions not analysed yet are missing from the result", ""))
	case !st.Converged:
		res.Uncertainties = append(res.Uncertainties, newUncertainty(KindRecursionApproximation, "the summaries did not stabilise within the pass limit", ""))
	}
	sort.SliceStable(res.Suppressions, func(i, j int) bool {
		a, b := res.Suppressions[i], res.Suppressions[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		return a.Position.Offset < b.Position.Offset
	})
	sort.SliceStable(res.Uncertainties, func(i, j int) bool {
		a, b := res.Uncertainties[i], res.Uncertainties[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		if a.Position.Offset != b.Position.Offset {
			return a.Position.Offset < b.Position.Offset
		}
		return a.Kind < b.Kind
	})

	keys := make([]findingKey, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	for _, k := range keys {
		f := e.buildFinding(merged[k])
		if f != nil {
			res.Findings = append(res.Findings, f)
		}
	}
	sort.SliceStable(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		if a.Sink.Position.File != b.Sink.Position.File {
			return a.Sink.Position.File < b.Sink.Position.File
		}
		if a.Sink.Position.Offset != b.Sink.Position.Offset {
			return a.Sink.Position.Offset < b.Sink.Position.Offset
		}
		return a.ID < b.ID
	})
	return res
}

func (e *Engine) fill(u *Uncertainty) {
	if !u.pos.IsZero() {
		u.Position = position(u.pos)
		u.Snippet = e.texts.Snippet(u.pos)
	}
}

// position converts a position of the IR to the one the output has.
func position(p ir.Pos) ir.Position { return p.Position() }

func (e *Engine) siteRef(s *sites.Site) SiteRef {
	return SiteRef{ID: s.ID, Symbol: s.Symbol, Func: s.Func, Position: s.Position, Snippet: s.Snippet, Fingerprint: s.Fingerprint}
}

func (e *Engine) buildFinding(rf *rawFinding) *Finding {
	src := e.sites.ByID[rf.origin]
	if src == nil || rf.site == nil {
		return nil
	}
	f := &Finding{
		Source:     e.siteRef(src),
		Sink:       e.siteRef(rf.site),
		Category:   rf.site.Category,
		Confidence: "requires_review",
	}
	for l := range rf.labels {
		f.Labels = append(f.Labels, l)
	}
	sort.Strings(f.Labels)

	// Steps without a position (compiler generated hops) tell a reader
	// nothing, unless they carry an uncertainty.
	steps := make([]Step, 0, rf.steps.len())
	for _, st := range rf.steps.flatten(maxPathSteps) {
		if !st.pos.IsZero() || len(st.Notes) > 0 {
			steps = append(steps, st)
		}
	}
	var funcs []string
	seen := map[string]bool{}
	for i := range steps {
		st := &steps[i]
		if !st.pos.IsZero() {
			st.Position = position(st.pos)
			st.Snippet = e.texts.Snippet(st.pos)
		}
		funcs = append(funcs, st.Func)
		for _, n := range st.Notes {
			e.fill(n)
			k := n.Kind + "|" + n.Symbol + "|" + posKey(n.pos)
			if !seen[k] {
				seen[k] = true
				f.Uncertainties = append(f.Uncertainties, n)
			}
		}
	}
	f.Paths = []Path{{Steps: steps}}
	f.ConditionalNodes = e.conditionalNodes(rf.guards)

	// Which models decided something on the path, and who decided them.
	seenModel := map[string]bool{}
	addModel := func(m ModelRef) {
		k := m.Role + "|" + m.Symbol + "|" + m.Origin
		if seenModel[k] {
			return
		}
		seenModel[k] = true
		f.Models = append(f.Models, m)
		if m.Origin == "llm" {
			f.LLMInfluenced = true
		}
	}
	for _, s := range src.Src {
		addModel(*modelRef(src.Symbol, "source", s.Provenance))
	}
	for _, st := range steps {
		if st.Model != nil {
			addModel(*st.Model)
		}
	}
	for _, k := range rf.site.Sink {
		addModel(*modelRef(rf.site.Symbol, "sink", k.Provenance))
	}

	fp := fingerprint.Combine(src.Fingerprint, rf.site.Fingerprint)
	fp[fingerprint.Path] = fingerprint.Hash(append([]string{src.Func}, append(funcs, rf.site.Func)...)...)
	f.Fingerprint = fp
	f.ID = "f1:" + fp[fingerprint.Structural]
	return f
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
