package taint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"

	"github.com/yagihash/rivus/internal/fingerprint"
)

// Output kinds of a summary.
const (
	OutResult = "result"
	OutParam  = "param"
	OutGlobal = "global"
	// OutThrow is what the function throws: the values of its throw statements
	// and of the calls that may throw. The caller reads it where it catches.
	OutThrow = "throw"
)

// excIdx is the result slot of the value a function throws (a function has
// its results at 0, 1, ...). All the exceptions of a function are one value:
// which throw a handler receives is not told apart (more than the truth, never
// less).
const excIdx = -2

func excRoot() Root { return Root{nil, excIdx} }

// OutElem is an element of an output with the steps that led to it inside
// the function.
type OutElem struct {
	E     Elem
	Steps *rope
	// Guards are the conditions, depending on parameters, under which this
	// element reaches the output.
	Guards []Guard
}

// Output says which taint ends up in a result, in (the object behind) a
// parameter, or in a global variable.
type Output struct {
	Kind   string
	Index  int       // result or parameter index
	Global *sg.Value // OutGlobal
	Path   string
	Elems  []OutElem
}

// Summary is what the rest of the program needs to know about a function.
type Summary struct {
	Func    string
	Outputs []Output
	Sinks   []*SinkReach
	// Opaque are the parameters whose memory a function that is not read had (opaqueParams)
	Opaque []int
	Sig    string
}

// summarize turns the final state of the analysis into a summary.
func (s *state) summarize() *Summary {
	sum := &Summary{Func: s.eng.funcID(s.fn)}

	roots := make([]Root, 0, len(s.cells))
	for r := range s.cells {
		roots = append(roots, r)
	}
	sort.Slice(roots, func(i, j int) bool {
		a, b := roots[i], roots[j]
		if s.rootNum(a) != s.rootNum(b) {
			return s.rootNum(a) < s.rootNum(b)
		}
		if a.Idx != b.Idx {
			return a.Idx < b.Idx
		}
		// package variables have no number: by name
		if a.V != nil && b.V != nil {
			return a.V.Less(b.V)
		}
		return false
	})
	for _, r := range roots {
		var kind string
		var idx int
		var glob *sg.Value
		switch {
		case r.V == nil && r.Idx == excIdx:
			kind, idx = OutThrow, 0
		case r.V == nil:
			kind, idx = OutResult, r.Idx
		case s.paramRoot[r] != 0 || s.isParamRoot(r):
			kind, idx = OutParam, s.paramRoot[r]
		default:
			if r.V.Kind != sg.KGlobal {
				continue
			}
			if s.eng.rules.HoldsNothing(r.V.Name) {
				// read back by nobody (a rule says so, ADR-056): the function is recorded, once for the variable
				for _, set := range s.cells[r] {
					produced := false
					for _, w := range set {
						produced = produced || w != nil
					}
					if produced {
						s.eng.noteHeldNothing(r.V.Name, s.fn)
						break
					}
				}
				continue
			}
			kind, glob = OutGlobal, r.V
		}
		cm := s.cells[r]
		for _, p := range s.pathsOf(r, cm) {
			var elems []OutElem
			for _, e := range s.elemsOf(r, p, cm[p]) {
				if kind == OutParam && p == "" && e == (ParamElem(idx, "")) {
					continue // the parameter's own taint: no flow
				}
				if kind == OutParam && cm[p][e].isParamSeed() {
					// A `kind: param` source says something about the value the
					// function received: the seed is not written back (ADR-048).
					// What the function writes itself (a store, a send, a callee's
					// summary) carries a reason of its own and is kept.
					continue
				}
				if cm[p][e] == nil {
					continue // read from the global store, not produced here
				}
				gs, over := s.guardsOfChain(Loc{r, p}, e)
				if over {
					s.guardOverflow = true
				}
				elems = append(elems, OutElem{E: e, Steps: s.trace(Loc{r, p}, e), Guards: gs})
			}
			if len(elems) > 0 {
				sum.Outputs = append(sum.Outputs, Output{Kind: kind, Index: idx, Global: glob, Path: p, Elems: widen(elems)})
			}
		}
	}

	keys := make([]sinkKey, 0, len(s.sinkReaches))
	for k := range s.sinkReaches {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessSinkKey(keys[i], keys[j]) })
	for _, k := range keys {
		sum.Sinks = append(sum.Sinks, s.sinkReaches[k])
	}
	sum.Opaque = s.opaqueParams()
	sum.Sig = sum.signature()
	return sum
}

