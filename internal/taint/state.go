package taint

import (
	"go/token"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// Root is where a cell tree is anchored: an SSA value (Idx -1), one result of
// a multi-valued instruction (Idx >= 0), or one result of the analysed
// function (V == nil).
type Root struct {
	V   ssa.Value
	Idx int
}

// Loc is a cell: a root plus an access path.
type Loc struct {
	Root Root
	Path string
}

func own(v ssa.Value) Root { return Root{V: v, Idx: -1} }

// ref names an element in a cell.
type ref struct {
	Loc Loc
	E   Elem
}

// why records how an element first got into a cell, so that a path from a
// source to a sink can be reconstructed.
type why struct {
	fn    *ssa.Function
	pos   token.Pos
	desc  string
	from  *ref
	steps []Step // steps taken inside a callee, spliced in from its summary
	note  *Uncertainty
}

// found is an element read from some cell.
type found struct {
	E    Elem // possibly extended to the read path
	Cell Loc  // where it was found
	Orig Elem // as stored there
	Rel  string
}

// state is the analysis state of one function.
type state struct {
	eng *Engine
	fn  *ssa.Function

	cells   map[Root]map[string]Set
	whys    map[ref]*why
	changed bool

	paramRoot map[Root]int // parameter and free variable roots -> index
	numbering map[ssa.Value]int
	locSet    map[ssa.Value]map[Loc]bool
	locCache  map[ssa.Value][]Loc

	findings    map[findingKey]*rawFinding
	sinkReaches map[sinkKey]*SinkReach
	uncertain   []*Uncertainty
	truncated   bool
}

func (s *state) cellsOf(r Root) map[string]Set {
	m := s.cells[r]
	if m == nil {
		m = map[string]Set{}
		s.cells[r] = m
		if g, ok := r.V.(*ssa.Global); ok {
			for path, set := range s.eng.globals.cellsOf(g) {
				cp := Set{}
				for e := range set {
					cp[e] = struct{}{}
				}
				m[path] = cp
			}
		}
	}
	return m
}

// add puts an element into a cell. It records w as the reason the first time.
func (s *state) add(l Loc, e Elem, w *why) {
	p, trunc := pathTrunc(l.Path, s.eng.opts.K)
	if trunc {
		s.truncated = true
	}
	l.Path = p
	cm := s.cellsOf(l.Root)
	set := cm[l.Path]
	if set == nil {
		set = Set{}
		cm[l.Path] = set
	}
	if _, ok := set[e]; ok {
		return
	}
	set[e] = struct{}{}
	s.changed = true
	if _, ok := s.whys[ref{l, e}]; !ok {
		s.whys[ref{l, e}] = w
	}
}

// collect reads everything that is known about the location: the taint of
// enclosing objects (extended to the path being read), the cell itself, and
// everything stored below it.
func (s *state) collect(l Loc) []found {
	cm := s.cellsOf(l.Root)
	paths := make([]string, 0, len(cm))
	for p := range cm {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var out []found
	for _, p := range paths {
		switch {
		case pathHasPrefix(l.Path, p) && p != l.Path:
			// An enclosing object is tainted as a whole: so is this part.
			rest := l.Path[len(p):]
			for _, e := range cm[p].sorted() {
				ext := e
				if e.Kind == Param {
					ext.Path = e.Path + rest
				}
				out = append(out, found{E: ext, Cell: Loc{l.Root, p}, Orig: e})
			}
		case pathHasPrefix(p, l.Path):
			rel := p[len(l.Path):]
			for _, e := range cm[p].sorted() {
				out = append(out, found{E: e, Cell: Loc{l.Root, p}, Orig: e, Rel: rel})
			}
		}
	}
	return out
}

// copyLoc copies the structure below src to dst.
func (s *state) copyLoc(src, dst Loc, w func(f found) *why) {
	for _, f := range s.collect(src) {
		s.add(Loc{dst.Root, dst.Path + f.Rel}, f.E, w(f))
	}
}

// copyLocs copies from every source location to every destination.
func (s *state) copyLocs(src []Loc, dst []Loc, kind string, in ssa.Instruction) {
	for _, d := range dst {
		for _, sl := range src {
			s.copyLoc(sl, d, s.whyFn(in, kind))
		}
	}
}

// flatten collects everything below the locations, ignoring structure.
func (s *state) flatten(locs []Loc) []found {
	var out []found
	for _, l := range locs {
		out = append(out, s.collect(l)...)
	}
	return out
}

// put adds the found elements to the destination, collapsed.
func (s *state) put(dst Loc, fs []found, w func(f found) *why) {
	for _, f := range fs {
		s.add(dst, f.E, w(f))
	}
}

func (s *state) whyFn(in ssa.Instruction, kind string) func(f found) *why {
	return func(f found) *why {
		w := &why{fn: s.fn, desc: kind, from: &ref{f.Cell, f.Orig}}
		if in != nil {
			w.pos = in.Pos()
		}
		return w
	}
}

func (s *state) whyNote(in ssa.Instruction, kind string, u *Uncertainty) func(f found) *why {
	base := s.whyFn(in, kind)
	return func(f found) *why {
		w := base(f)
		w.note = u
		return w
	}
}

// origin is the reason for an element that appears out of nothing (a source
// or a parameter).
func (s *state) origin(pos token.Pos, kind string) *why {
	return &why{fn: s.fn, pos: pos, desc: kind}
}

// trace reconstructs the steps that led to the element in the cell.
func (s *state) trace(l Loc, e Elem) []Step {
	var chunks [][]Step
	seen := map[ref]bool{}
	cur := ref{l, e}
	for len(chunks) < maxPathSteps {
		if seen[cur] {
			break
		}
		seen[cur] = true
		w := s.whys[cur]
		if w == nil {
			break
		}
		chunks = append(chunks, s.stepsOf(w))
		if w.from == nil {
			break
		}
		cur = *w.from
	}
	var out []Step
	for i := len(chunks) - 1; i >= 0; i-- {
		out = append(out, chunks[i]...)
	}
	return out
}

func (s *state) stepsOf(w *why) []Step {
	var out []Step
	if w.desc != "" && !skipStep(w.desc) {
		st := Step{Func: s.eng.funcID(w.fn), Desc: w.desc, pos: w.pos}
		if w.note != nil {
			st.Notes = []*Uncertainty{w.note}
		}
		out = append(out, st)
	} else if w.note != nil {
		out = append(out, Step{Func: s.eng.funcID(w.fn), Desc: w.desc, pos: w.pos, Notes: []*Uncertainty{w.note}})
	}
	out = append(out, w.steps...)
	return out
}

// skipStep hides bookkeeping hops that carry no information for a reader.
func skipStep(desc string) bool {
	switch desc {
	case "phi", "convert", "alias", "param":
		return true
	}
	return false
}
