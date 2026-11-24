package taint

import (
	"sort"
	"strconv"

	"github.com/yagihash/rivus/internal/sg"
)

// The budget of a large strongly connected component (ADR-124, #187).
//
// After each round of a large component (engine.go: parallelSCC) the engine counts, in the committed summaries of the
// functions of the dependencies in it, the keys of the param outputs deeper than one step. When the count is over the
// budget, the component is solved again from the start, once, with its dependency functions at access path depth 0
// (the same as -dep-k 0 for them; the functions of the project stay at their depth): what the component had worked
// out until then is thrown away (its summaries, its findings, what its functions wrote to the global state). The
// answer is the one of a run that had the dependency functions of the component at depth 0 from the start. It is not
// undone. The decision reads the committed summaries at the end of a round, so it does not depend on -workers.

// SCCCoarsenBudget is the budget of a large component: the keys of the param outputs deeper than one step (two or
// more), summed over its dependency functions. Zero (the default): no budget. Internal: there is no flag
// (RIVUS_INTERNAL_SCC_COARSEN_BUDGET in cmd/rivus).
type SCCCoarsenBudget = int // see Options.SCCCoarsen

// coarseSCC is a component whose dependency functions are solved at depth 0.
type coarseSCC struct {
	id      string // the first function of the component (the smallest ID)
	size    int
	members []*sg.Func // all the functions of the component
	funcs   []*sg.Func // its dependency functions
	// why it was done
	cause       string // "deep_param_keys" or "forced"
	deep        int    // the count at that round
	budget      int
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

// coarseCount counts the param outputs deeper than one step in the committed summaries.
func (e *Engine) coarseCount(funcs []*sg.Func) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n := 0
	for _, fn := range funcs {
		if sum := e.summaries[fn]; sum != nil {
			for _, o := range sum.Outputs {
				if o.Kind == OutParam && pathDepth(o.Path) >= 2 {
					n++
				}
			}
		}
	}
	return n
}

func depsOf(scc []*sg.Func) []*sg.Func {
	var deps []*sg.Func
	for _, fn := range scc {
		if fn.Dep {
			deps = append(deps, fn)
		}
	}
	return deps
}

// coarsenStep runs at the end of a round of a large component (after the commits). It decides, once, whether the
// component goes to depth 0 and is solved again, and reports whether it did now. In the first pass the component has
// written nothing to the global state yet (what a level writes waits for its end), so what it worked out is thrown
// away here, and the caller starts its rounds again (inline: true): the run is the one that had the dependency
// functions at depth 0 from the start. In a later pass it has written to the global state, which other components
// of the level read at the same time: the caller stops the rounds of this pass, and the summaries and what the
// component wrote are cleared at the end of the level (applyResets), the rounds start again in the next pass.
func (e *Engine) coarsenStep(scc []*sg.Func, pass int) (moved, inline bool) {
	if e.opts.SCCCoarsen <= 0 && e.opts.SCCCoarsenAtRound == 0 {
		return false, false
	}
	deps := depsOf(scc)
	if len(deps) == 0 || e.coarseOf(deps[0]) != nil {
		return false, false
	}
	e.mu.Lock()
	e.sccRounds[scc[0]]++
	round := e.sccRounds[scc[0]]
	e.mu.Unlock()
	deep := e.coarseCount(deps)
	cause := ""
	switch {
	case e.opts.SCCCoarsen > 0 && deep > e.opts.SCCCoarsen:
		cause = "deep_param_keys"
	case e.opts.SCCCoarsenAtRound > 0 && round >= e.opts.SCCCoarsenAtRound:
		cause = "forced"
	default:
		return false, false
	}
	c := e.moveToCoarse(scc, deps, cause, deep, round, pass)
	e.mu.Lock()
	defer e.mu.Unlock()
	if pass == 1 {
		e.discard(c)
		// what its functions wrote in the rounds that are thrown away waits for the end of the level: it goes too
		ids := map[string]bool{}
		for _, fn := range c.members {
			ids[e.funcID(fn)] = true
		}
		keep := e.pendingGlobals[:0]
		for _, w := range e.pendingGlobals {
			if !ids[w.fn] {
				keep = append(keep, w)
			}
		}
		e.pendingGlobals = keep
		in := map[*sg.Func]bool{}
		for _, fn := range c.members {
			in[fn] = true
		}
		kr := e.pendingReaders[:0]
		for _, r := range e.pendingReaders {
			if !in[r.fn] {
				kr = append(kr, r)
			}
		}
		e.pendingReaders = kr
		return true, true
	}
	e.pendingReset = append(e.pendingReset, c)
	return true, false
}