// maxOutElems bounds the elements of one output. Beyond it, the symbolic
// elements that stand for different parts of the same parameter are merged
// into one that stands for the whole parameter. That says "may come from
// anywhere in this parameter", which is more than the truth and never less,
// and it keeps the summaries of pointer-heavy code from growing with the
// number of fields (ADR-014, ADR-031).
const maxOutElems = 24

func widen(elems []OutElem) []OutElem {
	if len(elems) <= maxOutElems {
		return elems
	}
	type key struct {
		param int
		mods  string
	}
	merged := map[key]OutElem{}
	var order []key
	var out []OutElem
	for _, oe := range elems {
		if oe.E.Kind != Param {
			out = append(out, oe)
			continue
		}
		k := key{int(oe.E.Param), oe.E.Mods()}
		cur, ok := merged[k]
		if !ok {
			order = append(order, k)
			w := oe
			w.E.path = 0
			merged[k] = w
			continue
		}
		if oe.Steps.len() < cur.Steps.len() {
			cur.Steps = oe.Steps
		}
		if !sameGuards(cur.Guards, oe.Guards) {
			cur.Guards = nil // merged: no single condition describes them
		}
		merged[k] = cur
	}
	for _, k := range order {
		out = append(out, merged[k])
	}
	return out
}

func (s *state) isParamRoot(r Root) bool {
	_, ok := s.paramRoot[r]
	return ok
}

// signature identifies the meaning of a summary, ignoring the steps.
func (sum *Summary) signature() string {
	var parts []string
	for _, o := range sum.Outputs {
		g := ""
		if o.Global != nil {
			g = o.Global.Name
		}
		parts = append(parts, "out", o.Kind, strconv.Itoa(o.Index), g, o.Path)
		for _, e := range o.Elems {
			parts = append(parts, e.E.String(), guardsKey(e.Guards))
		}
	}
	for _, k := range sum.Sinks {
		parts = append(parts, "sink", k.Site, k.Elem.String(), guardsKey(k.Guards), k.Field, strconv.FormatBool(k.Unsure))
	}
	for _, i := range sum.Opaque {
		parts = append(parts, "opaque", strconv.Itoa(i))
	}
	return fingerprint.Hash(parts...)
}

// expanded is an element of the callee translated into the caller.
type expanded struct {
	E    Elem
	From *found // where the actual taint was found in the caller; nil if it originated in the callee
	// Enclosing: it was found in a cell that holds the place that was read whole (a cell above it), and
	// Rel is otherwise the path of the cell below the place that was read: what a sink that reads a field
	// matches (fieldsink.go).
	Enclosing bool
	Rel       string
}

// expandElem translates an element of a callee summary using the actual
// arguments of the call.
func (s *state) expandElem(e Elem, argv []*sg.Value) []expanded {
	if e.Kind == Real {
		return []expanded{{E: e}}
	}
	if s.expandMemo != nil {
		if r, ok := s.expandMemo[e]; ok {
			return r
		}
	}
	r := s.expandParam(e, argv, false)
	if s.expandMemo != nil {
		s.expandMemo[e] = r
	}
	return r
}

