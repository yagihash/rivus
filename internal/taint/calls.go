package taint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// callCtx is a call with its operands laid out.
type callCtx struct {
	in    *sg.Instr
	recv  *sg.Value   // nil for plain functions
	args  []*sg.Value // receiver excluded
	roots []Root      // one per result
	sym   string
	// made: the closure's summary is applied where the closure is made (C-7), not at a call: what
	// it throws goes to no handler here, and it has no result to give.
	made bool
	// rd: the callback of a row of calls, whose outputs go where the row says (callrows.go)
	rd *redirect
	// noResult: the body is read for what it does to the sinks, the arguments, the receiver and
	// the globals, and what it returns is dropped (the rule is a sanitizer: it says the result is clean)
	noResult bool
}

// argv lists the operands in parameter order (receiver first).
func (c *callCtx) argv() []*sg.Value {
	var out []*sg.Value
	if c.recv != nil {
		out = append(out, c.recv)
	}
	return append(out, c.args...)
}

// builtinName is the name of the builtin a call is of, or "".
func builtinName(ci *sg.Instr) string {
	if ci.Callee.Kind == ir.CalleeStatic && ci.Callee.Func == "" && strings.HasPrefix(ci.Callee.Symbol, "builtin.") {
		return strings.TrimPrefix(ci.Callee.Symbol, "builtin.")
	}
	return ""
}

func (s *state) call(ci *sg.Instr) {
	cc := &callCtx{in: ci, sym: ci.Callee.Symbol}
	if builtinName(ci) != "" {
		cc.sym = "" // a builtin has no symbol the rules could name
	}
	cc.recv, cc.args = ci.Recv(), ci.CallArgs()
	cc.roots = callRoots(ci)
	if vc := ci.Callee.ValueCands; ci.Callee.Kind == ir.CalleeValue && len(vc) == 1 {
		// A call of a function value that can reach one function is a call of it.
		cc = s.valueCtx(cc, vc[0])
	}

	// A call only reads the cells its operands point to. If none of them
	// changed since the last time, applying the callee again would add
	// nothing: skip it. This matters for functions with many calls, which
	// are otherwise revisited in every iteration of the fixed point.
	before := s.callSig(cc)
	if prev, ok := s.callSigs[ci]; ok && prev == before {
		return
	}
	s.callBody(cc)
	if s.callSig(cc) == before {
		if s.callSigs == nil {
			s.callSigs = map[*sg.Instr]int{}
		}
		s.callSigs[ci] = before
	} else {
		delete(s.callSigs, ci)
	}
}

// valueCtx is the call seen as a call of the candidate: the rules name it, and
// a method taken as a function has the receiver as its first argument.
func (s *state) valueCtx(cc *callCtx, c ir.ValueCand) *callCtx {
	x := *cc
	x.sym = c.Symbol
	if c.Recv && cc.recv == nil && len(cc.args) > 0 {
		x.recv, x.args = cc.args[0], cc.args[1:]
	}
	return &x
}

// callSig changes whenever something is added to a cell the call reads. The
// versions only grow, so their sum is enough.
func (s *state) callSig(cc *callCtx) int {
	sum := 1
	add := func(v *sg.Value) {
		for _, l := range s.locs(v) {
			sum += s.rootVer[l.Root]
		}
		sum += s.markSig(v, cc.in)
	}
	for _, v := range cc.argv() {
		add(v)
	}
	for _, b := range cc.in.Bindings {
		add(b)
	}
	// the value that is called is read by the synthetic sink of the diagnostic of the function values
	if s.eng.opts.FuncValues && cc.in.Callee != nil && cc.in.Callee.Kind == ir.CalleeValue && len(cc.in.Args) > 0 {
		add(cc.in.Args[0])
	}
	// the values that the closures given to a callback row captured are read by the call too
	for _, b := range s.callBindings(cc) {
		add(b)
	}
	return sum
}

// callee is a function a call may reach; fn is nil when it has no body in
// the program.
type callee struct {
	id string
	fn *sg.Func
}

