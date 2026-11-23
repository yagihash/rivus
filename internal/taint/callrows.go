package taint

import (
	"sort"
	"strings"

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

// callRowsFor are the rows of calls of the call: the ones of the models of the symbol, and where there are none, the
// rows that stand for the function values given to a function of a declared package (bodyreplaced.go).
func (e *Engine) callRowsFor(ci *sg.Instr, sym string) []callRowRef {
	if rows := e.callRowsOf(sym); len(rows) > 0 {
		return rows
	}
	if rows := e.replacedRows(ci); len(rows) > 0 {
		return rows
	}
	return e.noModelRows(ci)
}

// callbackFuncs are the functions that the rows of the call make concrete: the call depends on their
// summaries (the order of the analysis).
func (e *Engine) callbackFuncs(ci *sg.Instr) []*sg.Func {
	if ci.Callee == nil {
		return nil
	}
	rows := e.callRowsFor(ci, ci.Callee.Symbol)
	out := e.implFuncs(ci)
	if len(rows) == 0 {
		return out
	}
	for _, r := range rows {
		if m := r.row().Method; m != "" {
			out = append(out, e.methodImpls(ci.Callee.Symbol, m)...)
			continue
		}
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
	rows := s.eng.callRowsFor(cc.in, cc.sym)
	if len(rows) == 0 {
		return nil
	}
	var out []*sg.Value
	for _, r := range rows {
		if r.row().Method != "" {
			continue
		}
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
	if row.Method != "" {
		s.applyMethodRow(cc, row)
		return
	}
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
	if row.Assumed != "" {
		notes = append(notes, s.newNote(cc.in, KindAssumedCall, row.Assumed, cc.sym))
	}
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

// applyMethodRow applies a row of calls that names a method (a provided method of a trait of the library calls a
// method of the same trait on its receiver: `Write::write_all` calls `write`). The implementations of the method that
// the program has are called, as a call of them is, with the receiver and what the row says (`with`; the positions
// count every operand, the receiver first). When the receiver is seen to be of one type, only that type's. The flows
// of the model (the library's own body, which is not read) are applied beside it.
func (s *state) applyMethodRow(cc *callCtx, row model.CallRow) {
	impls := s.eng.methodImpls(cc.sym, row.Method)
	if len(impls) == 0 {
		return
	}
	ops := cc.argv()
	if len(ops) == 0 {
		return
	}
	if t := receiverType(ops[0]); t != "" {
		var same []*sg.Func
		for _, fn := range impls {
			if self := implSelfType(fn.ID); self != "" && sameBase(self, withoutGenerics(t)) {
				same = append(same, fn)
			}
		}
		if len(same) > 0 {
			impls = same
		} else if isStdType(t) {
			// a type of the standard library (a `Vec<u8>`, a `String`) has its own method: the project's
			// implementations are of other types
			return
		}
	}
	with := make([]*sg.Value, 1, 1+len(row.With))
	with[0] = ops[0]
	for _, t := range row.With {
		var v *sg.Value
		if sel, err := model.ParseSelector(t); err == nil && sel.Kind == model.SelArg && !sel.All && sel.Index < len(ops) {
			v = ops[sel.Index]
		} else if err == nil && sel.Kind == model.SelRecv {
			v = ops[0]
		}
		if v == nil {
			v = &sg.Value{Kind: sg.KUndef}
		}
		with = append(with, v)
	}
	var notes []*Uncertainty
	if len(impls) > 1 {
		d := s.newNote(cc.in, KindDynamicDispatch, cc.sym+" calls "+row.Method+" of its receiver, which has several implementations", cc.sym)
		for _, fn := range impls {
			d.Candidates = append(d.Candidates, fn.ID)
		}
		sort.Strings(d.Candidates)
		notes = append(notes, d)
	}
	for _, fn := range impls {
		args := make([]*sg.Value, len(fn.Params))
		for k := range args {
			if k < len(with) {
				args[k] = with[k]
			} else {
				args[k] = &sg.Value{Kind: sg.KUndef}
			}
		}
		x := &callCtx{in: cc.in, args: args, roots: cc.roots, sym: cc.sym}
		s.applySummary(x, fn, nil, notes, nil)
	}
}

// receiverType is the type of the object that a receiver was made of right there: the type of the first value, through
// the copies and the conversions to a trait object, that is not a trait object itself ("" when the receiver is not made
// here, or is a trait object all the way).
func receiverType(v *sg.Value) string {
	for i := 0; i < 8 && v != nil; i++ {
		if t := string(v.Type); t != "" && !strings.Contains(t, "dyn ") {
			return t
		}
		if v.Kind != sg.KResult || v.Instr == nil || len(v.Instr.Args) != 1 {
			return ""
		}
		switch v.Instr.Op {
		case ir.OpCopy, ir.OpConvert:
			v = v.Instr.Args[0]
		default:
			return ""
		}
	}
	return ""
}

// methodImpls are the implementations that the program has (its own code, not its dependencies) of the method of
// the trait that a symbol is a method of: for `std::io::Write::write_all` and `write`, the functions
// `<T as std::io::Write>::write`.
func (e *Engine) methodImpls(sym, method string) []*sg.Func {
	i := strings.LastIndex(sym, "::")
	if i < 0 {
		return nil
	}
	trait := sym[:i]
	e.implsOnce.Do(func() {
		e.implsIndex = map[string][]*sg.Func{}
		for _, fn := range e.prog.Funcs {
			if fn.Dep || !strings.HasPrefix(fn.ID, "<") {
				continue
			}
			tr, m, ok := splitImplID(fn.ID)
			if ok {
				k := lastSegment(tr) + "::" + m
				e.implsIndex[k] = append(e.implsIndex[k], fn)
			}
		}
	})
	var out []*sg.Func
	for _, fn := range e.implsIndex[lastSegment(trait)+"::"+method] {
		if tr, _, _ := splitImplID(fn.ID); sameTraitPath(tr, trait) {
			out = append(out, fn)
		}
	}
	return out
}

// implSelfType is the type an impl method is of, without its generic arguments (`<a::T<u8> as b::Tr>::m` is `a::T`).
func implSelfType(id string) string {
	i := strings.Index(id, " as ")
	if !strings.HasPrefix(id, "<") || i < 0 {
		return ""
	}
	return withoutGenerics(id[1:i])
}

func withoutGenerics(t string) string {
	if j := strings.IndexByte(t, '<'); j >= 0 {
		return t[:j]
	}
	return t
}

// splitImplID splits `<Type as path::Trait<A>>::method` into the path of the trait (without its generic
// arguments) and the method.
func splitImplID(id string) (trait, method string, ok bool) {
	depth, as, end := 0, -1, -1
	for i := 0; i < len(id); i++ {
		switch id[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				end = i
			}
		case ' ':
			if depth == 1 && as < 0 && strings.HasPrefix(id[i:], " as ") {
				as = i + 4
			}
		}
		if end >= 0 {
			break
		}
	}
	if as < 0 || end < 0 || !strings.HasPrefix(id[end:], ">::") {
		return "", "", false
	}
	trait = id[as:end]
	if j := strings.IndexByte(trait, '<'); j >= 0 {
		trait = trait[:j]
	}
	method = id[end+3:]
	if j := strings.IndexAny(method, "<:"); j >= 0 {
		method = method[:j]
	}
	return trait, method, trait != "" && method != ""
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "::"); i >= 0 {
		return path[i+2:]
	}
	return path
}

// sameTraitPath: two paths name one trait when they are equal, or both are of the standard library (where it is
// defined, `core::fmt::Write`; where it is used, `std::fmt::Write`) and end in the same two names.
func sameTraitPath(a, b string) bool {
	if a == b {
		return true
	}
	std := func(p string) bool {
		return strings.HasPrefix(p, "std::") || strings.HasPrefix(p, "core::") || strings.HasPrefix(p, "alloc::")
	}
	if !std(a) || !std(b) {
		return false
	}
	// the module that the standard library puts it in is the second name (`core::iter::traits::iterator::Iterator` is `std::iter::Iterator`)
	sa, sb := strings.Split(a, "::"), strings.Split(b, "::")
	return len(sa) >= 3 && len(sb) >= 3 && sa[1] == sb[1] && sa[len(sa)-1] == sb[len(sb)-1]
}

// coveredByModel reports whether every use of the closure made here, other than a call right here, is as the
// function that a row of calls of a model makes concrete: the call applies its summary with its captured values
// bound, and applying it where it is made as well would count it twice (assumed_call is for what is not seen).
func (s *state) coveredByModel(mk *sg.Instr) bool {
	rules := s.eng.rules
	if rules == nil || (len(rules.Calls) == 0 && !rules.HasDeclared()) || len(mk.Dsts) != 1 {
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
	for _, r := range s.eng.callRowsFor(u, u.Callee.Symbol) {
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

// isStdType: the type is a path of the standard library (std, alloc, core), not a type parameter or a type of the project.
func isStdType(t string) bool {
	for _, p := range []string{"std::", "alloc::", "core::", "&std::", "&mut std::", "&alloc::", "&mut alloc::", "&core::", "&mut core::"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}