// expandParam translates a symbolic element. keepAll: the same element found in several cells is not made
// one (the cells are told apart by the field that a sink reads: the same element may be in the field in
// one and beside it in another).
func (s *state) expandParam(e Elem, argv []*sg.Value, keepAll bool) []expanded {
	if e.Param < 0 || int(e.Param) >= len(argv) {
		return nil
	}
	locs := s.locs(argv[e.Param])
	ep := e.Path()
	if e.Param == 0 && s.recvBoxed {
		// both places: the interface holds the pointer's target below "/*" in Go; a language that
		// does not do so keeps it where the callee says
		boxed := make([]Loc, len(locs))
		for i, l := range locs {
			boxed[i] = Loc{l.Root, l.Path + "/*"}
		}
		locs = append(append([]Loc(nil), locs...), boxed...)
	}
	founds := make([][]found, len(locs))
	read := make([]int, len(locs)) // the length of the path that was read
	total := 0
	for i, l := range locs {
		founds[i] = s.collect(Loc{l.Root, l.Path + ep})
		read[i] = len(l.Path + ep)
		total += len(founds[i])
	}
	if total == 0 {
		return nil
	}
	// What is made lives for the length of one call of a summary: it is made at
	// its size, not grown.
	out := make([]expanded, 0, total)
	// The same element found in several cells is one element: the first
	// reason stays (a cell keeps the first reason an element got into it).
	seen := make(map[Elem]bool, total)
	for fi, found := range founds {
		for i := range found {
			f := &found[i]
			enclosing := len(f.Cell.Path) < read[fi]
			if f.E.Kind == Real {
				for _, x := range applyMods(f.E, e.Mods()) {
					if !seen[x] || keepAll {
						seen[x] = true
						out = append(out, expanded{E: x, From: f, Enclosing: enclosing, Rel: f.Rel})
					}
				}
				continue
			}
			x := f.E
			x = x.WithMods(joinMods(f.E.Mods(), e.Mods()))
			if !seen[x] || keepAll {
				seen[x] = true
				out = append(out, expanded{E: x, From: f, Enclosing: enclosing, Rel: f.Rel})
			}
		}
	}
	return out
}