func (s *state) callBody(cc *callCtx) {
	ci := cc.in

	// Sources and sinks matched by the rules at this call.
	sitesHere := s.eng.sites.ByIRInstr[sites.IRInstr{Func: s.fn.IR.ID, Block: ci.IRBlock, Instr: ci.IRIndex}]
	if why, ok := s.eng.sites.UnresolvedCallbacks[sites.IRInstr{Func: s.fn.IR.ID, Block: ci.IRBlock, Instr: ci.IRIndex}]; ok {
		// a callback_param source names this call, and the function it is given was not found
		s.newNote(ci, KindUnresolvedCallee, why, cc.sym)
	}
	for _, site := range sitesHere {
		if site.Symbol == FuncValueSymbol {
			// the sink that the engine makes for the diagnostic of the function values: what is called is the first operand
			s.funcValueSink(site, ci)
			continue
		}
		// a site on a call of a function value is for one of the functions it may reach
		sc := cc
		if site.Callee != "" && ci.Callee.Kind == ir.CalleeDynamic {
			// a site on a call through an interface is for one of its implementations
			x := *cc
			x.sym = site.Symbol
			sc = &x
			if site.Role == sites.RoleSink {
				// the flow reaches the sink if the call goes to this implementation: the
				// finding says it is one of the candidates
				d := s.newNote(ci, KindDynamicDispatch, "call through an interface or function value", cc.sym)
				for _, id := range ci.Candidates {
					d.Candidates = append(d.Candidates, string(id))
				}
				sort.Strings(d.Candidates)
				s.sinkNotes = []*Uncertainty{d}
				s.checkSink(sc, site)
				s.sinkNotes = nil
				continue
			}
		} else if site.Callee != "" {
			for _, c := range ci.Callee.ValueCands {
				if string(c.ID) == site.Callee {
					sc = s.valueCtx(cc, c)
				}
			}
		}
		switch site.Role {
		case sites.RoleSource:
			outOnly := false
			for _, src := range site.Src {
				outOnly = outOnly || len(src.Out) > 0
			}
			// a source with out: the call fills the arguments it names (the
			// result is the error or the count, not the data)
			if !outOnly {
				// a number or a bool beside the data (`v, ok := src()`) is not an input that became one: it
				// holds nothing that the label could follow, and no stop is recorded for it
				beside := false
				for _, r := range cc.roots {
					if !s.numericRoot(r) {
						beside = true
					}
				}
				for _, r := range cc.roots {
					if beside && s.numericRoot(r) {
						continue
					}
					for _, l := range site.Labels {
						s.add(Loc{r, ""}, RealElem(l, site.ID), s.origin(ci.Pos, "source: "+site.Symbol))
					}
				}
			}
			for _, src := range site.Src {
				for _, i := range src.Out {
					if i >= len(sc.args) {
						continue
					}
					v := sc.args[i]
					dsts := append(append([]Loc(nil), s.locs(v)...), s.pointeeOfInterface(v)...)
					// variadic destinations: the pointers are the elements of
					// the slice the call was given (rows.Scan(&a, &b))
					for _, e := range s.variadicElems(v) {
						dsts = append(dsts, s.locs(e)...)
						dsts = append(dsts, s.pointeeOfInterface(e)...)
					}
					for _, d := range dsts {
						for _, l := range site.Labels {
							s.add(d, RealElem(l, site.ID), s.origin(ci.Pos, "source: "+site.Symbol+" (fills an argument)"))
						}
					}
				}
			}
		case sites.RoleSink:
			s.checkSink(sc, site)
		}
	}

	if name := builtinName(ci); name != "" {
		s.builtin(cc, name)
		return
	}
	if ci.Callee.Kind == ir.CalleeValue && len(ci.Callee.ValueCands) > 1 {
		s.valueCandidates(cc)
		return
	}
	rules := s.eng.rules
	// a package whose body is not read: what it may call back is applied where the model does not say it
	s.replacedCallbacks(cc)
	modelled := cc.sym != "" && rules.HasModel(cc.sym)
	if modelled {
		s.applyModel(cc)
		if ci.Callee.Kind != ir.CalleeDynamic && !staticImplOfModelledTrait(ci) {
			return
		}
		// A method of an interface or trait that the model describes is also implemented by
		// the code (Iterator::next, io.Writer.Write, http.Handler.ServeHTTP of a type of the
		// project): the model says what the implementations outside the code do, and the code
		// of the project's own is read (a sanitizer's with its result dropped: noResult; not that of its dependencies: the model is what says
		// what they do). The two are joined, so what the model gives is never lost: a model
		// that says `pure` leaves the body alone to say what happens.
		fns, _ := s.callees(ci)
		s.noteDepBodies(ci, cc, fns)
		if !hasOwnBody(fns) {
			return
		}
	}

	fns, bindings := s.callees(ci)
	if modelled && len(rules.Sanitizers[cc.sym]) > 0 && !rules.ReadsImpls(cc.sym) {
		x := *cc
		x.noResult = true
		cc = &x
	}
	if modelled {
		own := fns[:0:0]
		for _, c := range fns {
			if c.fn != nil && !c.fn.Dep {
				own = append(own, c)
			}
		}
		fns = own
	}
	var external bool
	var withBody []*sg.Func
	for _, c := range fns {
		if c.fn != nil {
			withBody = append(withBody, c.fn)
		} else {
			external = true
		}
	}
	// A source rule says what the result holds: it is the model. A sink rule says only where a
	// value must not arrive; it says nothing of what the call returns, so a function without a
	// body that has only a sink rule gets the default (what goes in comes out) and the
	// uncertainty of an unmodelled call, like any other (CORE-K).
	ruleOnly := cc.sym != "" && len(rules.CallSources[cc.sym]) > 0

	var notes []*Uncertainty
	var dispatch *Uncertainty
	invoke := ci.Callee.Kind == ir.CalleeDynamic
	if invoke || len(fns) > 1 {
		dispatch = s.newNote(ci, KindDynamicDispatch, "call through an interface or function value", cc.sym)
		for _, c := range fns {
			dispatch.Candidates = append(dispatch.Candidates, c.id)
		}
		sort.Strings(dispatch.Candidates)
		s.noteNarrowed(dispatch, ci)
		if len(fns) == 0 {
			// The implementations are outside the loaded code (typically
			// the standard library or a dependency). A model for the
			// interface method removes the uncertainty, so this is rivus'
			// to improve, like an unmodeled library call.
			dispatch.Detail = "no implementation is known: the implementations are outside the loaded code; a model for the interface method would resolve this"
			dispatch.AttributableTo = AttributableRivus
		}
		notes = append(notes, dispatch)
	}
	if ci.Callee.Kind == ir.CalleeValue && ci.Callee.ValueLimit == ir.ValuePartial {
		// one function was found (several are told by valueCandidates): followed, and said not to be all
		notes = append(notes, s.newNote(ci, KindUnresolvedCallee, ci.Callee.ValueLimitDetail(), ""))
	}
	notes = append(notes, s.supplementNotes(ci)...)
	if ci.IR.Go {
		notes = append(notes, s.pathNote(KindConcurrency, "the callee runs in another goroutine; the order of effects is not modelled", ci.Pos))
	}
	which := s.dispatchGuards(cc, withBody)
	for _, fn := range withBody {
		b := bindings
		if cb, ok := ci.CandidateBindings[fn]; ok {
			b = cb
		}
		s.applySummary(cc, fn, b, notes, which[fn])
	}
	switch {
	case ruleOnly, modelled:
		// the rule is the model
	case len(fns) == 0 && invoke:
		s.unmodeled(cc, dispatch)
		s.noModelCallbacks(cc)
	case len(fns) == 0 && ci.Callee != nil && ci.Callee.ValueLimit != "":
		// the functions are known and rivus left them out: its limit, not the code's
		s.unmodeled(cc, s.newNote(ci, KindDepthLimit, ci.Callee.ValueLimitDetail(), cc.sym))
		if ci.Callee.ValueLimit == ir.ValueCandidateLimit {
			// the callers of this function that pass a function there are told (applySummary)
			for _, p := range ci.Callee.ValueLimitParams {
				if s.limited == nil {
					s.limited = map[int]bool{}
				}
				s.limited[p] = true
			}
		}
	case len(fns) == 0:
		s.unmodeled(cc, s.newNote(ci, KindUnresolvedCallee, "the called function is not known", cc.sym))
		if ci.Callee != nil && ci.Callee.Kind == ir.CalleeValue {
			// the callers of this function that pass something there are told (applySummary)
			for _, p := range ci.Callee.ValueUnresolvedParams {
				if s.unresolved == nil {
					s.unresolved = map[int]bool{}
				}
				s.unresolved[p] = true
			}
		}
	case external:
		ids := make([]string, len(fns))
		for i, c := range fns {
			ids[i] = c.id
		}
		kind, detail := externalKind(ids, cc.sym)
		if ci.Callee != nil && ci.Callee.FFI {
			kind, detail = KindFFIBoundary, "the call leaves the language (a foreign function)"
		}
		if sk := s.eng.skippedBody(ids, cc.sym); sk != nil && kind == KindUnmodeledLibraryCall {
			// the function has a body, and the extractor could not read it: not a library call without a model
			kind = KindDependencyBodyUnread
			if !sk.Dependency {
				kind = KindProjectBodyUnread
			}
			detail = "the extractor could not read the body of " + sk.Func + ": " + sk.Why + " (what the function does with the value is not followed)"
		}
		if u := s.eng.replacedUnit(ci); u != "" && kind == KindUnmodeledLibraryCall {
			detail = "the body of the package " + u + " is not read (a rule file declares that its models stand for it) and the model has no row for the function"
		}
		s.unmodeled(cc, s.newNote(ci, kind, detail, cc.sym))
		// a function value that is given is taken to be called, and its result to reach the result (unmodeledcb.go)
		s.noModelCallbacks(cc)
	}
}

