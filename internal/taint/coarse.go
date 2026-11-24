package taint

import (
	"sort"
	"strconv"

	"github.com/yagihash/rivus/internal/sg"
)

// The budget of a large strongly connected component (ADR-124, #187).
//
// After each round of a large component (engine.go: parallelSCC) the engine counts, in the committed summaries of the
// functions of the dependencies in it, (a) the keys of the param outputs below depth 1 and (b) the keys of the global
// outputs. When one of them is over its budget, the dependency functions of the component move, once, to a coarser
// form of the summary (foldSummary): the access paths of the outputs are cut at depth 0, and each function writes what
// it writes to any global to every global that the component writes. The move is not undone. The functions of the
// project are not touched.
//
// The decision reads counts that do not shrink while the component is solved (keys of the committed summaries; the
// widening of the elements does not change the number of outputs), at the end of a round, where the summaries are the
// same for any number of workers. The fold is a closure upwards (it only adds flows; applying it twice gives the same
// summary), so the summaries after a round at which it is applied are above those of a run that applied it earlier or
// not at all.

// SCCCoarsenBudget is the budget of the keys of one component. A zero is no budget on that count; both zero: no
// coarsening. Internal: there is no flag (RIVUS_INTERNAL_SCC_COARSEN_BUDGET in cmd/rivus).
type SCCCoarsenBudget struct {
	// DeepParamKeys: the outputs of kind param whose path is deeper than one step (two or more), summed over the
	// dependency functions of the component.
	DeepParamKeys int
	// GlobalKeys: the outputs of kind global, summed over the dependency functions of the component.
	GlobalKeys int
}

func (b SCCCoarsenBudget) on() bool { return b.DeepParamKeys > 0 || b.GlobalKeys > 0 }

// coarseSCC is a component whose dependency functions were moved to the coarse form.
type coarseSCC struct {
	id      string // the first function of the component (the smallest ID)
	size    int    // the functions of the component, all of them
	funcs   []*sg.Func
	globals map[string]*sg.Value // the globals that the dependency functions write, by name
	names   []string             // the names of globals, sorted
	// why it was done
	cause       string // "deep_param_keys", "global_keys" or "forced"
	deep, glob  int    // the counts at that round
	budget      SCCCoarsenBudget
	round, pass int // the round of the component (counted over the passes) and the pass
	packages    []string
}

// coarseOf is the coarse component the function is in, or nil.
func (e *Engine) coarseOf(fn *sg.Func) *coarseSCC {
	if !fn.Dep {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.coarse[fn]
}

// coarseCounts counts the two quantities of the budget in the committed summaries.
func (e *Engine) coarseCounts(funcs []*sg.Func) (deep, glob int) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, fn := range funcs {
		sum := e.summaries[fn]
		if sum == nil {
			continue
		}
		for _, o := range sum.Outputs {
			switch {
			case o.Kind == OutParam && pathDepth(o.Path) >= 2:
				deep++
			case o.Kind == OutGlobal:
				glob++
			}
		}
	}
	return deep, glob
}

// coarsenStep runs at the end of a round of a large component (after the commits). It decides, once, whether the
// component is moved to the coarse form, and when it is (now or before), brings the summaries of its dependency
// functions to it. It reports whether a summary changed.
func (e *Engine) coarsenStep(scc []*sg.Func, callers map[*sg.Func][]*sg.Func, pass int) bool {
	if !e.opts.SCCCoarsen.on() && e.opts.SCCCoarsenAtRound == 0 {
		return false
	}
	var deps []*sg.Func
	for _, fn := range scc {
		if fn.Dep {
			deps = append(deps, fn)
		}
	}
	if len(deps) == 0 {
		return false
	}
	c := e.coarseOf(deps[0])
	if c == nil {
		e.mu.Lock()
		e.sccRounds[scc[0]]++
		round := e.sccRounds[scc[0]]
		e.mu.Unlock()
		deep, glob := e.coarseCounts(deps)
		b := e.opts.SCCCoarsen
		cause := ""
		switch {
		case b.DeepParamKeys > 0 && deep > b.DeepParamKeys:
			cause = "deep_param_keys"
		case b.GlobalKeys > 0 && glob > b.GlobalKeys:
			cause = "global_keys"
		case e.opts.SCCCoarsenAtRound > 0 && round >= e.opts.SCCCoarsenAtRound:
			cause = "forced"
		default:
			return false
		}
		c = e.moveToCoarse(scc, deps, cause, deep, glob, round, pass)
	}
	return e.refold(c, callers)
}