// applySummary applies the summary of a callee at a call site.
func (s *state) applySummary(cc *callCtx, fn *sg.Func, bindings []*sg.Value, notes []*Uncertainty, extra []Guard) {
	sum := s.eng.summaryOf(fn)
	if sum == nil {
		return // not analysed yet; a later round picks it up
	}
	argv := append(cc.argv(), bindings...)
	// A callee may take optional parameters that the call leaves out (an
	// override can add them): what is not passed carries nothing.
	if want := len(fn.Params) + len(fn.FreeVars); len(argv) < want && len(bindings) == 0 {
		if len(fn.FreeVars) > 0 && usesCaptures(sum, len(fn.Params)) {
			// The closure is called where the values it captured are not at hand (a helper
			// that was given it): they are filled in as nothing, and the flows that go
			// through them are not followed. That is said (ADR-053), not left silent.
			s.noteUnboundCapture(cc, fn)
		}
		for len(argv) < want {
			argv = append(argv, &sg.Value{Kind: sg.KUndef})
		}
	}
	if len(argv) != len(fn.Params)+len(fn.FreeVars) {
		s.unmodeled(cc, s.newNote(cc.in, KindUnresolvedCallee, "the arguments do not match the parameters of "+s.eng.funcID(fn), cc.sym))
		return
	}
	desc := "call " + s.eng.funcID(fn)
	info := s.newCallInfo(cc, argv)
	// Within one application the same symbolic element is expanded many
	// times; the actual arguments do not change meanwhile.
	s.collectMemo = map[Loc][]found{}
	s.expandMemo = map[Elem][]expanded{}
	// A call through an interface whose method has a pointer receiver: the interface holds
	// the pointer's target below "/*" (make_interface), the summary speaks of the target.
	s.recvBoxed = cc.in.Callee != nil && cc.in.Callee.Kind == ir.CalleeDynamic && cc.recv != nil && !pointerLike(cc.recv) &&
		len(fn.Params) > 0 && pointerLike(fn.Params[0])
	defer func() { s.collectMemo, s.expandMemo, s.recvBoxed = nil, nil, false }()
	for _, i := range sum.Opaque {
		if i < len(argv) {
			s.markOpaque(argv[i], cc.in)
		}
	}
	for _, o := range sum.Outputs {
		var dsts []Loc
		switch o.Kind {
		case OutResult:
			if cc.noResult {
				continue // a sanitizer says the result is clean: the body is read for the rest
			}
			if cc.rd != nil {
				// the result of a callback goes where the row says (the first one only)
				if o.Index == 0 {
					for _, l := range cc.rd.ret {
						dsts = append(dsts, Loc{l.Root, l.Path + o.Path})
					}
				}
			} else if o.Index < len(cc.roots) {
				dsts = []Loc{{cc.roots[o.Index], o.Path}}
			}
		case OutThrow:
			if cc.rd != nil && cc.rd.throwSet {
				for _, l := range cc.rd.throw {
					dsts = append(dsts, Loc{l.Root, l.Path + o.Path})
				}
			} else if !cc.made {
				dsts = []Loc{{excRoot(), o.Path}}
			}
		case OutParam:
			if o.Index < len(argv) {
				// an interface value made of a pointer: the write is to what the pointer points to as well (ADR-090)
				dsts = append(dsts, s.ifaceTargets(argv[o.Index], o.Path)...)
				for _, l := range s.locs(argv[o.Index]) {
					dsts = append(dsts, Loc{l.Root, l.Path + o.Path})
					// a pointer held in a field of what was passed (the environment of a closure that captured a
					// variable by `&mut`): the write is to the variable it points to as well (ENV-PTR)
					dsts = append(dsts, s.mirror(Loc{l.Root, l.Path + o.Path})...)
					if o.Index == 0 && s.recvBoxed {
						dsts = append(dsts, Loc{l.Root, l.Path + "/*" + o.Path})
					}
				}
			}
			if cc.rd != nil {
				for _, l := range cc.rd.writes[o.Index] {
					dsts = append(dsts, Loc{l.Root, l.Path + o.Path})
				}
			}
		case OutGlobal:
			dsts = []Loc{{own(o.Global), o.Path}}
		}
		for _, oe := range o.Elems {
			if len(oe.Guards) > 0 && !info.feasible(oe.Guards) {
				continue // the call passes constants that rule this flow out
			}
			if cc.made && o.Kind == OutGlobal && o.Path == "" && oe.E.Kind == Param && oe.E.Mods() == "" &&
				int(oe.E.Param) < len(argv) && identityOfGlobal(s.fn, argv[oe.E.Param], oe.E.Path(), o.Global) {
				// The closure writes back the whole value of the variable that was read, whole, where it
				// was made (`before := G; ...; G = before`, the cleanup of a mock). The analysis does not
				// follow the order, so G holds what it was read to hold: writing it to G adds nothing,
				// except that the elements below a field are lifted to the whole value, which is an
				// error of the copy of a struct (ADR-059). It is counted (stats.identity_global_stores).
				s.eng.noteIdentityStore(cc.in)
				continue
			}
			for _, x := range s.expandElem(oe.E, argv) {
				// Most of what a summary says is already known here: then adding
				// it changes nothing, and its reason (the largest thing made) is
				// not needed.
				var nf ref
				if x.From != nil {
					nf = ref{x.From.Cell, x.From.Orig}
				}
				if len(oe.Guards) == 0 && s.addsNothing(dsts, x.E, nf, x.From != nil) {
					continue
				}
				var w *why
				if x.From != nil {
					w = newWhyFrom(s.fn, desc, x.From.Cell, x.From.Orig)
				} else {
					w = &why{fn: s.fn, desc: desc}
				}
				w.pos, w.steps = packPos(cc.in.Pos), oe.Steps
				if len(notes) > 0 {
					w.setNotes(notes)
				}
				if len(extra) > 0 {
					w.setGuards(extra)
				}
				if len(oe.Guards) > 0 {
					w.setCallee(oe.Guards, info)
				}
				for _, d := range dsts {
					s.add(d, x.E, w)
				}
			}
		}
	}
	for _, sr := range sum.Sinks {
		if len(sr.Guards) > 0 && !info.feasible(sr.Guards) {
			continue
		}
		srGuards := s.instantiate(info, sr.Guards)
		callGuards := unionGuards(s.blockGuards(cc.in.Block), extra)
		var xs []expanded
		if sr.Field == "" {
			xs = s.expandElem(sr.Elem, argv)
		} else {
			xs = s.expandParam(sr.Elem, argv, true)
		}
		for _, x := range xs {
			// the part of the value that the sink reads: what is found in the caller is matched against it
			field, unsure := "", sr.Unsure
			var fnote *Uncertainty
			if sr.Field != "" {
				names := strings.Split(sr.Field, ".")
				kind, rest := partAbove, names
				if !x.Enclosing {
					kind, rest = matchParts(names, x.Rel)
				}
				switch {
				case kind == partNone:
					continue
				case kind == partAbove && x.E.Kind == Param:
					field = strings.Join(rest, ".")
				case kind == partAbove:
					unsure = true
					cell := ""
					if x.From != nil {
						cell = x.From.Cell.Path
					}
					fnote = s.fieldNote(cc.in.Pos, sr.Field, s.whyUnsure(cell), cell)
				}
			}
			var steps *rope
			if x.From != nil {
				steps = s.trace(x.From.Cell, x.From.Orig)
			}
			// The length the path will have, which is all that the flows that
			// are dropped need; the path itself is made for the one that is kept.
			stepsLen := steps.len() + 1 + sr.Steps.len()
			if stepsLen > maxRopeLen {
				stepsLen = maxRopeLen
			}
			// The guards are worked out only for a flow that is kept: one
			// that is not shorter than what is already known is dropped, and
			// only its label counts.
			switch x.E.Kind {
			case Real:
				if !accepted(sr.Accepts, x.E.Label()) {
					continue
				}
				// A flow that is not shorter is dropped, unless the guards of
				// the one kept could differ from it: then they are compared.
				if rf := s.findings[findingKey{x.E.Origin(), sr.Site}]; rf != nil && rf.steps != nil && stepsLen >= rf.steps.len() && len(rf.guards) == 0 && !(rf.unsure && !unsure) {
					rf.labels[x.E.Label()] = true
					continue
				}
			case Param:
				if old, ok := s.sinkReaches[sinkKey{x.E, sr.Site, field, unsure}]; ok && stepsLen >= old.Steps.len() && len(old.Guards) == 0 {
					continue
				}
			}
			call := Step{Func: s.eng.funcID(s.fn), Desc: desc, pos: cc.in.Pos}
			call.Notes = appendNote(notes, fnote)
			steps = cat(cat(steps, leaf(call)), sr.Steps)
			var chain []Guard
			var over bool
			if x.From != nil {
				chain, over = s.guardsOfChain(x.From.Cell, x.From.Orig)
			}
			var gs []Guard
			// Far more guards than are kept: do not merge them just to drop
			// them (the flow is kept without guards: under any condition).
			if !over && len(srGuards)+len(chain)+len(callGuards) <= 4*maxGuards {
				gs = unionGuards(srGuards, chain, callGuards)
			} else {
				over = true
			}
			if over || len(gs) > maxGuards {
				s.guardOverflow = true
				gs = nil
			}
			switch x.E.Kind {
			case Real:
				if accepted(sr.Accepts, x.E.Label()) {
					s.recordFinding(x.E.Origin(), s.eng.sites.ByID[sr.Site], x.E.Label(), steps, gs, unsure)
				}
			case Param:
				s.noteSinkReach(sinkKey{x.E, sr.Site, field, unsure}, &SinkReach{Elem: x.E, Site: sr.Site, Accepts: sr.Accepts, Steps: steps, Guards: gs, Field: field, Unsure: unsure})
			}
		}
	}
}

