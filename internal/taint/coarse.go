package taint

import (
	"sort"
	"strconv"

	"github.com/yagihash/rivus/internal/sg"
)

// A large strongly connected component is solved with its dependency functions at access path depth 0 (ADR-124, #187).
//
// Before the run, after the components are made (references included: the components that the engine solves), every
// component that has functions of the dependencies, and has N functions or more (Options.SCCCoarsenN; or N functions of
// the dependencies, with SCCCoarsenDeps), has its dependency functions at depth 0 from the start: the same as -dep-k 0
// for them, and the functions of the project in it keep their depth. The decision reads the call graph only, so it does
// not depend on the order of the work, the machine, the load or a time limit; nothing is thrown away, as nothing is
// decided during the solving. The answer is the one of a run that has those dependency functions at depth 0.

// coarseSCC is a component whose dependency functions are solved at depth 0.
type coarseSCC struct {
	id       string // the first function of the component (the smallest ID)
	size     int    // the functions of the component
	funcs    []*sg.Func
	packages []string
	n        int
	counted  string // "functions" or "dependency_functions"
}

// coarsenBySize marks the dependency functions of the large components. It runs once, before the analysis.
func (e *Engine) coarsenBySize(sccs [][]*sg.Func) {
	n := e.opts.SCCCoarsenN
	if n <= 0 {
		return
	}
	counted := "functions"
	if e.opts.SCCCoarsenDeps {
		counted = "dependency_functions"
	}
	for _, scc := range sccs {
		var deps []*sg.Func
		for _, fn := range scc {
			if fn.Dep {
				deps = append(deps, fn)
			}
		}
		count := len(scc)
		if e.opts.SCCCoarsenDeps {
			count = len(deps)
		}
		if len(deps) == 0 || count < n {
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
		c := &coarseSCC{id: e.funcID(scc[0]), size: len(scc), funcs: deps, packages: pkgs, n: n, counted: counted}
		for _, fn := range deps {
			e.coarse[fn] = c
		}
		e.coarseList = append(e.coarseList, c)
	}
}

// sccCoarsenFlagOff is how the wording tells a reader to solve without this: the one place that has the name of the
// flag (the flag itself comes with the change that turns this on by default; then only this line changes).
const sccCoarsenFlagOff = "-scc-coarsen 0"

// sccCoarsenedMeaning is what depth 0 means, for the reader of a finding.
const sccCoarsenedMeaning = "the dependency functions of this group were solved with access paths cut at depth 0, " +
	"that is, what is told apart below the first step of a parameter (another field of the same value) is not told apart, " +
	"so a flow of another field can be mixed in (more flows than there are, never fewer)"

// sccCoarsenedDetail is the note on a hop that applied the summary of a dependency function of a component that is
// solved at depth 0: it names the function and its package (what the step went through), says what depth 0 means, and
// how to check whether the flow is real.
func sccCoarsenedDetail(funcID string) string {
	return "scc_coarsened: this step went through " + funcID + " (package " + UnitOf(funcID) + "), a function of a dependency in a large group of functions that call each other; " +
		sccCoarsenedMeaning + ". To check: read " + funcID + " in " + UnitOf(funcID) +
		" and see whether its input reaches its output, or solve with " + sccCoarsenFlagOff + " and see whether this finding remains"
}

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
			" " + c.counted + " or more), so " + sccCoarsenedMeaning + ". The dependency functions are in candidates. To check a finding that went through one: " +
			"read that function in its package, or solve with " + sccCoarsenFlagOff + " and see whether the finding remains"
		u := newUncertainty(KindDepthLimit, d, "")
		u.Candidates = ids
		out = append(out, u)
	}
	return out
}
