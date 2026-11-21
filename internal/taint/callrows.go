package taint

import (
	"sort"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
)

// The rows of `calls` in a model say that a library function calls a function it was given. Where the
// function that was passed can be told (a function, or a closure made right there: one to four of them),
// its summary is applied at the call, with the values the row names for its parameters, its captured values
// bound, and its result, throw and writes sent where the row says. Where it cannot, the flows that stand for
// the row (model.Propagator.DerivedFlows) are used, and each use is an assumed_call.

// maxCallbacks is how many functions one row is made concrete for. More are not told: the flows stand for it.
const maxCallbacks = 4

// redirect says where the outputs of the callback go, when they are not the outputs of a call of it.
type redirect struct {
	ret, throw []Loc
	// throwSet: throw goes to throw (which may be nowhere), not out of the caller
	throwSet bool
	writes   map[int][]Loc
}

// callbacksOf are the functions the value is, when they are told (ExactClosures, one to four) and the program
// has a body for each. ok is false otherwise.
func (e *Engine) callbacksOf(v *sg.Value) ([]sg.Closure, bool) {
	if v == nil {
		return nil, false
	}
	cs, ok := sg.ExactClosures(v)
	if !ok || len(cs) == 0 || len(cs) > maxCallbacks {
		return nil, false
	}
	for _, c := range cs {
		fn := e.prog.ByID[c.ID]
		if fn == nil {
			return nil, false
		}
		if c.Env != nil {
			if len(fn.FreeVars) != 0 || len(fn.Params) == 0 {
				return nil, false
			}
			continue
		}
		if c.Make != nil && len(c.Make.Args) != len(fn.FreeVars) {
			return nil, false
		}
		if c.Make == nil && len(fn.FreeVars) != 0 {
			return nil, false
		}
	}
	return cs, true
}

// pick is the value a position of a call names (recv, argN; the receiver is not counted in argN).
func pick(recv *sg.Value, args []*sg.Value, sel model.Selector) *sg.Value {
	switch sel.Kind {
	case model.SelRecv:
		return recv
	case model.SelArg:
		if !sel.All && sel.Index < len(args) {
			return args[sel.Index]
		}
	}
	return nil
}

type callRowRef struct {
	p *model.Propagator
	i int
}

func (r callRowRef) row() model.CallRow { return r.p.Calls[r.i] }

// callRowsOf are the rows of calls of the models of the symbol, with the propagator that has each.
func (e *Engine) callRowsOf(sym string) []callRowRef {
	if e.rules == nil || sym == "" || !e.rules.Calls[sym] {
		return nil
	}
	var out []callRowRef
	for _, p := range e.rules.Propagators[sym] {
		p := p
		for i := range p.Calls {
			out = append(out, callRowRef{&p, i})
		}
	}
	return out
}

// callbackFuncs are the functions that the rows of the call make concrete: the call depends on their
// summaries (the order of the analysis).
func (e *Engine) callbackFuncs(ci *sg.Instr) []*sg.Func {
	if ci.Callee == nil {
		return nil
	}
	rows := e.callRowsOf(ci.Callee.Symbol)
	if len(rows) == 0 {
		return nil
	}
	var out []*sg.Func
	for _, r := range rows {
		sel, err := model.ParseSelector(r.row().Fn)
		if err != nil {
			continue
		}
		cs, ok := e.callbacksOf(pick(ci.Recv(), ci.CallArgs(), sel))
		if !ok {
			continue
		}
		for _, c := range cs {
			out = append(out, e.prog.ByID[c.ID])
		}
	}
	return out
}

// callBindings are the values that the closures made concrete by the rows of the call captured: what the
// call reads besides its operands.
func (s *state) callBindings(cc *callCtx) []*sg.Value {
	rows := s.eng.callRowsOf(cc.sym)
	if len(rows) == 0 {
		return nil
	}
	var out []*sg.Value
	for _, r := range rows {
		sel, err := model.ParseSelector(r.row().Fn)
		if err != nil {
			continue
		}
		cs, ok := s.eng.callbacksOf(pick(cc.recv, cc.args, sel))
		if !ok {
			continue
		}
		for _, c := range cs {
			if c.Make != nil {
				out = append(out, c.Make.Args...)
			}
		}
	}
	return out
}