// joinSummaries merges a new summary into the previous one of the same
// function. The analysis is monotone in the summaries of the callees, so the
// join normally changes nothing; its job is to make termination certain when
// widening or mutual recursion would otherwise make a summary flip back and
// forth: the result can only grow, and the set of possible elements is finite.
// (A widened output joined with one that is not has the whole and the parts: the parts are
// dropped before widen (absorbWhole), or the elements would go up and down for ever.)
func joinSummaries(old, cur *Summary) *Summary {
	if old == nil {
		return cur
	}
	type okey struct {
		kind  string
		index int
		glob  *sg.Value
		path  string
	}
	elems := map[okey]map[Elem]OutElem{}
	var keys []okey
	addAll := func(sum *Summary) {
		for _, o := range sum.Outputs {
			k := okey{o.Kind, o.Index, o.Global, o.Path}
			m := elems[k]
			if m == nil {
				m = map[Elem]OutElem{}
				elems[k] = m
				keys = append(keys, k)
			}
			for _, oe := range o.Elems {
				have, ok := m[oe.E]
				switch {
				case !ok:
					m[oe.E] = oe
				case !sameGuards(have.Guards, oe.Guards):
					// Seen under different conditions: under none in particular.
					if oe.Steps.len() < have.Steps.len() {
						have.Steps = oe.Steps
					}
					have.Guards = nil
					m[oe.E] = have
				case oe.Steps.len() < have.Steps.len():
					m[oe.E] = oe
				}
			}
		}
	}
	addAll(old)
	addAll(cur)
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.index != b.index {
			return a.index < b.index
		}
		ga, gb := "", ""
		if a.glob != nil {
			ga = a.glob.Name
		}
		if b.glob != nil {
			gb = b.glob.Name
		}
		if ga != gb {
			return ga < gb
		}
		return a.path < b.path
	})
	out := &Summary{Func: cur.Func}
	for _, k := range keys {
		list := make([]OutElem, 0, len(elems[k]))
		for _, oe := range elems[k] {
			list = append(list, oe)
		}
		sort.Slice(list, func(i, j int) bool { return lessElem(list[i].E, list[j].E) })
		out.Outputs = append(out.Outputs, Output{Kind: k.kind, Index: k.index, Global: k.glob, Path: k.path, Elems: widen(absorbWhole(list))})
	}
	sinks := map[sinkKey]*SinkReach{}
	for _, sum := range []*Summary{old, cur} {
		for _, sr := range sum.Sinks {
			k := sinkKey{sr.Elem, sr.Site, sr.Field, sr.Unsure}
			if have, ok := sinks[k]; !ok {
				sinks[k] = sr
			} else {
				sinks[k] = mergeSinkReach(have, sr)
			}
		}
	}
	skeys := make([]sinkKey, 0, len(sinks))
	for k := range sinks {
		skeys = append(skeys, k)
	}
	sort.Slice(skeys, func(i, j int) bool { return lessSinkKey(skeys[i], skeys[j]) })
	for _, k := range skeys {
		out.Sinks = append(out.Sinks, sinks[k])
	}
	seen := map[int]bool{}
	for _, sum := range []*Summary{old, cur} {
		for _, i := range sum.Opaque {
			if !seen[i] {
				seen[i] = true
				out.Opaque = append(out.Opaque, i)
			}
		}
	}
	sort.Ints(out.Opaque)
	out.Sig = out.signature()
	return out
}

