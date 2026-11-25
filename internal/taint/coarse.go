package taint

import (
	"crypto/sha256"
	"encoding/hex"
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
	counted  string // what N counts: "dependency_functions"
	info     *CoarsenedSCC
}

// CoarsenedSCC is the machine-readable form of a group that is solved at depth 0: the same on the note of the group
// (depth_limit, scc_coarsened) and on each hop that went through one of its dependency functions. A finding lists the
// ID of each group it went through (Finding.ThroughCoarsenedSCC).
type CoarsenedSCC struct {
	// ID is the first 12 hex digits of the SHA-256 of the smallest function ID of the group: the same for the same
	// code, for any run.
	ID string `json:"id"`
	// Packages are the packages of the dependency functions of the group (the first MaxCoarsenedPackages, sorted);
	// PackagesCount is how many there are in all.
	Packages      []string           `json:"packages"`
	PackagesCount int                `json:"packages_count"`
	Functions     CoarsenedFunctions `json:"functions"`
}

// CoarsenedFunctions counts the functions of the group: all of them, and the ones of the dependencies (the ones
// solved at depth 0).
type CoarsenedFunctions struct {
	Group        int `json:"group"`
	Dependencies int `json:"dependencies"`
}

// MaxCoarsenedPackages is how many package names CoarsenedSCC.Packages lists.
const MaxCoarsenedPackages = 10

func coarsenedID(smallest string) string {
	h := sha256.Sum256([]byte(smallest))
	return hex.EncodeToString(h[:])[:12]
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
		smallest := e.funcID(scc[0])
		for _, fn := range scc {
			if id := e.funcID(fn); id < smallest {
				smallest = id
			}
		}
		listed := pkgs
		if len(listed) > MaxCoarsenedPackages {
			listed = listed[:MaxCoarsenedPackages]
		}
		c := &coarseSCC{id: smallest, size: len(scc), funcs: deps, packages: pkgs, n: n, counted: "dependency_functions"}
		c.info = &CoarsenedSCC{ID: coarsenedID(smallest), Packages: append([]string(nil), listed...), PackagesCount: len(pkgs),
			Functions: CoarsenedFunctions{Group: len(scc), Dependencies: len(deps)}}
		for _, fn := range deps {
			e.coarse[fn] = c
		}
		e.coarseList = append(e.coarseList, c)
	}
	return most
}

// sccCoarsenFlagOff is how the wording tells a reader to solve without this: the one place that has the name of the
// flag (-scc-coarsen; 0 turns it off).
const sccCoarsenFlagOff = "-scc-coarsen 0"

// sccCoarsenedMeaning is what depth 0 means, for the reader of a finding.
const sccCoarsenedMeaning = "the dependency functions of this group were solved with access paths cut at depth 0, " +
	"that is, what is told apart below the first step of a parameter (another field of the same value) is not told apart, " +
	"so a flow of another field can be mixed in (more flows than there are, never fewer); " +
	"which fields were told apart before the cut is not kept in the summary, and is found by reading the body of the function"

// sccCoarsenedLook is the list of what to look at to tell a real flow from a mixed one.
const sccCoarsenedLook = "the conditions of branches (a type test such as `is`, or an `if`, in the dependency), " +
	"where a function value is passed (which argument its caller gives it, and what), " +
	"and which fields are written and which are read"

// sccCoarsenedDetail is the note on a hop that applied the summary of a dependency function of a component that is
// solved at depth 0: it names the function and its package (what the step went through), says what depth 0 means, and
// how to check whether the flow is real.
func sccCoarsenedDetail(funcID string) string {
	return "scc_coarsened: this step went through " + funcID + " (package " + UnitOf(funcID) + "), a function of a dependency in a large group of functions that call each other; " +
		sccCoarsenedMeaning + ". To check: read the body of " + funcID + " in " + UnitOf(funcID) +
		" to see which fields it tells apart, and whether its input reaches its output. What to look at: " + sccCoarsenedLook +
		". Or solve with " + sccCoarsenFlagOff + " and see whether this finding remains"
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
			"read that function in its package (look at: " + sccCoarsenedLook + "), or solve with " + sccCoarsenFlagOff + " and see whether the finding remains"
		u := newUncertainty(KindDepthLimit, d, "")
		u.CoarsenedSCC = c.info
		u.Candidates = ids
		out = append(out, u)
	}
	return out
}