// valueCandidates is a call of a function value that may reach several
// functions, found from the function itself (they were copied or chosen in it:
// every one is reached), or from the callers of the function (it is a
// parameter: see valueGuards). What the rules and the models say about each is
// applied, and the call records the uncertainty with the candidates.
func (s *state) valueCandidates(cc *callCtx) {
	ci := cc.in
	rules := s.eng.rules
	cands := ci.Callee.ValueCands
	dispatch := s.newNote(ci, KindDynamicDispatch, "call through an interface or function value", "")
	for _, c := range cands {
		dispatch.Candidates = append(dispatch.Candidates, string(c.ID))
	}
	sort.Strings(dispatch.Candidates)
	notes := []*Uncertainty{dispatch}
	if ci.Callee.ValueLimit == ir.ValuePartial {
		// the candidates found are followed, and said not to be all of them
		notes = append(notes, s.newNote(ci, KindUnresolvedCallee, ci.Callee.ValueLimitDetail(), ""))
	}
	notes = append(notes, s.supplementNotes(ci)...)
	if ci.IR.Go {
		notes = append(notes, s.pathNote(KindConcurrency, "the callee runs in another goroutine; the order of effects is not modelled", ci.Pos))
	}
	guards := s.valueGuards(ci)
	var external []string
	for _, c := range cands {
		cx := s.valueCtx(cc, c)
		if cx.sym != "" && rules.HasModel(cx.sym) {
			s.applyModel(cx)
			continue
		}
		if fn := s.eng.prog.ByID[string(c.ID)]; fn != nil {
			b := []*sg.Value(nil)
			if cb, ok := ci.CandidateBindings[fn]; ok {
				b = cb
			}
			s.applySummary(cx, fn, b, notes, guards[string(c.ID)])
			continue
		}
		if cx.sym != "" && len(rules.CallSources[cx.sym]) > 0 {
			continue // the rule is the model
		}
		external = append(external, string(c.ID))
	}
	if len(external) > 0 {
		kind, detail := externalKind(external, "")
		s.unmodeled(cc, s.newNote(ci, kind, detail, ""))
	}
}

// valueGuards tells, for a call of a function value that the function has from
// one of its parameters, which candidate each flow went through. The callers
// know which function they passed: one that made it right there can leave out
// the flows through the other candidates (the call graph does not tell the
// callers apart: the functions every caller passes are the candidates). It is
// the guard of dispatchGuards, for a function value.
func (s *state) valueGuards(ci *sg.Instr) map[string][]Guard {
	cands := ci.Callee.ValueCands
	if len(cands) < 2 {
		return nil
	}
	if m, ok := s.eng.valueGuards.Load(ci); ok {
		return m.(map[string][]Guard)
	}
	var out map[string][]Guard
	for _, c := range cands {
		if c.Param < 0 || c.Param >= len(s.fn.Params) {
			continue
		}
		if out == nil {
			out = map[string][]Guard{}
		}
		out[string(c.ID)] = []Guard{{
			Func:     s.eng.funcID(s.fn),
			Params:   []int{c.Param},
			Cond:     "the function given as " + s.fn.Params[c.Param].Name + " is " + string(c.ID),
			Polarity: true,
			disp:     &dispatchSpec{fnID: string(c.ID)},
			fn:       s.fn,
			pos:      ci.Pos,
		}}
		out[string(c.ID)][0].k = out[string(c.ID)][0].computeKey() // the key is asked for at every comparison of guards
	}
	s.eng.valueGuards.Store(ci, out)
	return out
}

// dispatchGuards tells, for a call through an interface method on a parameter
// of this function, which implementation each flow went through: what the
// callee does with the flow is true only if the value has that type. The caller
// of this function knows what it passed, and can leave out the flows through the
// implementations of other types (the call graph does not tell the callers
// apart: the implementations of every caller are candidates of the call).
func (s *state) dispatchGuards(cc *callCtx, fns []*sg.Func) map[*sg.Func][]Guard {
	if cc.in.Callee != nil && cc.in.Callee.CandParam != nil && len(fns) > 1 {
		return s.paramGuards(cc, fns)
	}
	recv := cc.recv
	if cc.in.Callee.Kind != ir.CalleeDynamic || recv == nil || recv.Kind != sg.KParam || len(fns) < 2 {
		return nil
	}
	if m, ok := s.eng.dispatch.Load(cc.in); ok {
		return m.(map[*sg.Func][]Guard)
	}
	out := s.makeDispatchGuards(cc, fns)
	s.eng.dispatch.Store(cc.in, out)
	return out
}

func (s *state) makeDispatchGuards(cc *callCtx, fns []*sg.Func) map[*sg.Func][]Guard {
	recv := cc.recv
	var all []string
	for _, fn := range fns {
		if !fn.Recv || len(fn.Params) == 0 {
			return nil // a candidate that does not say what it is a method of: no guard
		}
		all = append(all, string(fn.Params[0].Type))
	}
	out := make(map[*sg.Func][]Guard, len(fns))
	for i, fn := range fns {
		out[fn] = []Guard{{
			Func:     s.eng.funcID(s.fn),
			Params:   []int{recv.Param},
			Cond:     "the value of " + recv.Name + " has the type " + all[i],
			Polarity: true,
			disp:     &dispatchSpec{recv: all[i], all: all},
			fn:       s.fn,
			pos:      cc.in.Pos,
		}}
		out[fn][0].k = out[fn][0].computeKey()
	}
	return out
}

// externalKind tells why a call to a function without a body cannot be
// followed: a missing model, reflection, or a foreign function interface.
func externalKind(ids []string, sym string) (kind, detail string) {
	for _, id := range ids {
		if strings.HasPrefix(id, "C.") || strings.Contains(id, "._Cfunc_") || strings.Contains(id, "._Cgo_") {
			return KindFFIBoundary, "the call leaves Go (cgo)"
		}
	}
	if strings.HasPrefix(sym, "reflect.") || strings.HasPrefix(sym, "(reflect.") || strings.HasPrefix(sym, "(*reflect.") {
		return KindReflection, "reflection decides at run time what happens"
	}
	return KindUnmodeledLibraryCall, "the function has no body and no model"
}