// closureSite is the function that makes a closure, and the place where it does.
type closureSite struct {
	fn  *sg.Func
	pos ir.Pos
}

// closureMakers is where each closure is made, by the ID of the closure.
func (e *Engine) closureMakers() map[string]closureSite {
	e.makersOnce.Do(func() {
		e.makers = map[string]closureSite{}
		for _, f := range e.prog.Funcs {
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op == ir.OpMakeClosure && in.IR != nil && in.IR.Func != "" {
						if _, ok := e.makers[string(in.IR.Func)]; !ok {
							e.makers[string(in.IR.Func)] = closureSite{f, in.Pos}
						}
					}
				}
			}
		}
	})
	return e.makers
}

// noteUnboundCapture records that a closure is called without the values it captured: the
// function that made it knows them, the call in the helper does not. The uncertainty is at the
// place where the closure is made (the function that holds the values, which is where a reader of
// a lost flow looks), of kind depth_limit (rivus' limit: the summary of the helper does not carry
// the captured values of the closure it is given), and it changes nothing of what is found.
func (s *state) noteUnboundCapture(cc *callCtx, fn *sg.Func) {
	id := s.eng.funcID(fn)
	u := newUncertainty(KindDepthLimit, "capture_unbound: the closure made here captured "+itoa(len(fn.FreeVars))+
		" value(s), and a function it was handed to calls it without them (they are known only here): the flows that go through them are not followed", id)
	u.Func = id
	u.pos = cc.in.Pos
	if mk, ok := s.eng.closureMakers()[fn.ID]; ok {
		u.Func, u.pos = s.eng.funcID(mk.fn), mk.pos
	}
	s.uncertain = append(s.uncertain, u)
}

