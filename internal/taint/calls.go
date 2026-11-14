package taint

import (
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// callCtx is a call with its operands laid out.
type callCtx struct {
	in    ssa.CallInstruction
	recv  ssa.Value   // nil for plain functions
	args  []ssa.Value // receiver excluded
	roots []Root      // one per result
	sym   string
}

// argv lists the operands in parameter order (receiver first).
func (c *callCtx) argv() []ssa.Value {
	var out []ssa.Value
	if c.recv != nil {
		out = append(out, c.recv)
	}
	return append(out, c.args...)
}

func (s *state) call(ci ssa.CallInstruction) {
	common := ci.Common()
	cc := &callCtx{in: ci, sym: gossa.CalleeSymbol(common)}
	cc.recv, cc.args = gossa.CallArgs(common)
	cc.roots = callRoots(ci)

	// Sources and sinks matched by the rules at this call.
	for _, site := range s.eng.sites.ByInstr[ci] {
		switch site.Role {
		case sites.RoleSource:
			for _, r := range cc.roots {
				for _, l := range site.Labels {
					s.add(Loc{r, ""}, Elem{Kind: Real, Label: l, Origin: site.ID}, s.origin(ci.Pos(), "source: "+site.Symbol))
				}
			}
		case sites.RoleSink:
			s.checkSink(cc, site)
		}
	}

	if b, ok := common.Value.(*ssa.Builtin); ok {
		s.builtin(cc, b)
		return
	}
	rules := s.eng.rules
	if cc.sym != "" && rules.HasModel(cc.sym) {
		s.applyModel(cc)
		return
	}

	fns, bindings := s.callees(ci)
	var external bool
	var withBody []*ssa.Function
	for _, fn := range fns {
		if s.eng.hasBody(fn) {
			withBody = append(withBody, fn)
		} else {
			external = true
		}
	}
	ruleOnly := cc.sym != "" && (len(rules.CallSources[cc.sym]) > 0 || len(rules.Sinks[cc.sym]) > 0)

	var note *Uncertainty
	if common.IsInvoke() || len(fns) > 1 {
		note = s.newNote(ci, KindDynamicDispatch, "call through an interface or function value", cc.sym)
		for _, fn := range fns {
			note.Candidates = append(note.Candidates, s.eng.funcID(fn))
		}
		sort.Strings(note.Candidates)
	}
	for _, fn := range withBody {
		s.applySummary(cc, fn, bindings, note)
	}
	switch {
	case ruleOnly:
		// the rule is the model
	case len(fns) == 0 && common.IsInvoke():
		s.unmodeled(cc, s.newNote(ci, KindDynamicDispatch, "no implementation with a body is known", cc.sym))
	case len(fns) == 0:
		s.unmodeled(cc, s.newNote(ci, KindUnresolvedCallee, "the called function is not known", cc.sym))
	case external:
		s.unmodeled(cc, s.newNote(ci, KindUnmodeledLibraryCall, "the function has no body and no model", cc.sym))
	}
}

func callRoots(ci ssa.CallInstruction) []Root {
	v, ok := ci.(*ssa.Call)
	if !ok {
		return nil
	}
	if tt, ok := v.Type().(*types.Tuple); ok {
		out := make([]Root, tt.Len())
		for i := range out {
			out[i] = Root{v, i}
		}
		return out
	}
	return []Root{own(v)}
}

func resultTypes(ci ssa.CallInstruction) []types.Type {
	v, ok := ci.(*ssa.Call)
	if !ok {
		return nil
	}
	if tt, ok := v.Type().(*types.Tuple); ok {
		out := make([]types.Type, tt.Len())
		for i := range out {
			out[i] = tt.At(i).Type()
		}
		return out
	}
	return []types.Type{v.Type()}
}

// callees resolves the possible functions called, and the values captured
// by the closure when the callee is a closure created right there.
func (s *state) callees(ci ssa.CallInstruction) ([]*ssa.Function, []ssa.Value) {
	c := ci.Common()
	if fn := c.StaticCallee(); fn != nil {
		var binds []ssa.Value
		if mc, ok := c.Value.(*ssa.MakeClosure); ok {
			binds = mc.Bindings
		}
		return []*ssa.Function{fn}, binds
	}
	if s.eng.graph != nil {
		return s.eng.graph.Callees(ci), nil
	}
	return nil, nil
}

func (s *state) newNote(in ssa.Instruction, kind, detail, symbol string) *Uncertainty {
	u := newUncertainty(kind, detail, symbol)
	u.Func = s.eng.funcID(s.fn)
	u.pos = in.Pos()
	s.uncertain = append(s.uncertain, u)
	return u
}

// --- sinks ---

type findingKey struct{ origin, sink string }

type rawFinding struct {
	origin string
	site   *sites.Site
	labels map[string]bool
	steps  []Step
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
	Steps   []Step
}

func (s *state) checkSink(cc *callCtx, site *sites.Site) {
	var vals []ssa.Value
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

func (s *state) reach(site *sites.Site, f found, in ssa.Instruction) {
	steps := s.trace(f.Cell, f.Orig)
	if f.E.Path != f.Orig.Path {
		// nothing to add: the extension is a read of an enclosing object
	}
	steps = append(steps, s.sinkStep(site, in))
	switch f.E.Kind {
	case Real:
		if !accepted(site.Labels, f.E.Label) {
			return
		}
		s.recordFinding(f.E.Origin, site, f.E.Label, steps)
	case Param:
		k := sinkKey{f.E, site.ID}
		if old, ok := s.sinkReaches[k]; !ok || len(steps) < len(old.Steps) {
			s.sinkReaches[k] = &SinkReach{Elem: f.E, Site: site.ID, Accepts: site.Labels, Steps: steps}
		}
	}
}

func (s *state) sinkStep(site *sites.Site, in ssa.Instruction) Step {
	return Step{Func: s.eng.funcID(s.fn), Desc: "sink: " + site.Symbol, pos: in.Pos()}
}

func (s *state) recordFinding(origin string, site *sites.Site, label string, steps []Step) {
	k := findingKey{origin, site.ID}
	rf := s.findings[k]
	if rf == nil {
		rf = &rawFinding{origin: origin, site: site, labels: map[string]bool{}}
		s.findings[k] = rf
	}
	rf.labels[label] = true
	if rf.steps == nil || len(steps) < len(rf.steps) {
		rf.steps = steps
	}
}

// --- builtins ---

func (s *state) builtin(cc *callCtx, b *ssa.Builtin) {
	args := cc.args
	switch b.Name() {
	case "append":
		if len(args) == 0 || len(cc.roots) == 0 {
			return
		}
		res := []Loc{{cc.roots[0], ""}}
		s.copyLocs(s.locs(args[0]), res, "append", cc.in)
		if len(args) > 1 {
			if pointerLike(args[1].Type()) {
				s.copyLocs(s.locs(args[1]), res, "append", cc.in)
			} else {
				s.put(Loc{cc.roots[0], "/[]"}, s.flatten(s.locs(args[1])), s.whyFn(cc.in, "append"))
			}
		}
	case "copy":
		if len(args) < 2 {
			return
		}
		if pointerLike(args[1].Type()) {
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
		s.applyFlows(cc, flows, &o, "sanitized by "+cc.sym)
	}
	for _, p := range rules.Propagators[cc.sym] {
		s.applyFlows(cc, p.Flows, nil, "")
	}
}

func (s *state) selectVals(cc *callCtx, sel model.Selector) []ssa.Value {
	switch sel.Kind {
	case model.SelRecv:
		if cc.recv != nil {
			return []ssa.Value{cc.recv}
		}
	case model.SelArg:
		if sel.All {
			return cc.args
		}
		if sel.Index < len(cc.args) {
			return []ssa.Value{cc.args[sel.Index]}
		}
	}
	return nil
}

func (s *state) applyFlows(cc *callCtx, flows []model.Flow, o *op, desc string) {
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
			}
		}
		for _, d := range dsts {
			for _, f := range src {
				elems := []Elem{f.E}
				kind := "propagated by " + cc.sym
				if o != nil {
					elems = sanitize(elems, *o)
					kind = desc
				}
				for _, e := range elems {
					s.add(d, e, &why{fn: s.fn, pos: cc.in.Pos(), desc: kind, from: &ref{f.Cell, f.Orig}})
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
	types := resultTypes(cc.in)
	for i, r := range cc.roots {
		if i < len(types) && isBool(types[i]) {
			continue
		}
		s.put(Loc{r, ""}, src, s.whyNote(cc.in, "call "+orUnknown(cc.sym)+" (unmodeled)", note))
	}
}

func valuesLocs(s *state, vals []ssa.Value) []Loc {
	var out []Loc
	for _, v := range vals {
		out = append(out, s.locs(v)...)
	}
	return out
}

func isBool(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

func orUnknown(s string) string {
	if s == "" {
		return "<unknown>"
	}
	return s
}

var _ = token.ADD