func callRoots(ci *sg.Instr) []Root {
	out := make([]Root, len(ci.Dsts))
	for i, d := range ci.Dsts {
		out[i] = own(d)
	}
	return out
}

// callees resolves the possible functions called, and the values captured
// by the closure when the callee is a closure created right there.
func (s *state) callees(ci *sg.Instr) ([]callee, []*sg.Value) {
	if ci.StaticID != "" {
		return []callee{{string(ci.StaticID), ci.Static}}, ci.Bindings
	}
	var out []callee
	for _, id := range ci.Candidates {
		out = append(out, callee{string(id), s.eng.prog.ByID[string(id)]})
	}
	return out, nil
}

func (s *state) newNote(in *sg.Instr, kind, detail, symbol string) *Uncertainty {
	u := newUncertainty(kind, detail, symbol)
	u.Func = s.eng.funcID(s.fn)
	u.pos = in.Pos
	s.uncertain = append(s.uncertain, u)
	return u
}

// --- sinks ---

type findingKey struct{ origin, sink string }

type rawFinding struct {
	origin string
	site   *sites.Site
	labels map[string]bool
	steps  *rope
	guards []Guard
	// unsure: the only ways known to reach the sink are ones that do not tell the field (field_imprecision):
	// a way that does tell it is the finding, and the ones that do not are dropped (a sure flow wins).
	unsure bool
}

// sinkKey is what a summary keeps apart: the element, the sink, the part of the value that is read below
// it (field: dotted names, "" for all of it), and whether the way is a sure one or an unsure one (the
// sure one is not mixed into the unsure one, or a flow that is known would be reported as a guess).
type sinkKey struct {
	e      Elem
	site   string
	field  string
	unsure bool
}

func lessSinkKey(a, b sinkKey) bool {
	if a.site != b.site {
		return a.site < b.site
	}
	if a.e != b.e {
		return lessElem(a.e, b.e)
	}
	if a.field != b.field {
		return a.field < b.field
	}
	return !a.unsure && b.unsure
}

// SinkReach says: taint coming from the symbolic element reaches a sink site.
type SinkReach struct {
	Elem    Elem
	Site    string
	Accepts []string
	Steps   *rope
	Guards  []Guard
	// Field is the part of the value that the sink reads, below Elem, still to be matched against the
	// cells of the caller (names separated by dots; "" for all of it: docs/DESIGN-field-sinks.md). The
	// summary does not know the steps of the path of the caller (the pointers), so the caller matches.
	Field string
	// Unsure: the way does not tell the field (field_imprecision), and the step of the sink carries it.
	Unsure bool
}

func (s *state) checkSink(cc *callCtx, site *sites.Site) {
	var vals []*sg.Value
	for _, i := range sites.SinkArgs(site.Sink, len(cc.args)) {
		vals = append(vals, cc.args[i])
	}
	for i := range site.Sink {
		if site.Sink[i].WholeRecv() && cc.recv != nil {
			vals = append(vals, cc.recv)
			break
		}
	}
	var note *Uncertainty
	for i := range site.Sink {
		if bad := site.Sink[i].NamedUnresolved; len(bad) > 0 {
			note = s.newNote(cc.in, KindNamedUnresolved,
				fmt.Sprintf("the sink %s names the parameter %s, which no call of it in the program has: the sink reads the whole call", site.Symbol, strings.Join(bad, ", ")), site.Symbol)
			break
		}
	}
	for _, v := range vals {
		for _, f := range s.flatten(s.locs(v)) {
			s.reach(site, f, cc.in, note, "")
		}
	}
	for _, p := range sites.SinkPartsAt(site.Sink, len(cc.args)) {
		s.checkPart(cc, site, p)
	}
}

// acceptedAt says whether the sink site reacts to a value of the label: the labels the rules of the sink accept, and for a label
// that a sanitizer gave with `neutralises`, the category of the site (ADR-115).
func (s *state) acceptedAt(accepts []string, label, siteID string) bool {
	if len(s.eng.rules.Derived) == 0 {
		return accepted(accepts, label)
	}
	var cats []string
	if site := s.eng.sites.ByID[siteID]; site != nil {
		if len(site.Categories) > 0 {
			cats = site.Categories
		} else if site.Category != "" {
			cats = []string{site.Category}
		}
	}
	return s.eng.rules.AcceptsLabel(accepts, label, cats)
}

func accepted(accepts []string, label string) bool {
	if len(accepts) == 0 {
		return true
	}
	for _, a := range accepts {
		if a == label {
			return true
		}
	}
	return false
}

// reach says that the element f, found in a value that the sink reads, reaches the sink. note is the
// uncertainty of a way that does not tell the field (field_imprecision, field_unresolved): nil for a sure
// one. field is the part still to be matched by the caller, for a symbolic element ("" for all of it).
func (s *state) reach(site *sites.Site, f found, in *sg.Instr, note *Uncertainty, field string) {
	steps := cat(s.trace(f.Cell, f.Orig), leaf(s.sinkStep(site, in, note)))
	gs := s.guardsAtSink(in, f)
	if site.Callee != "" && f.E.Kind == Param {
		if vg := s.valueGuards(in)[site.Callee]; len(vg) > 0 {
			if gs = unionGuards(gs, vg); len(gs) > maxGuards {
				s.guardOverflow = true
				gs = nil
			}
		}
	}
	switch f.E.Kind {
	case Real:
		if !s.acceptedAt(site.Labels, f.E.Label(), site.ID) {
			return
		}
		s.recordFinding(f.E.Origin(), site, f.E.Label(), steps, gs, note != nil)
	case Param:
		k := sinkKey{f.E, site.ID, field, note != nil}
		s.noteSinkReach(k, &SinkReach{Elem: f.E, Site: site.ID, Accepts: site.Labels, Steps: steps, Guards: gs, Field: field, Unsure: note != nil})
	}
}

// noteSinkReach records that a parameter element reaches a sink. The shorter
// path is kept; if the flow came under other guards than the one already
// known, the guards are dropped, whatever the lengths (ADR-035).
func (s *state) noteSinkReach(k sinkKey, sr *SinkReach) {
	old, ok := s.sinkReaches[k]
	if !ok {
		s.sinkReaches[k] = sr
		return
	}
	if m := mergeSinkReach(old, sr); m != old {
		s.sinkReaches[k] = m
	}
}

// mergeSinkReach joins two ways of reaching the same sink. It returns old
// itself when nothing changes; it never changes either argument.
func mergeSinkReach(old, nw *SinkReach) *SinkReach {
	keep := old
	if nw.Steps.len() < old.Steps.len() {
		keep = nw
	}
	if sameGuards(old.Guards, nw.Guards) {
		return keep
	}
	if len(keep.Guards) == 0 {
		return keep
	}
	c := *keep
	c.Guards = nil
	return &c
}