// moveToCoarse registers the component as coarse: from now on the summaries of its dependency functions are folded.
func (e *Engine) moveToCoarse(scc, deps []*sg.Func, cause string, deep, glob, round, pass int) *coarseSCC {
	pk := map[string]bool{}
	for _, fn := range deps {
		pk[UnitOf(fn.ID)] = true
	}
	var pkgs []string
	for p := range pk {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	c := &coarseSCC{id: e.funcID(scc[0]), size: len(scc), funcs: deps, cause: cause, deep: deep, glob: glob, budget: e.opts.SCCCoarsen,
		round: round, pass: pass, packages: pkgs}
	e.mu.Lock()
	for _, fn := range deps {
		e.coarse[fn] = c
	}
	e.coarseList = append(e.coarseList, c)
	e.mu.Unlock()
	return c
}

// coarsenFirst is for the tests (SCCCoarsenAtRound below 0): the component is moved before its first round.
func (e *Engine) coarsenFirst(scc []*sg.Func) {
	if e.opts.SCCCoarsenAtRound >= 0 {
		return
	}
	var deps []*sg.Func
	for _, fn := range scc {
		if fn.Dep {
			deps = append(deps, fn)
		}
	}
	if len(deps) > 0 && e.coarseOf(deps[0]) == nil {
		e.moveToCoarse(scc, deps, "forced", 0, 0, 0, 0)
	}
}

// refold takes the globals that the component writes now, and folds the summary of every dependency function of it.
func (e *Engine) refold(c *coarseSCC, callers map[*sg.Func][]*sg.Func) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	globals := map[string]*sg.Value{}
	for _, fn := range c.funcs {
		if sum := e.summaries[fn]; sum != nil {
			for _, o := range sum.Outputs {
				if o.Kind == OutGlobal && o.Global != nil {
					if _, ok := globals[o.Global.Name]; !ok {
						globals[o.Global.Name] = o.Global
					}
				}
			}
		}
	}
	c.globals = globals
	c.names = c.names[:0]
	for n := range globals {
		c.names = append(c.names, n)
	}
	sort.Strings(c.names)
	changed := false
	for _, fn := range c.funcs {
		sum := e.summaries[fn]
		if sum == nil {
			continue
		}
		nsum := foldSummary(sum, c)
		if nsum.Sig == sum.Sig {
			e.summaries[fn] = nsum
			continue
		}
		e.notePassChange(fn, sum, nsum)
		e.summaries[fn] = nsum
		changed = true
		for _, cl := range callers[fn] {
			e.dirty[cl] = true
		}
		e.dirty[fn] = true
	}
	return changed
}

