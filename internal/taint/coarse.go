package taint

import (
	"sort"
	"strconv"

	"github.com/yagihash/rivus/internal/sg"
)

// A large strongly connected component is solved with its dependency functions at access path depth 0 (ADR-124, #187).
//
// Before the run, after the components are made (references included: the components that the engine solves), every
// component that has Options.SCCCoarsenN functions of the dependencies or more (-scc-coarsen) has them at depth 0 from
// the start: the same as -dep-k 0 for them, and the functions of the project in it keep their depth. The decision
// reads the call graph only, so it does not depend on the order of the work, the machine, the load or a time limit;
// nothing is thrown away, as nothing is decided during the solving. The answer is the one of a run that has those
// dependency functions at depth 0.

// coarseSCC is a component whose dependency functions are solved at depth 0.
type coarseSCC struct {
	id       string // the first function of the component (the smallest ID)
	size     int    // the functions of the component
	funcs    []*sg.Func
	packages []string
	n        int
}

// coarsenBySize marks the dependency functions of the large components, and returns the most dependency functions that
// one component has. It runs once, before the analysis.
func (e *Engine) coarsenBySize(sccs [][]*sg.Func) int {
	most := 0
	for _, scc := range sccs {
		var deps []*sg.Func
		for _, fn := range scc {
			if fn.Dep {
				deps = append(deps, fn)
			}
		}
		if len(deps) > most {
			most = len(deps)
		}
		n := e.opts.SCCCoarsenN
		if n <= 0 || len(deps) < n {
			continue
		}
		pk := map[string]bool{}
		for _, fn := range deps {
			pk[UnitOf(fn.ID)] = true
		}
		var pkgs []string
		for p := range pk {
			pkgs = append(pkgs, p)
		}
		sort.Strings(pkgs)
		c := &coarseSCC{id: e.funcID(scc[0]), size: len(scc), funcs: deps, packages: pkgs, n: n}
		for _, fn := range deps {
			e.coarse[fn] = c
		}
		e.coarseList = append(e.coarseList, c)
	}
	return most
}

// sccCoarsenedDetail is the note on a hop that applied the summary of a dependency function of a component that is
// solved at depth 0: the flow from here on may exist only because of that (more flows than there are, never fewer).
const sccCoarsenedDetail = "scc_coarsened: the value went through a function of a dependency in a large group of functions that call each other, which was solved with the access paths of the dependency functions cut at depth 0 to keep the analysis finite: the flow from here may exist only because of that (more flows than there are, never fewer)"

// coarseRecords are the records of the components that are solved at depth 0, one each, in the order of the first
// function: where (the functions, the packages) and how large.
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
			" of the dependencies, in " + strconv.Itoa(len(c.packages)) + " packages) is large (" + strconv.Itoa(c.n) +
			" dependency functions or more), so the access paths of its dependency functions were cut at depth 0 from the start: " +
			"what is told apart below the first step of a parameter is not told apart there (more flows than there are, never fewer)"
		u := newUncertainty(KindDepthLimit, d, "")
		u.Candidates = ids
		out = append(out, u)
	}
	return out
}
