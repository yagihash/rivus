package taint

import (
	"sort"
	"strconv"

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
	Sig     string
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
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].site != keys[j].site {
			return keys[i].site < keys[j].site
		}
		return lessElem(keys[i].e, keys[j].e)
	})
	for _, k := range keys {
		sum.Sinks = append(sum.Sinks, s.sinkReaches[k])
	}
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
		parts = append(parts, "sink", k.Site, k.Elem.String(), guardsKey(k.Guards))
	}
	return fingerprint.Hash(parts...)
}

// expanded is an element of the callee translated into the caller.
type expanded struct {
	E    Elem
	From *found // where the actual taint was found in the caller; nil if it originated in the callee
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
	r := s.expandParam(e, argv)
	if s.expandMemo != nil {
		s.expandMemo[e] = r
	}
	return r
}

func (s *state) expandParam(e Elem, argv []*sg.Value) []expanded {
	if e.Param < 0 || int(e.Param) >= len(argv) {
		return nil
	}
	locs := s.locs(argv[e.Param])
	founds := make([][]found, len(locs))
	total := 0
	for i, l := range locs {
		founds[i] = s.collect(Loc{l.Root, l.Path + e.Path()})
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
	for _, found := range founds {
		for i := range found {
			f := &found[i]
			if f.E.Kind == Real {
				for _, x := range applyMods(f.E, e.Mods()) {
					if !seen[x] {
						seen[x] = true
						out = append(out, expanded{E: x, From: f})
					}
				}
				continue
			}
			x := f.E
			x = x.WithMods(joinMods(f.E.Mods(), e.Mods()))
			if !seen[x] {
				seen[x] = true
				out = append(out, expanded{E: x, From: f})
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
	defer func() { s.collectMemo, s.expandMemo = nil, nil }()
	for _, o := range sum.Outputs {
		var dsts []Loc
		switch o.Kind {
		case OutResult:
			if o.Index < len(cc.roots) {
				dsts = []Loc{{cc.roots[o.Index], o.Path}}
			}
		case OutThrow:
			dsts = []Loc{{excRoot(), o.Path}}
		case OutParam:
			if o.Index < len(argv) {
				for _, l := range s.locs(argv[o.Index]) {
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
		for _, x := range s.expandElem(sr.Elem, argv) {
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
				if rf := s.findings[findingKey{x.E.Origin(), sr.Site}]; rf != nil && rf.steps != nil && stepsLen >= rf.steps.len() && len(rf.guards) == 0 {
					rf.labels[x.E.Label()] = true
					continue
				}
			case Param:
				if old, ok := s.sinkReaches[sinkKey{x.E, sr.Site}]; ok && stepsLen >= old.Steps.len() && len(old.Guards) == 0 {
					continue
				}
			}
			call := Step{Func: s.eng.funcID(s.fn), Desc: desc, pos: cc.in.Pos}
			call.Notes = notes
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
					s.recordFinding(x.E.Origin(), s.eng.sites.ByID[sr.Site], x.E.Label(), steps, gs)
				}
			case Param:
				s.noteSinkReach(sinkKey{x.E, sr.Site}, &SinkReach{Elem: x.E, Site: sr.Site, Accepts: sr.Accepts, Steps: steps, Guards: gs})
			}
		}
	}
}

// joinSummaries merges a new summary into the previous one of the same
// function. The analysis is monotone in the summaries of the callees, so the
// join normally changes nothing; its job is to make termination certain when
// widening or mutual recursion would otherwise make a summary flip back and
// forth: the result can only grow, and the set of possible elements is finite.
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
		out.Outputs = append(out.Outputs, Output{Kind: k.kind, Index: k.index, Global: k.glob, Path: k.path, Elems: widen(list)})
	}
	sinks := map[sinkKey]*SinkReach{}
	for _, sum := range []*Summary{old, cur} {
		for _, sr := range sum.Sinks {
			k := sinkKey{sr.Elem, sr.Site}
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
	sort.Slice(skeys, func(i, j int) bool {
		if skeys[i].site != skeys[j].site {
			return skeys[i].site < skeys[j].site
		}
		return lessElem(skeys[i].e, skeys[j].e)
	})
	for _, k := range skeys {
		out.Sinks = append(out.Sinks, sinks[k])
	}
	out.Sig = out.signature()
	return out
}
