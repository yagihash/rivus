package taint

// Function values that are held in a cell (docs/issues/210). A function that the code of the project makes
// (make_closure, a function used as a value) puts one Real element of the label funcLabel, whose origin is
// "fn:<ID of the function>", in the cell of the value. The element is carried as the data is: by copies, φ,
// fields, containers, globals, the results and the arguments of calls. At a call of a function value that
// nothing else gave a candidate, the elements that the cell of the called value holds are the functions it may
// be, and the summary of each is applied (applyFuncCells).
//
// An element of this label is not taint: its origin is no source site, so it is never a finding (buildFinding), and
// the statistics of the summaries count it apart. Only Dart is carried (the call graph of Go resolves the calls,
// and the closures of Rust are narrowed by valuecalls), and only what the code of the project makes (what a
// dependency makes is not seeded: docs/DESIGN-callouts.md, 5.4).
//
// The cell is told by its root: the objects that two allocations make are two roots, and the functions stored in
// one are not read through the other. What the cell does not tell apart (the instances that one allocation makes,
// the keys of a container) is over-approximated, and the call says so (unresolved_callee).

import (
	"sort"
	"strings"
	"sync"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

const (
	funcLabel  = "FUNC"
	funcOrigin = "fn:"
)

var funcLabelID = intern(funcLabel)

// isDartFunc says whether the ID is of a Dart function (the extractor of Dart names them by package, file or dart: library).
func isDartFunc(id string) bool {
	return strings.HasPrefix(id, "package:") || strings.HasPrefix(id, "file:") || strings.HasPrefix(id, "dart:")
}

// funcCellsOn says whether the function values made in this function are carried.
func (s *state) funcCellsOn() bool {
	return !s.fn.Dep && isDartFunc(s.fn.ID)
}

// seedFunc puts the function the value stands for in the cell.
func (s *state) seedFunc(l Loc, id string, pos ir.Pos, what string) {
	s.add(l, RealElem(funcLabel, funcOrigin+id), s.origin(pos, what))
}

// funcCellCall says whether the call is one that the cell of the called value may give candidates: a call of a
// function value that nothing else resolved (the candidates of valuecalls and of the extractor come first, and a
// limit of rivus' own that left functions out keeps its uncertainty).
func (s *state) funcCellCall(ci *sg.Instr) bool {
	c := ci.Callee
	if c == nil || c.Kind != ir.CalleeValue || len(ci.Args) == 0 || !s.funcCellsOn() {
		return false
	}
	return ci.Static == nil && len(c.ValueCands) == 0 && len(c.Candidates) == 0 &&
		(c.ValueLimit == "" || c.ValueLimit == ir.ValueCallerDepthLimit)
}

// funcCellCands are the functions that the cell of the called value holds (sorted).
func (s *state) funcCellCands(ci *sg.Instr) []string {
	set := map[string]bool{}
	for _, f := range s.flatten(s.locs(ci.Args[0])) {
		if f.E.Kind == Real && f.E.label == funcLabelID {
			set[strings.TrimPrefix(f.E.Origin(), funcOrigin)] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// closureBindings are the values that the closure of fn captured, when it is made in this function.
func (s *state) closureBindings(fn *sg.Func) []*sg.Value {
	if s.closureMk == nil {
		s.closureMk = map[string]*sg.Instr{}
		for _, b := range s.fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpMakeClosure && in.IR != nil && in.IR.Func != "" {
					s.closureMk[string(in.IR.Func)] = in
				}
			}
		}
	}
	if mk := s.closureMk[fn.ID]; mk != nil && len(mk.Args) == len(fn.FreeVars) {
		return mk.Args
	}
	return nil
}

// applyFuncCells applies the summary of each function that the cell of the called value holds, besides what the call
// does without them: the call stays one that nothing resolved (the value may be a function that is not in the cell).
// More than ir.ValueCandidateMost functions are not followed (the cell is folded into the call as it was), and the call says so.
func (s *state) applyFuncCells(cc *callCtx) {
	ci := cc.in
	if !s.funcCellCall(ci) {
		return
	}
	ids := s.funcCellCands(ci)
	if len(ids) == 0 {
		return
	}
	if len(ids) > ir.ValueCandidateMost {
		s.newNote(ci, KindDepthLimit, "candidate_limit: the cell of the function value holds "+itoa(len(ids))+" functions, more than the "+itoa(ir.ValueCandidateMost)+" that rivus keeps; none is followed (rivus' limit, not the code's)", "")
		return
	}
	s.eng.noteCellCands(ci, ids)
	n := s.newNote(ci, KindUnresolvedCallee, "func_cells: the functions that were put in the cell this value is read from are followed ("+itoa(len(ids))+
		"); the cell does not tell apart the instances that one allocation makes or the keys of a container, so a function that this call does not call may be among them, and the value may be a function that was not put in the cell", "")
	n.Candidates = ids
	notes := []*Uncertainty{n}
	rules := s.eng.rules
	for _, id := range ids {
		fn := s.eng.prog.ByID[id]
		vc := ir.ValueCand{ID: ir.FuncID(id), Symbol: id, Param: -1, Recv: strings.HasPrefix(id, "(")}
		if fn != nil {
			vc.Symbol, vc.Recv = fn.Symbol, fn.Recv
		}
		cx := s.valueCtx(cc, vc)
		if cx.sym != "" && rules.HasModel(cx.sym) {
			s.applyModel(cx)
			continue
		}
		if fn == nil {
			continue // no body: the call keeps what it does for a function that is not known
		}
		s.applySummary(cx, fn, s.closureBindings(fn), notes, nil)
	}
}

// funcRefs are, for the instructions of the Dart functions of the project that name a function (a closure that is
// made, a function used as a value), that function: the call that reads the function values of a cell needs its
// summary, and the function that makes the value is analysed after it (as for a closure that escapes).
func funcRefs(fns []*sg.Func) map[*sg.Instr][]*sg.Func {
	out := map[*sg.Instr][]*sg.Func{}
	for _, fn := range fns {
		if fn.Dep || !isDartFunc(fn.ID) {
			continue
		}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpMakeClosure && in.IR != nil && in.IR.Func != "" {
					if f := fn.Prog.ByID[string(in.IR.Func)]; f != nil && f != fn {
						out[in] = append(out[in], f)
					}
				}
				for _, a := range in.Args {
					if a != nil && a.Kind == sg.KFunc && a.Fn != nil && a.Fn != fn && isDartFunc(a.Fn.ID) {
						out[in] = append(out[in], a.Fn)
					}
				}
			}
		}
	}
	return out
}

type cellCands struct {
	mu  sync.Mutex
	ids map[string]bool
}

// noteCellCands records the functions that a call read from a cell, in any of the rounds and for any caller.
func (e *Engine) noteCellCands(ci *sg.Instr, ids []string) {
	v, _ := e.cellCalls.LoadOrStore(ci, &cellCands{ids: map[string]bool{}})
	c := v.(*cellCands)
	c.mu.Lock()
	for _, id := range ids {
		c.ids[id] = true
	}
	c.mu.Unlock()
}

// FuncCellCandidates are the functions that the calls of function values read from a cell (sorted), after Run: the
// candidates of the call, for the graph of "rivus index -taint". A call that was given more than ir.ValueCandidateMost
// is left out.
func (e *Engine) FuncCellCandidates() map[*sg.Instr][]string {
	out := map[*sg.Instr][]string{}
	e.cellCalls.Range(func(k, v any) bool {
		c := v.(*cellCands)
		if len(c.ids) > ir.ValueCandidateMost {
			return true
		}
		ids := make([]string, 0, len(c.ids))
		for id := range c.ids {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out[k.(*sg.Instr)] = ids
		return true
	})
	return out
}