// usesCaptures reports whether what the summary says depends on the values the function captured
// (the parameters after the declared ones), or writes to one of them: only then does not knowing
// them lose anything.
func usesCaptures(sum *Summary, declared int) bool {
	for _, o := range sum.Outputs {
		if o.Kind == OutParam && o.Index >= declared {
			return true
		}
		for _, oe := range o.Elems {
			if oe.E.Kind == Param && int(oe.E.Param) >= declared {
				return true
			}
		}
	}
	for _, sk := range sum.Sinks {
		if sk.Elem.Kind == Param && int(sk.Elem.Param) >= declared {
			return true
		}
	}
	return false
}

// absorbWhole drops the elements that stand for a part of a parameter when the element that
// stands for the whole of it (what widen makes) is there too, with the same operations (mods)
// and the same conditions, or none: the part adds nothing, because the whole is read at the same
// place and below it. Without it, a summary that was widened and is joined with one that was
// not has both, is widened again, and the elements go up and down for ever.
func absorbWhole(list []OutElem) []OutElem {
	type key struct {
		param int32
		mods  string
	}
	whole := map[key]OutElem{}
	for _, oe := range list {
		if oe.E.Kind == Param && oe.E.Path() == "" {
			whole[key{oe.E.Param, oe.E.Mods()}] = oe
		}
	}
	if len(whole) == 0 {
		return list
	}
	out := list[:0:0]
	for _, oe := range list {
		if oe.E.Kind == Param && oe.E.Path() != "" {
			if w, ok := whole[key{oe.E.Param, oe.E.Mods()}]; ok && (len(w.Guards) == 0 || sameGuards(w.Guards, oe.Guards)) {
				continue
			}
		}
		out = append(out, oe)
	}
	return out
}

// identityOfGlobal reports whether what the element of a summary names (the value v of the home
// function, below the path epath of it) is the whole value of the global g, read as it is and nothing
// else: the same path (the whole), by a load of g, through copies and φ, or held in a cell (a variable
// captured by reference) that is stored once, by such a load, and is not written in any other way.
// When it cannot be said so for sure: false (the write is applied). One predicate, for the closure
// made here and for a plain `G = before` (ADR-059).
func identityOfGlobal(home *sg.Func, v *sg.Value, epath string, g *sg.Value) bool {
	if v == nil || g == nil || home == nil {
		return false
	}
	if epath != "" {
		return false
	}
	if isLoadOfGlobal(v, g) {
		return true
	}
	// a cell: the alloc that the closure captured (its content is the element)
	if v.Instr == nil || v.Instr.Op != ir.OpAlloc {
		return false
	}
	var stored *sg.Value
	for _, b := range home.Blocks {
		for _, in := range b.Instrs {
			for i, a := range in.Args {
				if a != v {
					continue
				}
				switch {
				case in.Op == ir.OpMakeClosure:
				case in.Op == ir.OpLoad && i == 0:
				case in.Op == ir.OpStore && i == 0 && len(in.Args) == 2:
					if stored != nil || in.Args[1] == v {
						return false
					}
					stored = in.Args[1]
				default:
					return false // written or given away in another way
				}
			}
		}
	}
	return stored != nil && isLoadOfGlobal(stored, g)
}

// isLoadOfGlobal reports whether v is, through copies and φ only, the whole value of the global g
// read as it is (a load of g, and nothing else mixed in). When it cannot be said so for sure: false.
func isLoadOfGlobal(v *sg.Value, g *sg.Value) bool {
	if v == nil || g == nil {
		return false
	}
	seen := map[*sg.Value]bool{}
	var walk func(x *sg.Value, depth int) bool
	walk = func(x *sg.Value, depth int) bool {
		if x == nil || depth > 8 || seen[x] {
			return seen[x] // a cycle through φ adds nothing
		}
		seen[x] = true
		if x.Instr == nil {
			return false
		}
		in := x.Instr
		switch {
		case in.Op == ir.OpGlobalLoad && in.IR != nil:
			return in.IR.Global == g.Name
		case in.Op == ir.OpLoad && len(in.Args) == 1:
			return in.Args[0] == g
		case in.Op == ir.OpCopy && in.IR != nil && !in.IR.View && len(in.Args) == 1:
			return walk(in.Args[0], depth+1)
		case in.Op == sg.OpPhi && len(in.Args) > 0:
			for _, a := range in.Args {
				if !walk(a, depth+1) {
					return false
				}
			}
			return true
		}
		return false
	}
	return walk(v, 0)
}
