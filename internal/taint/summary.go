package taint

import (
	"sort"
	"strconv"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/fingerprint"
)

// Output kinds of a summary.
const (
	OutResult = "result"
	OutParam  = "param"
	OutGlobal = "global"
)

// OutElem is an element of an output with the steps that led to it inside
// the function.
type OutElem struct {
	E     Elem
	Steps []Step
}

// Output says which taint ends up in a result, in (the object behind) a
// parameter, or in a global variable.
type Output struct {
	Kind   string
	Index  int         // result or parameter index
	Global *ssa.Global // OutGlobal
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
		return a.Idx < b.Idx
	})
	for _, r := range roots {
		var kind string
		var idx int
		var glob *ssa.Global
		switch {
		case r.V == nil:
			kind, idx = OutResult, r.Idx
		case s.paramRoot[r] != 0 || s.isParamRoot(r):
			kind, idx = OutParam, s.paramRoot[r]
		default:
			g, ok := r.V.(*ssa.Global)
			if !ok {
				continue
			}
			kind, glob = OutGlobal, g
		}
		cm := s.cells[r]
		paths := make([]string, 0, len(cm))
		for p := range cm {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			var elems []OutElem
			for _, e := range cm[p].sorted() {
				if kind == OutParam && p == "" && e == (Elem{Kind: Param, Param: idx}) {
					continue // the parameter's own taint: no flow
				}
				if _, ok := s.whys[ref{Loc{r, p}, e}]; !ok {
					continue // read from the global store, not produced here
				}
				elems = append(elems, OutElem{E: e, Steps: s.trace(Loc{r, p}, e)})
			}
			if len(elems) > 0 {
				sum.Outputs = append(sum.Outputs, Output{Kind: kind, Index: idx, Global: glob, Path: p, Elems: elems})
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
			g = o.Global.String()
		}
		parts = append(parts, "out", o.Kind, strconv.Itoa(o.Index), g, o.Path)
		for _, e := range o.Elems {
			parts = append(parts, e.E.String())
		}
	}
	for _, k := range sum.Sinks {
		parts = append(parts, "sink", k.Site, k.Elem.String())
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
func (s *state) expandElem(e Elem, argv []ssa.Value) []expanded {
	if e.Kind == Real {
		return []expanded{{E: e}}
	}
	if e.Param < 0 || e.Param >= len(argv) {
		return nil
	}
	var out []expanded
	for _, l := range s.locs(argv[e.Param]) {
		for _, f := range s.collect(Loc{l.Root, l.Path + e.Path}) {
			f := f
			if f.E.Kind == Real {
				for _, x := range applyMods(f.E, e.Mods) {
					out = append(out, expanded{E: x, From: &f})
				}
				continue
			}
			x := f.E
			x.Mods = joinMods(f.E.Mods, e.Mods)
			out = append(out, expanded{E: x, From: &f})
		}
	}
	return out
}

// applySummary applies the summary of a callee at a call site.
func (s *state) applySummary(cc *callCtx, fn *ssa.Function, bindings []ssa.Value, note *Uncertainty) {
	sum := s.eng.summaries[fn]
	if sum == nil {
		return // not analysed yet; a later round picks it up
	}
	argv := append(cc.argv(), bindings...)
	if len(argv) != len(fn.Params)+len(fn.FreeVars) {
		s.unmodeled(cc, s.newNote(cc.in, KindUnresolvedCallee, "the arguments do not match the parameters of "+s.eng.funcID(fn), cc.sym))
		return
	}
	desc := "call " + s.eng.funcID(fn)
	for _, o := range sum.Outputs {
		var dsts []Loc
		switch o.Kind {
		case OutResult:
			if o.Index < len(cc.roots) {
				dsts = []Loc{{cc.roots[o.Index], o.Path}}
			}
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
			for _, x := range s.expandElem(oe.E, argv) {
				w := &why{fn: s.fn, pos: cc.in.Pos(), desc: desc, steps: oe.Steps, note: note}
				if x.From != nil {
					w.from = &ref{x.From.Cell, x.From.Orig}
				}
				for _, d := range dsts {
					s.add(d, x.E, w)
				}
			}
		}
	}
	for _, sr := range sum.Sinks {
		for _, x := range s.expandElem(sr.Elem, argv) {
			steps := []Step{}
			if x.From != nil {
				steps = append(steps, s.trace(x.From.Cell, x.From.Orig)...)
			}
			call := Step{Func: s.eng.funcID(s.fn), Desc: desc, pos: cc.in.Pos()}
			if note != nil {
				call.Notes = []*Uncertainty{note}
			}
			steps = append(steps, call)
			steps = append(steps, sr.Steps...)
			switch x.E.Kind {
			case Real:
				if accepted(sr.Accepts, x.E.Label) {
					s.recordFinding(x.E.Origin, s.eng.sites.ByID[sr.Site], x.E.Label, steps)
				}
			case Param:
				k := sinkKey{x.E, sr.Site}
				if old, ok := s.sinkReaches[k]; !ok || len(steps) < len(old.Steps) {
					s.sinkReaches[k] = &SinkReach{Elem: x.E, Site: sr.Site, Accepts: sr.Accepts, Steps: steps}
				}
			}
		}
	}
}
