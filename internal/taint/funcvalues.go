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
	"sync"
	"time"

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
				fc := FuncCall{Site: s, Limit: in.Callee.ValueLimit, Fn: fn, Instr: in}
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
	Site *sites.Site
	// Fn and Instr are the function that holds the call and the call, in the graph (the call is the one that
	// -resolve-func-values gives candidates to).
	Fn         *sg.Func
	Instr      *sg.Instr
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

// ZeroCandidate is true for a call that nothing resolved: no candidate from valuecalls or the extractor, and no limit of
// rivus' own was hit (the definition of TestZeroCandidateCalls in internal/valuecalls).
func (c *FuncCall) ZeroCandidate() bool {
	cl := c.Instr.Callee
	return len(cl.ValueCands) == 0 && len(cl.Candidates) == 0 && cl.ValueLimit == ""
}

// supplementNotes are the uncertainties of a call whose candidates the analysis of the function values gave: the call
// keeps the uncertainty that its callee is not known to be only these (unresolved_callee), and, when the rounds were cut
// while the candidates still grew, says so (depth_limit, on this call only).
func (s *state) supplementNotes(ci *sg.Instr) []*Uncertainty {
	var out []*Uncertainty
	if ci.Callee == nil || ci.Callee.Kind != ir.CalleeValue {
		return nil
	}
	if d := ci.Callee.ValueSupplementedDetail(); d != "" {
		u := s.newNote(ci, KindUnresolvedCallee, d, "")
		for _, c := range ci.Callee.ValueCands {
			u.Candidates = append(u.Candidates, string(c.ID))
		}
		sort.Strings(u.Candidates)
		out = append(out, u)
	}
	if d := ci.Callee.ValueRoundsDetail(); d != "" {
		out = append(out, s.newNote(ci, KindDepthLimit, d, ""))
	}
	return out
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

// FuncValuesStats is what -resolve-func-values did (Stats.FuncValues; absent when the flag is off).
type FuncValuesStats struct {
	// ZeroBefore and ZeroAfter are the calls of function values that had no candidate before the rounds and after them.
	ZeroBefore int `json:"zero_candidate_calls_before"`
	ZeroAfter  int `json:"zero_candidate_calls_after"`
	// Supplemented are the calls that were given candidates, and Limited those that got more than ir.ValueCandidateMost
	// (none is followed; they are not in Supplemented).
	Supplemented int `json:"supplemented_calls"`
	Limited      int `json:"candidate_limit_calls,omitempty"`
	// Rounds is the number of runs of the analysis of the function values (0 when it was skipped). RoundsCut is the
	// calls whose candidates were still growing in the last round when the limit was reached.
	Rounds    int  `json:"rounds"`
	RoundsCut int  `json:"rounds_cut,omitempty"`
	Skipped   bool `json:"skipped"`
	// Violations counts the rounds in which a candidate of a call was gone (a defect: the candidates only grow).
	Violations int `json:"monotone_violations,omitempty"`
	// TimedOut is true when the time limit of the run (-timeout) was reached in a round: the candidates of that round
	// are not used (a round that did not finish may have found only some of them), the rounds before it were whole and
	// are kept, and the calls that have candidates from them say that the rounds were cut. The limit is shared by all
	// the rounds and the taint analysis after them.
	TimedOut bool `json:"timed_out,omitempty"`
}

// roundIncomplete says that a run of the analysis was stopped by the time limit of the run: functions were left
// unanalysed or analysed in part, so the candidates it carried are not all that it would have carried.
func roundIncomplete(res *Result) bool {
	return res.Stats.TimedOut || res.Stats.RunTimeouts > 0
}

// FuncValueRounds is the most runs of the analysis of the function values that -resolve-func-values makes. Provisional:
// the number of rounds that the benchmarks need is measured (docs/DESIGN-callouts.md, 5.6) and the
// limit is chosen from that.
const FuncValueRounds = 4

// ResolveFuncValues gives the calls of function values that have no candidate the functions that the analysis of the
// function values carries to them, before the taint analysis (docs/DESIGN-callouts.md, 5.6). It writes into the graph:
// only the calls that had no candidate are changed (Callee.ValueCands, Instr.CandidateFns).
//
// One run of the analysis with Options.FuncValues says, for each of those calls, the functions that reach it; a call that
// got candidates lets the functions go on, so the run is repeated until no call gets a new candidate, at most
// FuncValueRounds times. When the last run still gave a call new candidates, that call says so (ValueRoundsLimit).
//
// When no call of a function value in what was read (dependencies included: they are where a library calls what the code
// gave it) has no candidate, nothing is run: the candidates are used only for the calls that have none, so the result
// would be the same.
func ResolveFuncValues(graph *sg.Program, texts ir.Texts, st *sites.Result, rules *model.Rules, opts Options, loadErrors []ir.LoadError) *FuncValuesStats {
	return ResolveFuncValuesRounds(graph, texts, st, rules, opts, loadErrors, FuncValueRounds)
}

// ResolveFuncValuesRounds is ResolveFuncValues with the limit of the rounds given (the tests cut them short).
func ResolveFuncValuesRounds(graph *sg.Program, texts ir.Texts, st *sites.Result, rules *model.Rules, opts Options, loadErrors []ir.LoadError, rounds int) *FuncValuesStats {
	st2, calls := FuncValueSites(graph, st)
	var zero []*FuncCall
	for i := range calls {
		if calls[i].ZeroCandidate() {
			zero = append(zero, &calls[i])
		}
	}
	out := &FuncValuesStats{ZeroBefore: len(zero), ZeroAfter: len(zero)}
	if len(zero) == 0 {
		out.Skipped = true
		return out
	}
	opts.FuncValues = true
	opts.PassStats, opts.TraceElement, opts.SlowFunc = false, "", nil
	// opts.Timeout is the limit of all the rounds together (and of the taint analysis after them: the caller gives it
	// what is left): each round gets what is left of it, not the whole.
	total, begun := opts.Timeout, time.Now()
	have := make([][]string, len(zero))
	limited := make([]bool, len(zero))
	grew := make([]bool, len(zero))
	for round := 0; round < rounds; round++ {
		if total > 0 {
			opts.Timeout = total - time.Since(begun)
			if opts.Timeout <= 0 {
				out.TimedOut = true
				break
			}
		}
		eng := New(graph, texts, st2, rules, opts)
		eng.SetLoadErrors(loadErrors)
		res := eng.Run()
		out.Rounds++
		if roundIncomplete(res) {
			// not used: candidates of a run that did not finish would hide the ones it missed
			out.TimedOut = true
			break
		}
		carried := map[string][]string{}
		for _, c := range res.FuncCandidates {
			carried[c.Site.ID] = c.Candidates
		}
		changed := false
		for i, c := range zero {
			grew[i] = false
			if limited[i] {
				continue
			}
			next := carried[c.Site.ID]
			if !subset(have[i], next) {
				// candidates only grow: a run that lost one is a defect (CheckMonotone); what was found is kept
				out.Violations++
				next = union(have[i], next)
			}
			if len(next) == len(have[i]) {
				continue
			}
			have[i], changed, grew[i] = next, true, true
			graph.SetValueCandidates(c.Instr, next)
			limited[i] = len(next) > ir.ValueCandidateMost
		}
		if !changed {
			break
		}
	}
	for i, c := range zero {
		if (grew[i] || out.TimedOut && len(have[i]) > 0) && !limited[i] {
			c.Instr.Callee.ValueRoundsLimit = true
			out.RoundsCut++
		}
		switch {
		case limited[i]:
			out.Limited++
		case len(have[i]) > 0:
			out.Supplemented++
		}
		if c.ZeroCandidate() {
			continue
		}
		out.ZeroAfter--
	}
	return out
}

// subset says whether every function of a (sorted) is in b (sorted).
func subset(a, b []string) bool {
	in := make(map[string]bool, len(b))
	for _, x := range b {
		in[x] = true
	}
	for _, x := range a {
		if !in[x] {
			return false
		}
	}
	return true
}

func union(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, x := range b {
		set[x] = true
	}
	out := make([]string, 0, len(set))
	for x := range set {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

// CheckMonotone returns the calls (by the ID of the synthetic site) whose candidates in the next round do not hold all of
// the candidates of the round before: the candidates only grow, so each of them is a defect.
func CheckMonotone(prev, next map[string][]string) []string {
	var bad []string
	for id, p := range prev {
		if !subset(p, next[id]) {
			bad = append(bad, id)
		}
	}
	sort.Strings(bad)
	return bad
}

// FuncDropped is a symbol with a model that was given function values it says nothing of (FV-2 of the review of the first
// step): Calls is the number of calls (in all that was read) where it happened.
type FuncDropped struct {
	Symbol string `json:"symbol"`
	Calls  int    `json:"calls"`
	// Funcs are the functions that hold such a call (by the ID).
	Funcs []string `json:"-"`
}

// countDroppedFuncValues counts the call (once, however often the function is analysed) when an operand of a call that
// has a model holds a function value and no flow, call row or sanitizer of the model reads that operand: the model is
// where the function value is lost. Only counted: nothing is changed (no exception of the label FUNC in the engine).
func (s *state) countDroppedFuncValues(cc *callCtx) {
	rules := s.eng.rules
	covered := func(sel string, recv bool, i int) bool {
		sl, err := model.ParseSelector(sel)
		if err != nil {
			return false
		}
		switch sl.Kind {
		case model.SelRecv:
			return recv
		case model.SelArg:
			return !recv && (sl.All || sl.Index == i)
		}
		return false
	}
	reads := func(recv bool, i int) bool {
		for _, san := range rules.Sanitizers[cc.sym] {
			if len(san.Flows) == 0 {
				return true // args to result
			}
			for _, fl := range san.Flows {
				if covered(fl.From, recv, i) {
					return true
				}
			}
		}
		for _, p := range rules.Propagators[cc.sym] {
			for _, fl := range p.Flows {
				if covered(fl.From, recv, i) {
					return true
				}
			}
			for _, row := range p.Calls {
				if covered(row.Fn, recv, i) {
					return true
				}
				for _, w := range row.With {
					if covered(w, recv, i) {
						return true
					}
				}
			}
		}
		return false
	}
	holds := func(v *sg.Value) bool {
		for _, f := range s.flatten(s.locs(v)) {
			if f.E.Kind == Real && f.E.Label() == FuncLabel {
				return true
			}
		}
		return false
	}
	dropped := false
	if cc.recv != nil && !reads(true, 0) && holds(cc.recv) {
		dropped = true
	}
	for i, a := range cc.args {
		if !dropped && !reads(false, i) && holds(a) {
			dropped = true
		}
	}
	if !dropped {
		return
	}
	m, _ := s.eng.fvDropped.LoadOrStore(cc.sym, &sync.Map{})
	m.(*sync.Map).Store(cc.in, s.eng.funcID(s.fn))
}

func (e *Engine) droppedAtModel() []FuncDropped {
	var out []FuncDropped
	e.fvDropped.Range(func(k, v any) bool {
		d := FuncDropped{Symbol: k.(string)}
		set := map[string]bool{}
		v.(*sync.Map).Range(func(_, fn any) bool {
			d.Calls++
			set[fn.(string)] = true
			return true
		})
		for f := range set {
			d.Funcs = append(d.Funcs, f)
		}
		sort.Strings(d.Funcs)
		out = append(out, d)
		return true
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}
