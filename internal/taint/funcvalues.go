package taint

// Function values, carried by the machinery that carries taint (docs/DESIGN-callouts.md, section 5). This is the
// diagnostic (stage 4, first step): with Options.FuncValues the engine is run once more, and in that run
//   - every place that makes a function value (make_closure, a function used as a value) is a synthetic source: it puts
//     a Real element of the label FuncLabel, whose origin is "fn:<ID of the function>", in the value (funcSeed*);
//   - every call of a function value is a synthetic sink (a site that the engine makes, FuncValueSites): it reads the
//     value that is called (the first operand);
// and the findings that reach those sinks are the candidates of the call: the functions that the value may be. What
// the summaries of the callees say of their parameters (a symbolic element that reaches a sink) is applied at each call
// site, as for any element: the candidates are those of the call, not of all the calls of the function.
// Nothing here changes a result: the findings of this run are not reported (they are dropped, as the findings
// of an origin that no source site has are), and the run is made by the caller of the library after the real one.

import (
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// FuncLabel is the label of the elements that stand for functions; FuncValueSymbol the symbol of the synthetic sinks.
const (
	FuncLabel       = "FUNC"
	FuncValueSymbol = "rivus.func_value_call"
	funcOrigin      = "fn:"
)

// FuncValueSites is the sites of a program with, for every call of a function value, a synthetic sink that reads the
// value that is called. The sites of the rules are the same.
func FuncValueSites(p *sg.Program, st *sites.Result) (*sites.Result, []FuncCall) {
	var calls []FuncCall
	out := &sites.Result{
		Sites:     append([]*sites.Site(nil), st.Sites...),
		ByIRInstr: make(map[sites.IRInstr][]*sites.Site, len(st.ByIRInstr)),
		ByIRParam: st.ByIRParam,
		ByID:      make(map[string]*sites.Site, len(st.ByID)),
	}
	for k, v := range st.ByIRInstr {
		out.ByIRInstr[k] = v
	}
	for k, v := range st.ByID {
		out.ByID[k] = v
	}
	for _, fn := range p.Funcs {
		if fn.IR == nil {
			continue
		}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall || in.Callee == nil || in.Callee.Kind != ir.CalleeValue || len(in.Args) == 0 {
					continue
				}
				s := &sites.Site{
					ID:       "fv:" + string(fn.IR.ID) + ":" + itoa(in.IRBlock) + ":" + itoa(in.IRIndex),
					Role:     sites.RoleSink,
					Kind:     "call",
					Func:     fn.ID,
					Symbol:   FuncValueSymbol,
					Labels:   []string{FuncLabel},
					Position: in.Pos.Position(),
					Sink:     []model.Sink{{Symbol: FuncValueSymbol}},
				}
				k := sites.IRInstr{Func: fn.IR.ID, Block: in.IRBlock, Instr: in.IRIndex}
				out.ByIRInstr[k] = append(out.ByIRInstr[k], s)
				out.ByID[s.ID] = s
				fc := FuncCall{Site: s, Limit: in.Callee.ValueLimit}
				for _, c := range in.Callee.ValueCands {
					fc.ValueCands = append(fc.ValueCands, string(c.ID))
				}
				for _, c := range in.Callee.Candidates {
					fc.Extractor = append(fc.Extractor, string(c))
				}
				sort.Strings(fc.ValueCands)
				sort.Strings(fc.Extractor)
				calls = append(calls, fc)
			}
		}
	}
	return out, calls
}

// FuncCall is a call of a function value, with the candidates that the rest of the analysis has for it: those of
// valuecalls and those that the extractor wrote.
type FuncCall struct {
	Site       *sites.Site
	ValueCands []string
	Extractor  []string
	Limit      string
}

// seedFuncValue puts the function the value stands for in the value: a function used as a value, or a closure made.
// A function of a dependency is not a source (the first version carries the function values that the code of the
// project makes; the cost of the others is measured in docs/DESIGN-callouts.md, 5.4).
func (s *state) seedFuncValue(l Loc, id string, pos ir.Pos, what string) {
	if s.fn.Dep {
		return
	}
	s.add(l, RealElem(FuncLabel, funcOrigin+id), s.origin(pos, what))
}

// funcValueSink reads what a call of a function value calls: the elements that are in the value that is called.
func (s *state) funcValueSink(site *sites.Site, ci *sg.Instr) {
	for _, f := range s.flatten(s.locs(ci.Args[0])) {
		s.reach(site, f, ci, nil, "")
	}
}

// FuncCandidates is, for a call of a function value, the functions that the elements say it may call (sorted), by
// the ID of its synthetic site.
type FuncCandidates struct {
	Site       *sites.Site
	Candidates []string
}

// collectFuncCandidates reads them from the raw findings, and removes them from the findings that are reported.
func (e *Engine) collectFuncCandidates(merged map[findingKey]*rawFinding) []FuncCandidates {
	by := map[string]map[string]bool{}
	site := map[string]*sites.Site{}
	for k, rf := range merged {
		if rf.site == nil || rf.site.Symbol != FuncValueSymbol {
			continue
		}
		delete(merged, k)
		if !strings.HasPrefix(rf.origin, funcOrigin) {
			continue
		}
		if by[rf.site.ID] == nil {
			by[rf.site.ID] = map[string]bool{}
			site[rf.site.ID] = rf.site
		}
		by[rf.site.ID][strings.TrimPrefix(rf.origin, funcOrigin)] = true
	}
	var out []FuncCandidates
	for id, set := range by {
		c := FuncCandidates{Site: site[id]}
		for f := range set {
			c.Candidates = append(c.Candidates, f)
		}
		sort.Strings(c.Candidates)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Site.ID < out[j].Site.ID })
	return out
}