func (s *state) sinkStep(site *sites.Site, in *sg.Instr, note *Uncertainty) stepRec {
	return newRec(s.eng.funcID(s.fn), "sink: "+site.Symbol, packPos(in.Pos), nil, appendNote(s.sinkNotes, note))
}

// guardsAtSink are the conditions under which the flow reaches the sink: those
// of the block with the call, and those along the way. More than maxGuards
// of them are dropped (ADR-006), and the function is marked.
func (s *state) guardsAtSink(in *sg.Instr, f found) []Guard {
	chain, over := s.guardsOfChain(f.Cell, f.Orig)
	all := unionGuards(s.blockGuards(in.Block), chain)
	if over || len(all) > maxGuards {
		s.guardOverflow = true
		return nil
	}
	return all
}

func (s *state) recordFinding(origin string, site *sites.Site, label string, steps *rope, guards []Guard, unsure bool) {
	k := findingKey{origin, site.ID}
	rf := s.findings[k]
	if rf == nil {
		rf = &rawFinding{origin: origin, site: site, labels: map[string]bool{}}
		s.findings[k] = rf
	}
	rf.labels[label] = true
	switch {
	case rf.steps == nil:
		rf.steps = steps
		rf.guards = guards
		rf.unsure = unsure
	case rf.unsure && !unsure:
		// a way that tells the field: the finding is that one, with no uncertainty of the field
		rf.steps, rf.guards, rf.unsure = steps, guards, false
	case !rf.unsure && unsure:
		// a way that does not tell the field, beside one that does: nothing to add
	default:
		// The same flow came under other guards (or none): the guards of
		// the one kept would not describe it (ADR-035: they are dropped).
		if !sameGuards(rf.guards, guards) {
			guards = nil
		}
		rf.guards = guards
		if steps.len() < rf.steps.len() {
			rf.steps = steps
		}
	}
}

// --- builtins ---

func (s *state) builtin(cc *callCtx, name string) {
	args := cc.args
	switch name {
	case "append":
		if len(args) == 0 || len(cc.roots) == 0 {
			return
		}
		res := []Loc{{cc.roots[0], ""}}
		s.copyLocs(s.locs(args[0]), res, "append", cc.in)
		if len(args) > 1 {
			if pointerLike(args[1]) {
				s.copyLocs(s.locs(args[1]), res, "append", cc.in)
			} else {
				s.put(Loc{cc.roots[0], "/[]"}, s.flatten(s.locs(args[1])), s.whyFn(cc.in, "append"))
			}
		}
	case "copy":
		if len(args) < 2 {
			return
		}
		if pointerLike(args[1]) {
			s.copyLocs(s.locs(args[1]), s.locs(args[0]), "copy", cc.in)
		} else {
			for _, d := range s.locs(args[0]) {
				s.put(Loc{d.Root, pathJoin(d.Path, "[]")}, s.flatten(s.locs(args[1])), s.whyFn(cc.in, "copy"))
			}
		}
	case "ssa:wrapnilchk":
		if len(args) > 0 && len(cc.roots) > 0 {
			s.copyLocs(s.locs(args[0]), []Loc{{cc.roots[0], ""}}, "alias", cc.in)
		}
	case "min", "max":
		// the result is one of the arguments (a string compared, a number: the number stops where it does)
		for _, a := range args {
			if len(cc.roots) > 0 {
				s.copyLocs(s.locs(a), []Loc{{cc.roots[0], ""}}, name, cc.in)
			}
		}
	case "unsafe.String", "unsafe.Slice", "unsafe.StringData", "unsafe.SliceData", "unsafe.Add":
		// unsafe.String / Slice / StringData / SliceData / Add (the Go extractor names them with the package): the result is made of the first argument (the
		// pointer, or the data); the length does not flow
		if len(args) > 0 && len(cc.roots) > 0 {
			s.copyLocs(s.locs(args[0]), []Loc{{cc.roots[0], ""}}, "unsafe."+name, cc.in)
		}
	case "len", "cap", "complex", "real", "imag", "delete", "clear", "close", "print", "println", "ssa:deferstack":
		// a number, or no result: nothing flows (the number stops where a number does)
	case "recover":
		// the value of a panic is not followed (there is no edge from a panic to its recover): said where it is read
		s.newNote(cc.in, KindUnsupported, "recover(): the value of a panic is not followed, so what it holds is not traced", "")
	default:
		// a builtin that this version does not know (a Go that has more of them): the result is taken to come from the
		// arguments, more than the truth and never less, and it is said, on the way of the flow
		note := s.newNote(cc.in, KindUnsupported, "an unknown builtin function (builtin."+name+"): its result is taken to come from its arguments", "")
		if len(cc.roots) > 0 && len(args) > 0 {
			s.put(Loc{cc.roots[0], ""}, s.flatten(valuesLocs(s, args)), s.whyNote(cc.in, "builtin "+name+" (not known)", note))
		}
	}
}

// --- models ---

func (s *state) applyModel(cc *callCtx) {
	rules := s.eng.rules
	if cc.in.Callee != nil && cc.in.Callee.Kind == ir.CalleeDynamic && cc.recv != nil && rules.SelfFirst[cc.sym] {
		// a call through a trait whose model names the receiver arg0 (Rust): the receiver is the first of the
		// operands, as it is for a call of the function itself (Iterator::next, IntoIterator::into_iter)
		c2 := *cc
		c2.recv, c2.args = nil, append([]*sg.Value{cc.recv}, cc.args...)
		cc = &c2
	}
	for _, san := range rules.Sanitizers[cc.sym] {
		flows := san.Flows
		if len(flows) == 0 {
			flows = []model.Flow{{From: "args", To: "result"}}
			if cc.recv != nil {
				// a method: what it is called on is data too (`text.Clean()`), and what the sanitizer does not drop goes on
				flows = append(flows, model.Flow{From: "recv", To: "result"})
			}
		}
		o := op{drops: san.Drops, adds: san.Adds, scoped: san.Neutralises != nil}
		s.applyFlows(cc, flows, &o, "sanitized by "+cc.sym, modelRef(cc.sym, "sanitizer", san.Provenance))
	}
	for _, p := range rules.Propagators[cc.sym] {
		p := p
		if p.Boundary != "" {
			s.newNote(cc.in, KindStoredDataBoundary, "data moves between the code and a data store here; what is read back is not followed unless a source rule says what comes from it", cc.sym)
		}
		s.suppressedByCut(cc, &p, modelRef(cc.sym, "propagator", p.Provenance))
		if p.Pure {
			// nothing flows: that is the model. When what comes out is a number or a bool, and what went
			// in came from a source, this is where the input becomes a number: said, where it happens
			s.noteNumericStopOfPure(cc)
		} else if len(p.Flows) > 0 {
			s.applyFlows(cc, p.Flows, nil, "", modelRef(cc.sym, "propagator", p.Provenance))
			s.noteNumericStopOfUncovered(cc, p.Flows)
		}
		for i := range p.Calls {
			s.applyCallRow(cc, &p, i, modelRef(cc.sym, "propagator", p.Provenance))
		}
	}
	if s.eng.opts.FuncValues {
		s.countDroppedFuncValues(cc)
	}
}