// applyCallRow applies one row of calls at the call.
func (s *state) applyCallRow(cc *callCtx, p *model.Propagator, i int, mref *ModelRef) {
	row := p.Calls[i]
	fsel, err := model.ParseSelector(row.Fn)
	if err != nil {
		return
	}
	cs, ok := s.eng.callbacksOf(pick(cc.recv, cc.args, fsel))
	if !ok {
		derived := p.DerivedFlows(i)
		s.applyFlows(cc, derived, nil, "", mref)
		if len(derived) == 0 {
			// nothing stands for the row (it carries nothing, or the flows of the model say it): the function that was
			// given is still not followed, and that is said
			s.newNote(cc.in, KindUnresolvedCallee, "the function given to "+cc.sym+" is not known here: what it does with what the library hands it is not followed", cc.sym)
		}
		return
	}
	with := make([]*sg.Value, len(row.With))
	for k, t := range row.With {
		if sel, err := model.ParseSelector(t); err == nil {
			with[k] = pick(cc.recv, cc.args, sel)
		}
		if with[k] == nil {
			with[k] = &sg.Value{Kind: sg.KUndef}
		}
	}
	dests := func(t string) []Loc {
		sel, err := model.ParseSelector(t)
		if err != nil {
			return nil
		}
		if sel.Kind == model.SelResult {
			if sel.Index < len(cc.roots) {
				return []Loc{{cc.roots[sel.Index], ""}}
			}
			return nil
		}
		v := pick(cc.recv, cc.args, sel)
		if v == nil {
			return nil
		}
		return append(append([]Loc(nil), s.locs(v)...), s.pointeeOfInterface(v)...)
	}
	background := row.Mode == model.CallModeGo || row.Mode == model.CallModeDefer
	rd := &redirect{throwSet: row.Throw != "" || background}
	if row.Ret != "" {
		rd.ret = dests(row.Ret)
	}
	if row.Throw != "" && !background {
		rd.throw = dests(row.Throw)
	}
	if len(row.Writes) > 0 {
		rd.writes = map[int][]Loc{}
		for k, t := range row.Writes {
			rd.writes[k] = dests(t)
		}
	}

	var notes []*Uncertainty
	if len(cs) > 1 {
		d := s.newNote(cc.in, KindDynamicDispatch, "the function given to "+cc.sym+" is one of several", cc.sym)
		for _, c := range cs {
			d.Candidates = append(d.Candidates, c.ID)
		}
		sort.Strings(d.Candidates)
		notes = append(notes, d)
	}
	if background {
		notes = append(notes, s.pathNote(KindConcurrency, "the callback runs on its own; the order of effects is not modelled", cc.in.Pos))
	}
	for _, c := range cs {
		fn := s.eng.prog.ByID[c.ID]
		args := make([]*sg.Value, len(fn.Params))
		first := 0
		if c.Env != nil {
			args[0], first = c.Env, 1 // a closure of Rust takes its environment first
		}
		for k := first; k < len(args); k++ {
			if k-first < len(with) {
				args[k] = with[k-first]
			} else {
				args[k] = &sg.Value{Kind: sg.KUndef}
			}
		}
		var bindings []*sg.Value
		if c.Make != nil {
			bindings = c.Make.Args
		}
		x := &callCtx{in: cc.in, args: args, roots: cc.roots, sym: cc.sym, rd: rd}
		s.applySummary(x, fn, bindings, notes, nil)
	}
}

// coveredByModel reports whether every use of the closure made here, other than a call right here, is as the
// function that a row of calls of a model makes concrete: the call applies its summary with its captured values
// bound, and applying it where it is made as well would count it twice (assumed_call is for what is not seen).
func (s *state) coveredByModel(mk *sg.Instr) bool {
	rules := s.eng.rules
	if rules == nil || len(rules.Calls) == 0 || len(mk.Dsts) != 1 {
		return false
	}
	if s.useIndex == nil {
		s.useIndex = map[*sg.Value][]*sg.Instr{}
		for _, blk := range s.fn.Blocks {
			for _, in := range blk.Instrs {
				for _, a := range in.Args {
					if a != nil {
						s.useIndex[a] = append(s.useIndex[a], in)
					}
				}
			}
		}
	}
	seen := map[*sg.Value]bool{}
	work := []*sg.Value{mk.Dsts[0]}
	for len(work) > 0 {
		x := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[x] {
			continue
		}
		seen[x] = true
		for _, u := range s.useIndex[x] {
			switch {
			case u.Op == ir.OpCall && u.Callee != nil && u.Callee.Kind == ir.CalleeValue && len(u.Args) > 0 && u.Args[0] == x && !hasValue(u.Args[1:], x):
				// called right here
			case u.Op == sg.OpPhi, u.Op == ir.OpCopy && u.IR != nil && !u.IR.View && len(u.Args) == 1:
				work = append(work, u.Dsts...)
			case u.Op == ir.OpCall && s.givenToModel(u, x):
			default:
				return false
			}
		}
	}
	return true
}

// givenToModel reports whether the call has the value, once, as the function of a row of calls that is made
// concrete.
func (s *state) givenToModel(u *sg.Instr, x *sg.Value) bool {
	if u.Callee == nil {
		return false
	}
	n := 0
	for _, a := range u.Args {
		if a == x {
			n++
		}
	}
	if n != 1 {
		return false
	}
	recv, args := u.Recv(), u.CallArgs()
	for _, r := range s.eng.callRowsOf(u.Callee.Symbol) {
		sel, err := model.ParseSelector(r.row().Fn)
		if err != nil || pick(recv, args, sel) != x {
			continue
		}
		if _, ok := s.eng.callbacksOf(x); ok {
			return true
		}
	}
	return false
}

func hasValue(vs []*sg.Value, x *sg.Value) bool {
	for _, v := range vs {
		if v == x {
			return true
		}
	}
	return false
}
