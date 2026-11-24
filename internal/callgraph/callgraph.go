// Package callgraph builds the call graph used to resolve calls through
// interfaces and function values.
//
// The builder is an interface so that every language can bring its own
// implementation (ADR-018): Go uses CHA refined by VTA, Dart will use the
// type flow analysis of the Dart VM, and so on. Go specific algorithms must
// not leak out of this package.
package callgraph

import (
	"sort"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/yagihash/rivus/internal/frontend/gossa"
)

// Graph answers who a call may call.
type Graph interface {
	// Callees returns the possible callees of the call, in a stable order.
	Callees(ci ssa.CallInstruction) []*ssa.Function
}

// BuildGoOn builds the call graph of a go/ssa program: the class hierarchy
// analysis gives a sound but coarse graph, which variable type analysis then
// narrows down. The extraction asks it of the lifted program only
// (goir.LiftedTable).
func BuildGoOn(prog *ssa.Program) Graph {
	initial := cha.CallGraph(prog)
	funcs := ssautil.AllFunctions(prog)
	g := vta.CallGraph(funcs, initial)
	return newGraph(g)
}

type graph struct {
	callees map[ssa.CallInstruction][]*ssa.Function
}

func newGraph(g *callgraph.Graph) *graph {
	out := &graph{callees: map[ssa.CallInstruction][]*ssa.Function{}}
	for _, n := range g.Nodes {
		for _, e := range n.Out {
			if e.Site == nil || e.Callee == nil || e.Callee.Func == nil {
				continue
			}
			out.callees[e.Site] = append(out.callees[e.Site], e.Callee.Func)
		}
	}
	for site, fns := range out.callees {
		sort.Slice(fns, func(i, j int) bool { return gossa.FuncID(fns[i]) < gossa.FuncID(fns[j]) })
		out.callees[site] = dedupe(fns)
	}
	return out
}

func dedupe(fns []*ssa.Function) []*ssa.Function {
	out := fns[:0]
	var last *ssa.Function
	for _, f := range fns {
		if f != last {
			out = append(out, f)
		}
		last = f
	}
	return out
}

func (g *graph) Callees(ci ssa.CallInstruction) []*ssa.Function { return g.callees[ci] }