// noteNumericStopOfUncovered is noteNumericStopOfPure for a model that has flows, none of them to a result that is
// a number or a bool (strconv.Atoi: the input goes to the error only): the number is where the input stops (R-03).
func (s *state) noteNumericStopOfUncovered(cc *callCtx, flows []model.Flow) {
	toResult := false
	for _, fl := range flows {
		if to, err := model.ParseSelector(fl.To); err == nil && to.Kind == model.SelResult {
			toResult = true
		}
	}
	if !toResult {
		return // a model that writes to an argument (Buffer.Write) is not one that turns its input into a number
	}
	var numeric *sg.Value
	for i, d := range cc.in.Dsts {
		if d == nil || (d.Class != ir.ClassNumber && d.Class != ir.ClassBool) {
			continue
		}
		covered := false
		for _, fl := range flows {
			if to, err := model.ParseSelector(fl.To); err == nil && to.Kind == model.SelResult && (to.All || to.Index == i) {
				covered = true
			}
		}
		if !covered {
			numeric = d
			break
		}
	}
	if numeric == nil {
		return
	}
	for _, f := range s.flatten(valuesLocs(s, cc.argv())) {
		s.countNumericStop(numeric, f.E)
		if f.E.Kind == Real {
			s.noteNumericStop(f.E, nil, numeric)
		}
	}
}

// noteNumericStopOfPure records the data of the sources that goes into a call that the model
// says is pure, and comes out as a number or a bool (strconv.Atoi, len-like functions).
func (s *state) noteNumericStopOfPure(cc *callCtx) {
	var numeric *sg.Value
	for _, d := range cc.in.Dsts {
		if d != nil && (d.Class == ir.ClassNumber || d.Class == ir.ClassBool) {
			numeric = d
			break
		}
	}
	if numeric == nil {
		return
	}
	for _, f := range s.flatten(valuesLocs(s, cc.argv())) {
		s.countNumericStop(numeric, f.E)
		if f.E.Kind == Real {
			s.noteNumericStop(f.E, nil, numeric)
		}
	}
}

func (s *state) selectVals(cc *callCtx, sel model.Selector) []*sg.Value {
	switch sel.Kind {
	case model.SelRecv:
		if cc.recv != nil {
			return []*sg.Value{cc.recv}
		}
	case model.SelArg:
		if sel.All {
			return cc.args
		}
		if sel.Index < len(cc.args) {
			return []*sg.Value{cc.args[sel.Index]}
		}
	}
	return nil
}

// suppressed records that a model decided by an LLM removed a label.
func (s *state) suppressed(cc *callCtx, m *ModelRef, e Elem) {
	if s.suppressions == nil {
		s.suppressions = map[string]*Suppression{}
	}
	key := m.Symbol + "|" + posKey(cc.in.Pos)
	sp := s.suppressions[key]
	if sp == nil {
		sp = &Suppression{Model: *m, Func: s.eng.funcID(s.fn), pos: cc.in.Pos}
		s.suppressions[key] = sp
	}
	sp.Labels = addSorted(sp.Labels, e.Label())
	sp.Origins = addSorted(sp.Origins, e.Origin())
}

// suppressedByCut records what a model that is not a built-in one (decided by an LLM, or written by a person in
// a rule file) does not let through to a result of a call: with `pure`, or with flows that leave an operand out
// of a result, what the call would have returned from that operand (the guess of a call that is not modelled)
// is gone. The finding that this removes cannot say so; the record can. A built-in model is not recorded: it
// is checked by a person and by the tests, and recording every one of them would fill the output. A result that is
// a number or a bool is the business of the numeric stop, not of this record.
func (s *state) suppressedByCut(cc *callCtx, p *model.Propagator, mref *ModelRef) {
	if mref == nil || mref.Origin == model.OriginBuiltin || len(cc.in.Dsts) == 0 {
		return
	}
	for i, d := range cc.in.Dsts {
		if d == nil || d.Class == ir.ClassNumber || d.Class == ir.ClassBool {
			continue
		}
		kept := map[*sg.Value]bool{}
		if !p.Pure {
			for _, fl := range p.Flows {
				to, err1 := model.ParseDest(fl.To)
				from, err2 := model.ParseSelector(fl.From)
				if err1 != nil || err2 != nil || to.Kind != model.SelResult || !(to.All || to.Index == i) {
					continue
				}
				for _, v := range s.selectVals(cc, from) {
					kept[v] = true
				}
			}
		}
		for _, v := range cc.argv() {
			if kept[v] {
				continue
			}
			for _, f := range s.flatten(s.locs(v)) {
				if f.E.Kind == Real {
					s.suppressed(cc, mref, f.E)
				}
			}
		}
	}
}

func addSorted(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	list = append(list, v)
	sort.Strings(list)
	return list
}

func modelRef(symbol, role string, p model.Provenance) *ModelRef {
	return &ModelRef{Symbol: symbol, Role: role, Origin: p.Origin, DecidedAt: p.DecidedAt, ModelVersion: p.ModelVersion, Rationale: p.Rationale}
}

