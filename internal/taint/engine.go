package taint

import (
	"sort"
	"time"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// Graph resolves the possible callees of calls whose target is not static
// (interface methods and function values).
type Graph interface {
	Callees(ci ssa.CallInstruction) []*ssa.Function
}

// Options tunes the analysis.
type Options struct {
	// K is the maximum access path depth (ADR-014).
	K int
	// MaxPasses bounds the number of times the whole program is revisited
	// while summaries change.
	MaxPasses int
}

// DefaultOptions are the defaults.
func DefaultOptions() Options { return Options{K: 3, MaxPasses: 12} }

// Engine runs the whole-program analysis.
type Engine struct {
	prog  *gossa.Program
	sites *sites.Result
	rules *model.Rules
	graph Graph
	opts  Options

	summaries map[*ssa.Function]*Summary
	states    map[*ssa.Function]*state
	globals   *globalStore

	notConverged []string
}

// New creates an engine. graph may be nil: calls through interfaces and
// function values are then reported as unresolved.
func New(p *gossa.Program, st *sites.Result, rules *model.Rules, graph Graph, opts Options) *Engine {
	if opts.K <= 0 {
		opts.K = DefaultOptions().K
	}
	if opts.MaxPasses <= 0 {
		opts.MaxPasses = DefaultOptions().MaxPasses
	}
	return &Engine{
		prog: p, sites: st, rules: rules, graph: graph, opts: opts,
		summaries: map[*ssa.Function]*Summary{},
		states:    map[*ssa.Function]*state{},
		globals:   newGlobalStore(),
	}
}

func (e *Engine) funcID(fn *ssa.Function) string { return gossa.FuncID(fn) }

func (e *Engine) hasBody(fn *ssa.Function) bool { return fn != nil && len(fn.Blocks) > 0 }

func (e *Engine) noteNotConverged(fn *ssa.Function) {
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
}

// Result is the outcome of a run.
type Result struct {
	Findings      []*Finding     `json:"findings"`
	Uncertainties []*Uncertainty `json:"uncertainties"`
	Stats         Stats          `json:"stats"`
}

// Run analyses every function with a body, callees before callers, until the
// summaries stop changing.
func (e *Engine) Run() *Result {
	start := time.Now()
	fns := e.prog.Funcs
	callees := func(ci ssa.CallInstruction) []*ssa.Function {
		if fn := ci.Common().StaticCallee(); fn != nil {
			return []*ssa.Function{fn}
		}
		if e.graph != nil {
			return e.graph.Callees(ci)
		}
		return nil
	}
	sccs, callers := order(fns, callees)

	dirty := map[*ssa.Function]bool{}
	for _, fn := range fns {
		dirty[fn] = true
	}
	var st Stats
	st.Functions = len(fns)
	for pass := 0; pass < e.opts.MaxPasses; pass++ {
		st.Passes = pass + 1
		any := false
		for _, scc := range sccs {
			for iter := 0; iter < 8; iter++ {
				progress := false
				for _, fn := range scc {
					if !dirty[fn] {
						continue
					}
					dirty[fn] = false
					any = true
					st.Reanalyses++
					if e.analyzeFunc(fn, dirty, callers) {
						progress = true
					}
				}
				if !progress || len(scc) == 1 && !selfCalls(scc[0], callees) {
					break
				}
			}
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
	st.DurationMilli = time.Since(start).Milliseconds()
	return e.collect(st)
}

func (e *Engine) anyDirty(dirty map[*ssa.Function]bool) bool {
	for _, d := range dirty {
		if d {
			return true
		}
	}
	return false
}

func selfCalls(fn *ssa.Function, callees func(ssa.CallInstruction) []*ssa.Function) bool {
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			if ci, ok := in.(ssa.CallInstruction); ok {
				for _, c := range callees(ci) {
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
func (e *Engine) analyzeFunc(fn *ssa.Function, dirty map[*ssa.Function]bool, callers map[*ssa.Function][]*ssa.Function) bool {
	s := &state{
		eng: e, fn: fn,
		cells:       map[Root]map[string]Set{},
		whys:        map[ref]*why{},
		paramRoot:   map[Root]int{},
		findings:    map[findingKey]*rawFinding{},
		sinkReaches: map[sinkKey]*SinkReach{},
	}
	s.analyze()
	sum := s.summarize()
	e.states[fn] = s

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
		g, ok := r.V.(*ssa.Global)
		if !ok {
			continue
		}
		for path, set := range cm {
			for el := range set {
				if el.Kind != Real {
					continue
				}
				if _, produced := s.whys[ref{Loc{r, path}, el}]; !produced {
					continue
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
		if g, ok := r.V.(*ssa.Global); ok {
			e.globals.noteReader(g, fn)
		}
	}
	return changed
}

// order computes the strongly connected components of the call graph with
// callees before callers, and the reverse edges.
func order(fns []*ssa.Function, callees func(ssa.CallInstruction) []*ssa.Function) ([][]*ssa.Function, map[*ssa.Function][]*ssa.Function) {
	inSet := map[*ssa.Function]bool{}
	for _, fn := range fns {
		inSet[fn] = true
	}
	succ := map[*ssa.Function][]*ssa.Function{}
	callers := map[*ssa.Function][]*ssa.Function{}
	for _, fn := range fns {
		seen := map[*ssa.Function]bool{}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				ci, ok := in.(ssa.CallInstruction)
				if !ok {
					continue
				}
				for _, c := range callees(ci) {
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
	index := map[*ssa.Function]int{}
	low := map[*ssa.Function]int{}
	on := map[*ssa.Function]bool{}
	var stack []*ssa.Function
	var sccs [][]*ssa.Function
	n := 0
	var visit func(v *ssa.Function)
	visit = func(v *ssa.Function) {
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
			var comp []*ssa.Function
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sort.Slice(comp, func(i, j int) bool { return gossa.FuncID(comp[i]) < gossa.FuncID(comp[j]) })
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
	cells    map[*ssa.Global]map[string]Set
	readers_ map[*ssa.Global]map[*ssa.Function]bool
}

func newGlobalStore() *globalStore {
	return &globalStore{cells: map[*ssa.Global]map[string]Set{}, readers_: map[*ssa.Global]map[*ssa.Function]bool{}}
}

func (g *globalStore) cellsOf(v *ssa.Global) map[string]Set { return g.cells[v] }

func (g *globalStore) merge(v *ssa.Global, path string, e Elem) bool {
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
	set[e] = struct{}{}
	return true
}

func (g *globalStore) noteReader(v *ssa.Global, fn *ssa.Function) {
	m := g.readers_[v]
	if m == nil {
		m = map[*ssa.Function]bool{}
		g.readers_[v] = m
	}
	m[fn] = true
}

func (g *globalStore) readers(v *ssa.Global) []*ssa.Function {
	var out []*ssa.Function
	for fn := range g.readers_[v] {
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return gossa.FuncID(out[i]) < gossa.FuncID(out[j]) })
	return out
}

// --- results ---

func (e *Engine) collect(st Stats) *Result {
	res := &Result{Stats: st, Findings: []*Finding{}, Uncertainties: []*Uncertainty{}}

	type best struct {
		rf *rawFinding
	}
	merged := map[findingKey]*rawFinding{}
	fnList := make([]*ssa.Function, 0, len(e.states))
	for fn := range e.states {
		fnList = append(fnList, fn)
	}
	sort.Slice(fnList, func(i, j int) bool { return gossa.FuncID(fnList[i]) < gossa.FuncID(fnList[j]) })

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
			if len(rf.steps) < len(cur.steps) {
				cur.steps = rf.steps
			}
		}
		if !e.prog.Own[fn] {
			continue
		}
		for _, u := range s.uncertain {
			key := u.Kind + "|" + u.Symbol + "|" + u.Func + "|" + itoa(int(u.pos))
			if seenU[key] {
				continue
			}
			seenU[key] = true
			e.fill(u)
			res.Uncertainties = append(res.Uncertainties, u)
		}
	}
	for _, pe := range e.prog.LoadErrors {
		u := newUncertainty(KindMissingSource, pe.Message, pe.Package)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	if len(e.notConverged) > 0 {
		u := newUncertainty(KindRecursionApproximation, "the local analysis of these functions did not reach a fixed point", "")
		u.Candidates = append([]string(nil), e.notConverged...)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	if !st.Converged {
		res.Uncertainties = append(res.Uncertainties, newUncertainty(KindRecursionApproximation, "the summaries did not stabilise within the pass limit", ""))
	}
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
	if u.pos.IsValid() {
		u.Position = e.prog.Position(u.pos)
		u.Snippet = e.prog.Snippet(u.pos)
	}
}

func (e *Engine) siteRef(s *sites.Site) SiteRef {
	return SiteRef{ID: s.ID, Symbol: s.Symbol, Func: s.Func, Position: s.Position, Snippet: s.Snippet}
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

	steps := make([]Step, len(rf.steps))
	copy(steps, rf.steps)
	var funcs []string
	seen := map[string]bool{}
	for i := range steps {
		st := &steps[i]
		if st.pos.IsValid() {
			st.Position = e.prog.Position(st.pos)
			st.Snippet = e.prog.Snippet(st.pos)
		}
		funcs = append(funcs, st.Func)
		for _, n := range st.Notes {
			e.fill(n)
			k := n.Kind + "|" + n.Symbol + "|" + itoa(int(n.pos))
			if !seen[k] {
				seen[k] = true
				f.Uncertainties = append(f.Uncertainties, n)
			}
		}
	}
	f.Paths = []Path{{Steps: steps}}

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