// moveToCoarse registers the component: from now on the dependency functions of it are solved at depth 0.
func (e *Engine) moveToCoarse(scc, deps []*sg.Func, cause string, deep, round, pass int) *coarseSCC {
	pk := map[string]bool{}
	for _, fn := range deps {
		pk[UnitOf(fn.ID)] = true
	}
	var pkgs []string
	for p := range pk {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	c := &coarseSCC{id: e.funcID(scc[0]), size: len(scc), members: scc, funcs: deps, cause: cause, deep: deep,
		budget: e.opts.SCCCoarsen, round: round, pass: pass, packages: pkgs}
	e.mu.Lock()
	for _, fn := range deps {
		e.coarse[fn] = c
	}
	e.coarseList = append(e.coarseList, c)
	e.mu.Unlock()
	return c
}

// coarsenFirst is for the tests (SCCCoarsenAtRound below 0): the component goes to depth 0 before its first round, so
// nothing is thrown away.
func (e *Engine) coarsenFirst(scc []*sg.Func) {
	if e.opts.SCCCoarsenAtRound >= 0 {
		return
	}
	if deps := depsOf(scc); len(deps) > 0 && e.coarseOf(deps[0]) == nil {
		e.moveToCoarse(scc, deps, "forced", 0, 0, 0)
	}
}

// applyResets throws away what the components that were moved to depth 0 in a later pass had worked out (see
// coarsenStep), and what they wrote to the global state (an element that another function wrote stays). It runs at
// the end of a level, with mu held, when no function is being analysed.
func (e *Engine) applyResets() {
	for _, c := range e.pendingReset {
		e.discard(c)
		ids := map[string]bool{}
		for _, fn := range c.members {
			ids[e.funcID(fn)] = true
		}
		e.globals.forget(ids, c.members)
	}
	e.pendingReset = e.pendingReset[:0]
}

// discard restores the engine state that the analysis of the functions of the component wrote (commit, compute):
// the summaries, the findings and suppressions (states), the records of the function (coarsened, guardLimit,
// notConverged, degraded, the work counted: addsTotal, spent), the package variables it wrote that hold nothing
// (heldNothing), and the closures whose write of a global was left out (identityStores), and marks its functions to
// be analysed again. What is not undone: the counts of the work done (stats.reanalyses, passes,
// adds_max_single_run: they say what was done, and the thrown-away rounds were done), the functions outside the
// component that were marked to be analysed again (they are analysed again, as they would be), and the
// diagnostics (-pass-stats, TraceElement). With mu held.
func (e *Engine) discard(c *coarseSCC) {
	ids := map[string]bool{}
	for _, fn := range c.members {
		ids[e.funcID(fn)] = true
	}
	for _, fn := range c.members {
		delete(e.summaries, fn)
		delete(e.states, fn)
		delete(e.coarsened, fn)
		delete(e.guardLimit, fn)
		delete(e.degraded, fn)
		delete(e.addsTotal, fn)
		delete(e.spent, fn)
		e.dirty[fn] = true
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				e.identityStores.Delete(in)
			}
		}
	}
	if len(e.notConverged) > 0 {
		keep := e.notConverged[:0:0]
		for _, id := range e.notConverged {
			if ids[id] {
				delete(e.notConvergedSet, id)
			} else {
				keep = append(keep, id)
			}
		}
		e.notConverged = keep
	}
	e.heldMu.Lock()
	for name, fns := range e.heldNothing {
		for _, fn := range c.members {
			delete(fns, fn)
		}
		if len(fns) == 0 {
			delete(e.heldNothing, name)
		}
	}
	e.heldMu.Unlock()
}

// forget takes away what only the given functions wrote to the global state, and the marks that they read it.
func (g *globalStore) forget(writers map[string]bool, fns []*sg.Func) {
	for v, pm := range g.writerFns {
		for path, em := range pm {
			for el, list := range em {
				keep := list[:0:0]
				for _, w := range list {
					if !writers[w] {
						keep = append(keep, w)
					}
				}
				if len(keep) == len(list) {
					continue
				}
				if len(keep) > 0 {
					em[el] = keep
					continue
				}
				delete(em, el)
				delete(g.cells[v][path], el)
				delete(g.writers[v][path], el)
			}
		}
	}
	for _, m := range g.readers_ {
		for _, fn := range fns {
			delete(m, fn)
		}
	}
}

// sccCoarsenedDetail is the note on a hop that applied the summary of a dependency function of a component that was
// solved again at depth 0: the flow from here on may exist only because of that (more flows than there are, never
// fewer).
const sccCoarsenedDetail = "scc_coarsened: the value went through a function of a dependency in a large group of functions that call each other, which was solved again with the access paths of the dependency functions cut at depth 0 to keep the analysis finite: the flow from here may exist only because of that (more flows than there are, never fewer)"

// coarseRecords are the records of the components that were solved again at depth 0, one each, in the order of the
// first function: where (the functions, the packages), what was over, the count, the round.
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
			" of the dependencies, in " + strconv.Itoa(len(c.packages)) + " packages) was solved again from the start, once, with the access paths of its dependency functions cut at depth 0 (" +
			c.cause + ": " + strconv.Itoa(c.deep) + " keys of param outputs deeper than one step, budget " + strconv.Itoa(c.budget) +
			", after round " + strconv.Itoa(c.round) + " of pass " + strconv.Itoa(c.pass) + "): what is told apart below the first step of a parameter is not told apart there (more flows than there are, never fewer)"
		u := newUncertainty(KindDepthLimit, d, "")
		u.Candidates = ids
		out = append(out, u)
	}
	return out
}