func (s *state) applyFlows(cc *callCtx, flows []model.Flow, o *op, desc string, mref *ModelRef) {
	for _, fl := range flows {
		from, err1 := model.ParseSelector(fl.From)
		to, err2 := model.ParseDest(fl.To)
		if err1 != nil || err2 != nil {
			continue
		}
		var src []found
		for _, v := range s.selectVals(cc, from) {
			src = append(src, s.flatten(s.locs(v))...)
		}
		if len(src) == 0 {
			continue
		}
		var approx []*Uncertainty
		if fl.Approximate != "" {
			approx = []*Uncertainty{s.newNote(cc.in, KindModelApproximation, fl.Approximate, cc.sym)}
		}
		if fl.Assumed != "" {
			approx = append(approx, s.newNote(cc.in, KindAssumedCall, fl.Assumed, cc.sym))
		}
		var dsts []Loc
		if to.Kind == model.SelResult {
			if to.All {
				for _, r := range cc.roots {
					dsts = append(dsts, Loc{r, ""})
				}
			} else if to.Index < len(cc.roots) {
				dsts = append(dsts, Loc{cc.roots[to.Index], ""})
			}
		} else {
			for _, v := range s.selectVals(cc, to) {
				here := append(append([]Loc(nil), s.locs(v)...), s.pointeeOfInterface(v)...)
				if to.Content && s.contentOnly(to, v, s.eng.rules.SelfFirst[cc.sym]) {
					// what was written goes below the writer, in a step of its own: a field of the
					// writer (a file name, a setting) is not what was written to it, and a read of the
					// whole writer (`from: recv`) still sees it
					for i := range here {
						here[i].Path = pathJoin(here[i].Path, model.ContentStep)
					}
				}
				dsts = append(dsts, here...)
				// a variadic destination: the pointers that were stored in the slice the call was given
				// (row.Columns(&a, &b)) are where the data goes, as for a source that fills its arguments
				for _, e := range s.variadicElems(v) {
					dsts = append(dsts, s.locs(e)...)
					dsts = append(dsts, s.pointeeOfInterface(e)...)
				}
			}
		}
		for _, d := range dsts {
			for _, f := range src {
				elems := []Elem{f.E}
				kind := "propagated by " + cc.sym
				if o != nil {
					elems = sanitize(elems, *o, s.eng.rules)
					kind = desc
					if mref != nil && mref.Origin == model.OriginLLM && f.E.Kind == Real && o.drops1(f.E.Label()) {
						s.suppressed(cc, mref, f.E)
					}
				}
				for _, e := range elems {
					wm := newWhyFrom(s.fn, kind, f.Cell, f.Orig)
					wm.pos = packPos(cc.in.Pos)
					if f.Cut {
						wm = s.withCutNote(wm, f.Cell.Root, f.E.Path(), -1, "read")
					}
					wm.setModel(mref)
					if approx != nil {
						wm.setNotes(append(append([]*Uncertainty(nil), wm.notes()...), approx...))
					}
					s.add(d, e, wm)
				}
			}
		}
	}
}

// contentOnly reports whether what a flow writes goes only below the writer (ADR-082), and not to the
// whole writer as well. The writer itself (`recv.content`: a method of the writer, which the body of
// the project's own implementation is applied for) and a writer of the library that holds what it is
// written in a way its own models read back (bytes.Buffer, strings.Builder) are. A writer that is given
// to a function that does not call its Write (`fmt.Fprint(w, x)`, `io.Copy(w, r)`) and that is of a type
// of the project, an interface or any other library type, is not: the body of its Write is not applied,
// so the data must reach the fields it is stored in (`w.data`, `rec.Body`) through the whole writer.
func (s *state) contentOnly(to model.Selector, v *sg.Value, selfFirst bool) bool {
	// in a set with self_first (Rust) the receiver is arg0
	if to.Kind == model.SelRecv || (selfFirst && to.Kind == model.SelArg && to.Index == 0) {
		// a writer of the Rust standard library (`Vec<u8>`, `String`, `File`) has no body to apply, and its
		// models read the whole writer
		if isStdType(string(v.Type)) {
			return false
		}
		return true
	}
	t := strings.TrimLeft(string(v.Type), "*")
	return t == "bytes.Buffer" || t == "strings.Builder"
}

// unmodeled is the conservative default for a call whose callee is not
// known: everything that goes in may come out. The result is recorded as
// uncertainty so that a reviewer sees the guess.
func (s *state) unmodeled(cc *callCtx, note *Uncertainty) {
	if cc.recv != nil {
		note.Recv = &Operand{Type: cc.recv.Type, Class: cc.recv.Class}
	}
	note.Operands = nil
	for _, a := range cc.args {
		if a == nil {
			note.Operands = append(note.Operands, Operand{})
			continue
		}
		note.Operands = append(note.Operands, Operand{Type: a.Type, Class: a.Class})
	}
	s.noteOpaque(cc)
	src := s.flatten(valuesLocs(s, cc.argv()))
	if s.eng.opts.PassStats && cc.sym != "" {
		s.eng.noteUnmodeled(cc.sym, len(src) > 0)
	}
	for i, r := range cc.roots {
		if i < len(cc.in.Dsts) && cc.in.Dsts[i].Class == ir.ClassBool {
			continue
		}
		s.put(Loc{r, ""}, src, s.whyNote(cc.in, "call "+orUnknown(cc.sym)+" (unmodeled)", note))
	}
}

func valuesLocs(s *state, vals []*sg.Value) []Loc {
	var out []Loc
	for _, v := range vals {
		out = append(out, s.locs(v)...)
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "<unknown>"
	}
	return s
}

// pointeeOfInterface is where a pointer that was converted to an interface
// (json.Unmarshal(b, &x): the argument is any(&x)) points: a model that writes
// to the argument writes to what the pointer refers to.
func (s *state) pointeeOfInterface(v *sg.Value) []Loc {
	var out []Loc
	for i := 0; i < 4 && v != nil && v.Kind == sg.KResult && v.Instr != nil; i++ {
		in := v.Instr
		if in.Op != ir.OpConvert || in.IR == nil || len(in.Args) != 1 {
			break
		}
		switch in.IR.Operator {
		case "make_interface":
			if pointerLike(in.Args[0]) {
				out = append(out, s.locs(in.Args[0])...)
			}
			return out
		case "change_interface":
			v = in.Args[0]
		default:
			return out
		}
	}
	return out
}

// variadicElems are the values that were stored into the slice v is a view of,
// when v is the slice of a variadic call made in this function: the pointers a
// call like rows.Scan(&a, &b) is given. Memory does not alias in the
// analysis (a pointer stored in a slice is its content), so the places they
// point to are found here, where they were stored.
func (s *state) variadicElems(v *sg.Value) []*sg.Value {
	for i := 0; i < 4 && v != nil && v.Kind == sg.KResult && v.Instr != nil && v.Instr.Op == ir.OpCopy && len(v.Instr.Args) == 1; i++ {
		v = v.Instr.Args[0]
	}
	if v == nil || v.Kind != sg.KResult || v.Instr == nil || v.Instr.Op != ir.OpAlloc {
		return nil
	}
	var out []*sg.Value
	for _, b := range s.fn.Blocks {
		for _, in := range b.Instrs {
			if in.Op == ir.OpIndexStore && len(in.Args) >= 3 && in.Args[0] == v {
				out = append(out, in.Args[2])
			}
		}
	}
	return out
}

