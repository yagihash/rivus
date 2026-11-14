package taint

import (
	"go/token"
	"sort"
	"strings"

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
	steps *rope // steps taken inside a callee, spliced in from its summary
	notes []*Uncertainty
	model *ModelRef
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

	rootVer     map[Root]int // number of elements added to each root: change detector
	callSigs    map[ssa.CallInstruction]int
	sortedElems map[Loc]cachedElems
	sortedPaths map[Root]cachedPaths
	collectMemo map[Loc][]found // active during one summary application
	expandMemo  map[Elem][]expanded

	suppressions map[string]*Suppression
	findings     map[findingKey]*rawFinding
	sinkReaches  map[sinkKey]*SinkReach
	uncertain    []*Uncertainty
	truncated    bool
	adds         int
	exceeded     bool
}

// cachedElems and cachedPaths remember sorted views of cells. Sets only
// ever grow, so a cache is valid as long as its length matches.
type cachedElems struct {
	n     int
	elems []Elem
}

type cachedPaths struct {
	n     int
	paths []string
}

func (s *state) elemsOf(r Root, path string, set Set) []Elem {
	if s.sortedElems == nil {
		s.sortedElems = map[Loc]cachedElems{}
	}
	k := Loc{r, path}
	if c, ok := s.sortedElems[k]; ok && c.n == len(set) {
		return c.elems
	}
	e := set.sorted()
	s.sortedElems[k] = cachedElems{len(set), e}
	return e
}

func (s *state) pathsOf(r Root, cm map[string]Set) []string {
	if s.sortedPaths == nil {
		s.sortedPaths = map[Root]cachedPaths{}
	}
	if c, ok := s.sortedPaths[r]; ok && c.n == len(cm) {
		return c.paths
	}
	paths := make([]string, 0, len(cm))
	for p := range cm {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	s.sortedPaths[r] = cachedPaths{len(cm), paths}
	return paths
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
	s.adds++
	if s.rootVer == nil {
		s.rootVer = map[Root]int{}
	}
	s.rootVer[l.Root]++
	if _, ok := s.whys[ref{l, e}]; !ok {
		s.whys[ref{l, e}] = w
	}
}

// collect reads everything that is known about the location: the taint of
// enclosing objects (extended to the path being read), the cell itself, and
// everything stored below it.
func (s *state) collect(l Loc) []found {
	if s.collectMemo != nil {
		if c, ok := s.collectMemo[l]; ok {
			return c
		}
	}
	out := s.collectUncached(l)
	if s.collectMemo != nil {
		s.collectMemo[l] = out
	}
	return out
}

func (s *state) collectUncached(l Loc) []found {
	cm := s.cellsOf(l.Root)
	var out []found
	for _, p := range s.pathsOf(l.Root, cm) {
		switch {
		case pathHasPrefix(l.Path, p) && p != l.Path:
			// An enclosing object is tainted as a whole: so is this part.
			rest := l.Path[len(p):]
			for _, e := range s.elemsOf(l.Root, p, cm[p]) {
				ext := e
				if e.Kind == Param {
					// Without the bound a recursive structure would extend
					// the path in every pass, forever.
					ext.Path, _ = pathTrunc(e.Path+rest, s.eng.opts.K)
				}
				out = append(out, found{E: ext, Cell: Loc{l.Root, p}, Orig: e})
			}
		case pathHasPrefix(p, l.Path):
			rel := p[len(l.Path):]
			for _, e := range s.elemsOf(l.Root, p, cm[p]) {
				out = append(out, found{E: e, Cell: Loc{l.Root, p}, Orig: e, Rel: rel})
			}
		}
	}
	return out
}

// copyLoc copies the structure below src to dst.
func (s *state) copyLoc(src, dst Loc, w func(f found) *why) {
	for _, f := range s.collect(src) {
		dl := Loc{dst.Root, dst.Path + f.Rel}
		s.add(dl, f.E, s.withContainerNote(w(f), f.Cell.Path, dl.Path))
	}
}

// pathNote is an uncertainty that belongs to one hop of a path. It is shown
// with the findings whose path contains the hop, not in the program-level
// list: "container_element_imprecision" only matters where it influenced a
// result.
func (s *state) pathNote(kind, detail string, pos token.Pos) *Uncertainty {
	u := newUncertainty(kind, detail, "")
	u.Func = s.eng.funcID(s.fn)
	u.pos = pos
	return u
}

func hasKind(notes []*Uncertainty, kind string) bool {
	for _, n := range notes {
		if n.Kind == kind {
			return true
		}
	}
	return false
}

// appendNote adds a note without touching the slice other hops share.
func appendNote(notes []*Uncertainty, u *Uncertainty) []*Uncertainty {
	if u == nil {
		return notes
	}
	return append(append([]*Uncertainty(nil), notes...), u)
}

func hasElementSegment(p string) bool {
	return strings.Contains(p, "/[]") || strings.HasSuffix(p, "/[k]") || strings.Contains(p, "/[k]/")
}

// withContainerNote marks a hop that moves data in or out of the elements of
// a slice, array, map or channel: the elements are not told apart, and
// writes through aliases of the container are not followed.
func (s *state) withContainerNote(w *why, from, to string) *why {
	if w == nil || hasKind(w.notes, KindContainerImprecision) || !(hasElementSegment(from) || hasElementSegment(to)) {
		return w
	}
	w.notes = appendNote(w.notes, s.pathNote(KindContainerImprecision,
		"the elements of the container are not told apart, and writes through other references to it are not followed", w.pos))
	return w
}

// copyLocs copies from every source location to every destination.
func (s *state) copyLocs(src []Loc, dst []Loc, kind string, in ssa.Instruction) {
	s.copyLocsNoted(src, dst, kind, in, nil)
}

// copyLocsNoted is copyLocs with an uncertainty attached to the hop.
func (s *state) copyLocsNoted(src []Loc, dst []Loc, kind string, in ssa.Instruction, note *Uncertainty) {
	for _, d := range dst {
		for _, sl := range src {
			s.copyLoc(sl, d, s.whyNote(in, kind, note))
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
		s.add(dst, f.E, s.withContainerNote(w(f), f.Cell.Path, dst.Path))
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
		w.notes = appendNote(w.notes, u)
		return w
	}
}

// origin is the reason for an element that appears out of nothing (a source
// or a parameter).
func (s *state) origin(pos token.Pos, kind string) *why {
	return &why{fn: s.fn, pos: pos, desc: kind}
}

// trace reconstructs the steps that led to the element in the cell.
func (s *state) trace(l Loc, e Elem) *rope {
	var chunks []*rope
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
	var out *rope
	for i := len(chunks) - 1; i >= 0; i-- {
		out = cat(out, chunks[i])
	}
	return out
}

func (s *state) stepsOf(w *why) *rope {
	var out *rope
	if w.desc != "" && !skipStep(w.desc) {
		st := Step{Func: s.eng.funcID(w.fn), Desc: w.desc, pos: w.pos, Model: w.model}
		st.Notes = w.notes
		out = leaf(st)
	} else if len(w.notes) > 0 {
		out = leaf(Step{Func: s.eng.funcID(w.fn), Desc: w.desc, pos: w.pos, Notes: w.notes})
	}
	return cat(out, w.steps)
}

// skipStep hides bookkeeping hops that carry no information for a reader.
func skipStep(desc string) bool {
	switch desc {
	case "phi", "convert", "alias", "param":
		return true
	}
	return false
}