// foldSummary is the coarse form of the summary of a dependency function of a coarse component: every output has the
// path "" (an element that stands for a part of a parameter stands for the whole of it), and what the function writes
// to any global it writes to every global that the component writes (with its own elements, never with those of
// another function: they speak of the parameters of that one). The result has the elements of the summary and more.
func foldSummary(sum *Summary, c *coarseSCC) *Summary {
	type okey struct {
		kind  string
		index int
		glob  string
	}
	groups := map[okey]map[Elem]OutElem{}
	var keys []okey
	widened := map[okey]bool{}
	globs := map[string]*sg.Value{}
	for n, g := range c.globals {
		globs[n] = g
	}
	own := map[Elem]OutElem{}
	add := func(m map[Elem]OutElem, oe OutElem, self int) {
		oe.Guards = nil // the conditions are dropped: where two flows with different conditions meet, the fold would otherwise depend on which came first
		if oe.E.Kind == Param {
			oe.E = oe.E.WithPath("")
			if int(oe.E.Param) == self && oe.E.Mods() == "" {
				return // the parameter's own taint in itself: no flow (summarize leaves it out as well)
			}
		}
		have, ok := m[oe.E]
		switch {
		case !ok:
			m[oe.E] = oe
		case oe.Steps.len() < have.Steps.len():
			m[oe.E] = oe
		}
	}
	for _, o := range sum.Outputs {
		if o.Kind == OutGlobal {
			if o.Global != nil {
				if _, ok := globs[o.Global.Name]; !ok {
					globs[o.Global.Name] = o.Global
				}
			}
			for _, oe := range o.Elems {
				add(own, oe, -1)
			}
			continue
		}
		k := okey{o.Kind, o.Index, ""}
		m := groups[k]
		if m == nil {
			m = map[Elem]OutElem{}
			groups[k] = m
			keys = append(keys, k)
		}
		if o.Widened {
			widened[k] = true
		}
		for _, oe := range o.Elems {
			add(m, oe, selfOf(o))
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind < keys[j].kind
		}
		return keys[i].index < keys[j].index
	})
	out := *sum
	out.Outputs = nil
	list := func(m map[Elem]OutElem) []OutElem {
		l := make([]OutElem, 0, len(m))
		for _, oe := range m {
			l = append(l, oe)
		}
		sort.Slice(l, func(i, j int) bool { return lessElem(l[i].E, l[j].E) })
		return l
	}
	for _, k := range keys {
		out.Outputs = append(out.Outputs, Output{Kind: k.kind, Index: k.index, Path: "", Elems: list(groups[k]), Widened: widened[k]})
	}
	if len(own) > 0 {
		names := make([]string, 0, len(globs))
		for n := range globs {
			names = append(names, n)
		}
		sort.Strings(names)
		shared := list(own) // the same list for every global: it is not changed
		for _, n := range names {
			out.Outputs = append(out.Outputs, Output{Kind: OutGlobal, Global: globs[n], Path: "", Elems: shared})
		}
	}
	// the sinks the function reaches: the element of a sink has its path cut as well (and reads the whole of it)
	sinks := map[sinkKey]*SinkReach{}
	var skeys []sinkKey
	for _, sr := range sum.Sinks {
		n := *sr
		n.Guards = nil
		if n.Elem.Kind == Param && n.Elem.Path() != "" {
			n.Elem = n.Elem.WithPath("")
			n.Field = ""
		}
		k := sinkKey{n.Elem, n.Site, n.Field, n.Unsure}
		if have, ok := sinks[k]; ok {
			sinks[k] = mergeSinkReach(have, &n)
		} else {
			sinks[k] = &n
			skeys = append(skeys, k)
		}
	}
	sort.Slice(skeys, func(i, j int) bool { return lessSinkKey(skeys[i], skeys[j]) })
	out.Sinks = nil
	for _, k := range skeys {
		out.Sinks = append(out.Sinks, sinks[k])
	}
	out.Coarse = c
	out.Sig = out.signature()
	return &out
}

// sccCoarsenedDetail is the note on a hop that applied the summary of a dependency function that was moved to the
// coarse form: the flow from there on may exist only because of that (more flows than there are, never fewer).
const sccCoarsenedDetail = "scc_coarsened: the value went through a function of a dependency in a large group of functions that call each other, whose summary was made coarser to keep the analysis finite (paths cut at depth 0, what a function writes to a global variable given to every global that the group writes): the flow from here may exist only because of that (more flows than there are, never fewer)"

// coarseRecords are the records of the components that were moved to the coarse form, one each, in the order of
// the first function: where (the functions, the packages), which count was over, the counts, the round.
func (e *Engine) coarseRecords() []*Uncertainty {
	cs := append([]*coarseSCC(nil), e.coarseList...)
	sort.Slice(cs, func(i, j int) bool { return cs[i].id < cs[j].id })
	var out []*Uncertainty
	for _, c := range cs {
		ids := make([]string, 0, len(c.funcs))
		for _, fn := range c.funcs {
			ids = append(ids, e.funcID(fn))
		}
		sort.Strings(ids)
		d := "scc_coarsened: a group of " + strconv.Itoa(c.size) + " functions that call each other (" + strconv.Itoa(len(c.funcs)) +
			" of the dependencies, in " + strconv.Itoa(len(c.packages)) + " packages) went over its budget (" + c.cause +
			": " + strconv.Itoa(c.deep) + " keys of param outputs deeper than one step, " + strconv.Itoa(c.glob) + " keys of global outputs; budget " +
			strconv.Itoa(c.budget.DeepParamKeys) + " and " + strconv.Itoa(c.budget.GlobalKeys) + ") after round " + strconv.Itoa(c.round) +
			" (pass " + strconv.Itoa(c.pass) + "); the summaries of its dependency functions were made coarser, once (paths cut at depth 0, " +
			"what a function writes to a global variable given to every global that the group writes): more flows than there are, never fewer"
		u := newUncertainty(KindDepthLimit, d, "")
		u.Candidates = ids
		out = append(out, u)
	}
	return out
}

// selfOf is the parameter whose own taint an element of the output is no flow for (the output of that parameter), or -1.
func selfOf(o Output) int {
	if o.Kind == OutParam {
		return o.Index
	}
	return -1
}