// noteDepBodies records, for a call of a modelled interface method, the candidates whose body is in the
// program but belongs to a dependency: they are not read (the model stands for them), and a dynamic_dispatch
// cannot tell that from a candidate that was never there (dependency_body_unread).
func (s *state) noteDepBodies(ci *sg.Instr, cc *callCtx, fns []callee) {
	var ids []string
	for _, c := range fns {
		if c.fn != nil && c.fn.Dep {
			ids = append(ids, c.id)
		}
	}
	if len(ids) == 0 {
		return
	}
	sort.Strings(ids)
	n := s.newNote(ci, KindDependencyBodyUnread, fmt.Sprintf("%d candidate(s) of %s have a body in a dependency that is not read (the model stands for them): what they hand on to an implementation of the project is not followed (rivus' limit, ADR-075)", len(ids), cc.sym), cc.sym)
	n.Candidates = ids
}

// staticImplOfModelledTrait: a call that the compiler resolved (Rust: the type is known, the call is of the impl
// `<A as std::io::Write>::write`) and that the extractor names by the trait's method, which has a model. It is the
// same method as the dynamic call of the trait, with the impl known: the model says what the implementations outside
// the code do, and the impl of the project is read besides (the sum, as for a dynamic call).
func staticImplOfModelledTrait(ci *sg.Instr) bool {
	if ci.Callee == nil || ci.Callee.Kind != ir.CalleeStatic || ci.Static == nil || ci.Static.Dep {
		return false
	}
	id := string(ci.StaticID)
	return strings.HasPrefix(id, "<") && strings.Contains(id, " as ") && id != ci.Callee.Symbol
}

// hasOwnBody reports whether one of the callees is code of the project (not of a
// dependency) that the program has.
func hasOwnBody(fns []callee) bool {
	for _, c := range fns {
		if c.fn != nil && !c.fn.Dep {
			return true
		}
	}
	return false
}

// captured applies the summary of a closure at the place where it is made, to the values it
// captured (C-7, ADR-059). A closure that is given to another function, stored or returned is
// called where it cannot be seen from here: what the call would do with what it captured is
// done here, and it is taken to be called (assumed_call). A closure that is only called in this
// function (the call, go, defer) is seen by the call, which has the values: nothing is done.
//
// Not applied: what the closure throws, and its result, which are for the place that calls it;
// the closure's own parameters, which are what the caller gives it (nothing is known of them
// here: they carry nothing). The two that are not applied are said, if they depend on the
// values captured.
func (s *state) captured(in *sg.Instr) {
	if len(in.Args) == 0 || !in.Escapes || in.IR == nil || in.IR.Func == "" {
		return
	}
	fn := s.eng.prog.ByID[string(in.IR.Func)]
	if fn == nil || len(in.Args) != len(fn.FreeVars) {
		return
	}
	if s.coveredByModel(in) {
		return // the call it is given to applies it, with what it captured: nothing is assumed
	}
	sigOf := func() int {
		sum := 1
		for _, v := range in.Args {
			for _, l := range s.locs(v) {
				sum += s.rootVer[l.Root]
			}
		}
		return sum
	}
	// the cells of the captured values are all it reads: if none grew, there is nothing to add
	before := sigOf()
	if prev, ok := s.callSigs[in]; ok && prev == before {
		return
	}
	s.applyMade(in, fn)
	if s.callSigs == nil {
		s.callSigs = map[*sg.Instr]int{}
	}
	if sigOf() == before {
		s.callSigs[in] = before
	} else {
		delete(s.callSigs, in)
	}
}

func (s *state) applyMade(in *sg.Instr, fn *sg.Func) {
	sum := s.eng.summaryOf(fn)
	if sum == nil {
		return // not analysed yet; a later round picks it up
	}
	declared := len(fn.Params)
	if !usesCaptures(sum, declared) {
		return
	}
	detail := "assumed_call: the closure made here is given to another function, stored or returned: where it is called is not seen, and what it captured is applied here, as if it were called"
	if capturedDepends(sum, declared, OutThrow) {
		detail += "; what it throws is not applied"
	}
	if capturedDepends(sum, declared, OutResult) {
		detail += "; what it returns is not applied"
	}
	u := s.newNote(in, KindAssumedCall, detail, s.eng.funcID(fn))
	args := make([]*sg.Value, declared)
	for i := range args {
		args[i] = &sg.Value{Kind: sg.KUndef}
	}
	cc := &callCtx{in: in, args: args, made: true}
	s.applySummary(cc, fn, in.Args, []*Uncertainty{u}, nil)
}

// capturedDepends reports whether an output of the kind carries something of the values the
// function captured (the parameters after the declared ones).
func capturedDepends(sum *Summary, declared int, kind string) bool {
	for _, o := range sum.Outputs {
		if o.Kind != kind {
			continue
		}
		for _, oe := range o.Elems {
			if oe.E.Kind == Param && int(oe.E.Param) >= declared {
				return true
			}
		}
	}
	return false
}

// noteNarrowed says on the uncertainty of a call that its candidates were narrowed to the functions that flow to
// the receiver (internal/valuecalls), and the assumption it rests on.
func (s *state) noteNarrowed(u *Uncertainty, ci *sg.Instr) {
	c := ci.Callee
	if c == nil || c.NarrowedFrom == 0 {
		return
	}
	u.NarrowedFrom = c.NarrowedFrom
	u.Basis = c.NarrowBasis
	u.Detail += fmt.Sprintf("; the candidates were narrowed from %d to %d (%s)", c.NarrowedFrom, len(c.Candidates), c.NarrowFrom)
	if c.NarrowBasis == ir.BasisClosedWorldEntry {
		u.Detail += "; basis closed_world_entry: the program was read as one that starts at its entry point, so a call from outside it is not counted"
	}
}

// paramGuards is dispatchGuards for a call of a closure (Rust, Fn::call) whose candidates were narrowed to the
// functions that flow to the receiver: a candidate that came only through one parameter of this function is
// called only if the function given as that parameter is it, and the caller, which knows what it passed, can
// leave out the flows through the other candidates (the guard of valueGuards).
func (s *state) paramGuards(cc *callCtx, fns []*sg.Func) map[*sg.Func][]Guard {
	if m, ok := s.eng.dispatch.Load(cc.in); ok {
		return m.(map[*sg.Func][]Guard)
	}
	var out map[*sg.Func][]Guard
	for _, fn := range fns {
		p, ok := cc.in.Callee.CandParam[ir.FuncID(fn.ID)]
		if !ok || p < 0 || p >= len(s.fn.Params) {
			continue
		}
		if out == nil {
			out = map[*sg.Func][]Guard{}
		}
		out[fn] = []Guard{{
			Func:     s.eng.funcID(s.fn),
			Params:   []int{p},
			Cond:     "the function given as " + s.fn.Params[p].Name + " is " + fn.ID,
			Polarity: true,
			disp:     &dispatchSpec{fnID: fn.ID},
			fn:       s.fn,
			pos:      cc.in.Pos,
		}}
		out[fn][0].k = out[fn][0].computeKey()
	}
	s.eng.dispatch.Store(cc.in, out)
	return out
}
