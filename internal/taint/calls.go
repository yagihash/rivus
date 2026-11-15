package taint

import (
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

// callSig changes whenever something is added to a cell the call reads. The
// versions only grow, so their sum is enough.
func (s *state) callSig(cc *callCtx) int {
	sum := 1
	add := func(v *sg.Value) {
		for _, l := range s.locs(v) {
			sum += s.rootVer[l.Root]
		}
	}
	for _, v := range cc.argv() {
		add(v)
	}
	for _, b := range cc.in.Bindings {
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
	for _, site := range s.eng.sites.ByIRInstr[sites.IRInstr{Func: s.fn.IR.ID, Block: ci.IRBlock, Instr: ci.IRIndex}] {
		switch site.Role {
		case sites.RoleSource:
			for _, r := range cc.roots {
				for _, l := range site.Labels {
					s.add(Loc{r, ""}, Elem{Kind: Real, Label: l, Origin: site.ID}, s.origin(ci.Pos, "source: "+site.Symbol))
				}
			}
		case sites.RoleSink:
			s.checkSink(cc, site)
		}
	}

	if name := builtinName(ci); name != "" {
		s.builtin(cc, name)
		return
	}
	rules := s.eng.rules
	if cc.sym != "" && rules.HasModel(cc.sym) {
		s.applyModel(cc)
		return
	}

	fns, bindings := s.callees(ci)
	var external bool
	var withBody []*sg.Func
	for _, c := range fns {
		if c.fn != nil {
			withBody = append(withBody, c.fn)
		} else {
			external = true
		}
	}
	ruleOnly := cc.sym != "" && (len(rules.CallSources[cc.sym]) > 0 || len(rules.Sinks[cc.sym]) > 0)

	var notes []*Uncertainty
	var dispatch *Uncertainty
	invoke := ci.Callee.Kind == ir.CalleeDynamic
	if invoke || len(fns) > 1 {
		dispatch = s.newNote(ci, KindDynamicDispatch, "call through an interface or function value", cc.sym)
		for _, c := range fns {
			dispatch.Candidates = append(dispatch.Candidates, c.id)
		}
		sort.Strings(dispatch.Candidates)
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
	if ci.IR.Go {
		notes = append(notes, s.pathNote(KindConcurrency, "the callee runs in another goroutine; the order of effects is not modelled", ci.Pos))
	}
	for _, fn := range withBody {
		b := bindings
		if cb, ok := ci.CandidateBindings[fn]; ok {
			b = cb
		}
		s.applySummary(cc, fn, b, notes)
	}
	switch {
	case ruleOnly:
		// the rule is the model
	case len(fns) == 0 && invoke:
		s.unmodeled(cc, dispatch)
	case len(fns) == 0:
		s.unmodeled(cc, s.newNote(ci, KindUnresolvedCallee, "the called function is not known", cc.sym))
	case external:
		ids := make([]string, len(fns))
		for i, c := range fns {
			ids[i] = c.id
		}
		kind, detail := externalKind(ids, cc.sym)
		s.unmodeled(cc, s.newNote(ci, kind, detail, cc.sym))
	}
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
}

type sinkKey struct {
	e    Elem
	site string
}

// SinkReach says: taint coming from the symbolic element reaches a sink site.
type SinkReach struct {
	Elem    Elem
	Site    string
	Accepts []string
	Steps   *rope
	Guards  []Guard
}

func (s *state) checkSink(cc *callCtx, site *sites.Site) {
	var vals []*sg.Value
	for _, i := range sites.SinkArgs(site.Sink, len(cc.args)) {
		vals = append(vals, cc.args[i])
	}
	for _, sk := range site.Sink {
		if sk.Recv && cc.recv != nil {
			vals = append(vals, cc.recv)
			break
		}
	}
	for _, v := range vals {
		for _, f := range s.flatten(s.locs(v)) {
			s.reach(site, f, cc.in)
		}
	}
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

func (s *state) reach(site *sites.Site, f found, in *sg.Instr) {
	steps := cat(s.trace(f.Cell, f.Orig), leaf(s.sinkStep(site, in)))
	gs := s.guardsAtSink(in, f)
	switch f.E.Kind {
	case Real:
		if !accepted(site.Labels, f.E.Label) {
			return
		}
		s.recordFinding(f.E.Origin, site, f.E.Label, steps, gs)
	case Param:
		k := sinkKey{f.E, site.ID}
		if old, ok := s.sinkReaches[k]; !ok || steps.len() < old.Steps.len() {
			s.sinkReaches[k] = &SinkReach{Elem: f.E, Site: site.ID, Accepts: site.Labels, Steps: steps, Guards: gs}
		}
	}
}

func (s *state) sinkStep(site *sites.Site, in *sg.Instr) Step {
	return Step{Func: s.eng.funcID(s.fn), Desc: "sink: " + site.Symbol, pos: in.Pos}
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

func (s *state) recordFinding(origin string, site *sites.Site, label string, steps *rope, guards []Guard) {
	k := findingKey{origin, site.ID}
	rf := s.findings[k]
	if rf == nil {
		rf = &rawFinding{origin: origin, site: site, labels: map[string]bool{}}
		s.findings[k] = rf
	}
	rf.labels[label] = true
	if rf.steps == nil || steps.len() < rf.steps.len() {
		rf.steps = steps
		rf.guards = guards
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
	}
}

// --- models ---

func (s *state) applyModel(cc *callCtx) {
	rules := s.eng.rules
	for _, san := range rules.Sanitizers[cc.sym] {
		flows := san.Flows
		if len(flows) == 0 {
			flows = []model.Flow{{From: "args", To: "result"}}
		}
		o := op{drops: san.Drops, adds: san.Adds}
		s.applyFlows(cc, flows, &o, "sanitized by "+cc.sym, modelRef(cc.sym, "sanitizer", san.Provenance))
	}
	for _, p := range rules.Propagators[cc.sym] {
		if p.Pure {
			continue // nothing flows: that is the model
		}
		s.applyFlows(cc, p.Flows, nil, "", modelRef(cc.sym, "propagator", p.Provenance))
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
	sp.Labels = addSorted(sp.Labels, e.Label)
	sp.Origins = addSorted(sp.Origins, e.Origin)
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
		to, err2 := model.ParseSelector(fl.To)
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
				dsts = append(dsts, s.locs(v)...)
				dsts = append(dsts, s.pointeeOfInterface(v)...)
			}
		}
		for _, d := range dsts {
			for _, f := range src {
				elems := []Elem{f.E}
				kind := "propagated by " + cc.sym
				if o != nil {
					elems = sanitize(elems, *o)
					kind = desc
					if mref != nil && mref.Origin == model.OriginLLM && f.E.Kind == Real && o.drops1(f.E.Label) {
						s.suppressed(cc, mref, f.E)
					}
				}
				for _, e := range elems {
					s.add(d, e, &why{fn: s.fn, pos: cc.in.Pos, desc: kind, from: &ref{f.Cell, f.Orig}, model: mref})
				}
			}
		}
	}
}

// unmodeled is the conservative default for a call whose callee is not
// known: everything that goes in may come out. The result is recorded as
// uncertainty so that a reviewer sees the guess.
func (s *state) unmodeled(cc *callCtx, note *Uncertainty) {
	src := s.flatten(valuesLocs(s, cc.argv()))
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
