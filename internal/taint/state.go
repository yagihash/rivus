package taint

import (
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// Root is where a cell tree is anchored: an SSA value (Idx -1), one result of
// a multi-valued instruction (Idx >= 0), or one result of the analysed
// function (V == nil).
type Root struct {
	V   *sg.Value
	Idx int
}

// Loc is a cell: a root plus an access path.
type Loc struct {
	Root Root
	Path string
}

func own(v *sg.Value) Root { return Root{V: v, Idx: -1} }

// ref names an element in a cell.
type ref struct {
	Loc Loc
	E   Elem
}

// why records how an element first got into a cell, so that a path from a
// source to a sink can be reconstructed.
type why struct {
	fn    *sg.Func
	pos   cpos
	desc  string
	from  *ref
	steps *rope // steps taken inside a callee, spliced in from its summary
	// rare holds what few hops have (the notes, the model, the guards): a hop
	// is made millions of times, and a plain one is a good deal smaller
	// without them. It is nil when none of them is set, which reads the same as
	// all of them being empty.
	rare *whyRare
}

// whyRare is what only some hops have.
type whyRare struct {
	notes []*Uncertainty
	model *ModelRef
	// guards are the parameter dependent conditions under which this hop
	// happens. They are recorded where flows meet or leave: at a phi, a
	// return and a sink. unguarded is set when the same element also gets
	// here without them, so that the guards no longer describe it.
	guards    []Guard
	unguarded bool
	// calleeGuards are the guards of a summary applied here, in the callee's
	// terms; call tells how to read them from this side.
	calleeGuards []Guard
	call         *callInfo
}

func (w *why) notes() []*Uncertainty {
	if w.rare == nil {
		return nil
	}
	return w.rare.notes
}

func (w *why) model() *ModelRef {
	if w.rare == nil {
		return nil
	}
	return w.rare.model
}

func (w *why) guards() []Guard {
	if w.rare == nil {
		return nil
	}
	return w.rare.guards
}

func (w *why) unguarded() bool { return w.rare != nil && w.rare.unguarded }

func (w *why) calleeGuards() []Guard {
	if w.rare == nil {
		return nil
	}
	return w.rare.calleeGuards
}

func (w *why) call() *callInfo {
	if w.rare == nil {
		return nil
	}
	return w.rare.call
}

// plain reports whether the hop has no guards and no summary applied.
func (w *why) plain() bool {
	return w.rare == nil || (len(w.rare.guards) == 0 && len(w.rare.calleeGuards) == 0 && w.rare.call == nil)
}

func (w *why) mk() *whyRare {
	if w.rare == nil {
		w.rare = &whyRare{}
	}
	return w.rare
}

func (w *why) setNotes(n []*Uncertainty) {
	if n == nil && w.rare == nil {
		return
	}
	w.mk().notes = n
}

func (w *why) setModel(m *ModelRef) {
	if m != nil {
		w.mk().model = m
	}
}

func (w *why) setGuards(g []Guard) {
	if g == nil && w.rare == nil {
		return
	}
	w.mk().guards = g
}

func (w *why) setCallee(g []Guard, c *callInfo) {
	if g == nil && c == nil && w.rare == nil {
		return
	}
	r := w.mk()
	r.calleeGuards, r.call = g, c
}

func (w *why) markUnguarded() { w.mk().unguarded = true }

// cpos is a position (ir.Pos) in 20 bytes: the file is the number of its name
// in the table of strings.
type cpos struct {
	file                      strID
	offset, length, line, col int32
}

func packPos(p ir.Pos) cpos {
	return cpos{intern(p.File), int32(p.Offset), int32(p.Length), int32(p.Line), int32(p.Col)}
}

func (c cpos) pos() ir.Pos {
	return ir.Pos{File: c.file.str(), Offset: int(c.offset), Length: int(c.length), Line: int(c.line), Col: int(c.col)}
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
	fn  *sg.Func

	cells   map[Root]map[string]Set
	changed bool

	paramRoot map[Root]int // parameter and free variable roots -> index
	numbering map[*sg.Value]int
	locSet    map[*sg.Value]map[Loc]bool
	locCache  map[*sg.Value][]Loc
	locsFinal bool // locCache may be filled: the fixed point of the locations is reached

	branches        *[]*sg.Block
	edgeGuardCache  map[[2]*sg.Block][]Guard
	guardOverflow   bool
	condCache       map[*sg.Block]*Guard
	blockGuardCache map[*sg.Block][]Guard
	rootVer         map[Root]int // number of elements added to each root: change detector
	callSigs        map[*sg.Instr]int
	sortedElems     map[Loc]cachedElems
	sortedPaths     map[Root]cachedPaths
	collectMemo     map[Loc][]found // active during one summary application
	collectCache    map[Loc]cachedCollect
	traceMemo       map[ref]*rope
	guardMemo       map[ref]guardAnswer
	guardEpoch      int // changes when an element loses the guards recorded for it
	expandMemo      map[Elem][]expanded

	suppressions map[string]*Suppression
	findings     map[findingKey]*rawFinding
	sinkReaches  map[sinkKey]*SinkReach
	uncertain    []*Uncertainty
	truncated    bool
	widened      bool
	adds         int
	exceeded     bool
}

// maxCellElems is the number of elements a cell may hold before the symbolic
// ones are widened to the whole parameter.
const maxCellElems = 96

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
		if r.V != nil && r.V.Kind == sg.KGlobal {
			for path, set := range s.eng.globals.cellsOf(r.V) {
				cp := Set{}
				for e := range set {
					cp[e] = nil
				}
				m[path] = cp
			}
		}
	}
	return m
}

// add puts an element into a cell. It records w as the reason the first time.
func (s *state) add(l Loc, e Elem, w *why) {
	// A number or a bool cannot carry an injection (a path, a query, a
	// command, a URL): taint stops there, unless asked to follow it (a rule
	// whose sink is a size or a count needs that).
	if v := l.Root.V; v != nil && l.Root.Idx < 0 && l.Path == "" && !s.eng.opts.NumericTaint &&
		(v.Class == ir.ClassNumber || v.Class == ir.ClassBool) {
		return
	}
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
	// A cell that holds very many symbolic elements of one parameter, each
	// for another part of it, says "something from this parameter": the parts
	// are merged into the whole (more than the truth, never less). Without it
	// the work grows with the product of what the cell holds and what the
	// summaries ask of it.
	if e.Kind == Param && e.path != 0 && len(set) > maxCellElems {
		e.path = 0
		s.widened = true
	}
	if old, ok := set[e]; ok {
		// The element is already here. If it came with other guards, the
		// guards recorded for it do not describe it any more.
		if old != nil && w != nil && !(sameGuards(old.guards(), w.guards()) && sameGuards(old.calleeGuards(), w.calleeGuards()) && old.call() == w.call()) {
			old.markUnguarded()
			s.guardEpoch++
		}
		return
	}
	set[e] = w
	s.changed = true
	s.adds++
	if s.rootVer == nil {
		s.rootVer = map[Root]int{}
	}
	s.rootVer[l.Root]++
}

// whyOf returns the reason the element got into the cell, if the analysis
// of the function put it there.
func (s *state) whyOf(r ref) *why {
	cm := s.cells[r.Loc.Root]
	if cm == nil {
		return nil
	}
	set := cm[r.Loc.Path]
	if set == nil {
		return nil
	}
	return set[r.E]
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
	// What is known of a location changes only when something is added to
	// its root: until then, the answer stands.
	ver := s.rootVer[l.Root]
	if c, ok := s.collectCache[l]; ok && c.ver == ver {
		return c.found
	}
	out := s.collectUncached(l)
	if s.collectCache == nil {
		s.collectCache = map[Loc]cachedCollect{}
	}
	s.collectCache[l] = cachedCollect{ver, out}
	if s.collectMemo != nil {
		s.collectMemo[l] = out
	}
	return out
}

func (s *state) collectUncached(l Loc) []found {
	cm := s.cellsOf(l.Root)
	// The answer is made once, at its size: growing it by appending leaves the
	// smaller arrays behind as garbage, and these slices are what the analysis
	// makes the most of.
	paths := s.pathsOf(l.Root, cm)
	first := sort.SearchStrings(paths, l.Path)
	n := 0
	if l.Path != "" {
		n += len(cm[""])
		for i := 1; i < len(l.Path); i++ {
			if l.Path[i] == '/' {
				n += len(cm[l.Path[:i]])
			}
		}
	}
	for i := first; i < len(paths) && strings.HasPrefix(paths[i], l.Path); i++ {
		if pathHasPrefix(paths[i], l.Path) {
			n += len(cm[paths[i]])
		}
	}
	var out []found
	if n > 0 {
		out = make([]found, 0, n)
	}
	// The enclosing objects, outermost first: an enclosing object that is
	// tainted as a whole taints this part.
	enclosing := func(p string) {
		set := cm[p]
		if set == nil {
			return
		}
		rest := l.Path[len(p):]
		for _, e := range s.elemsOf(l.Root, p, set) {
			ext := e
			if e.Kind == Param {
				// Without the bound a recursive structure would extend
				// the path in every pass, forever.
				tp, _ := pathTrunc(e.Path()+rest, s.eng.opts.K)
				ext = ext.WithPath(tp)
			}
			out = append(out, found{E: ext, Cell: Loc{l.Root, p}, Orig: e})
		}
	}
	if l.Path != "" {
		enclosing("")
		for i := 1; i < len(l.Path); i++ {
			if l.Path[i] == '/' {
				enclosing(l.Path[:i])
			}
		}
	}
	// The cell itself and everything stored below it: the paths are sorted,
	// so they are one run.
	for i := first; i < len(paths) && strings.HasPrefix(paths[i], l.Path); i++ {
		p := paths[i]
		if !pathHasPrefix(p, l.Path) {
			continue
		}
		rel := p[len(l.Path):]
		for _, e := range s.elemsOf(l.Root, p, cm[p]) {
			out = append(out, found{E: e, Cell: Loc{l.Root, p}, Orig: e, Rel: rel})
		}
	}
	return out
}

// copyLoc copies the structure below src to dst.
func (s *state) copyLoc(src, dst Loc, w func(f found) *why) {
	for _, f := range s.collect(src) {
		dl := Loc{dst.Root, dst.Path + f.Rel}
		if s.plainExists(dl, f.E) {
			continue
		}
		s.add(dl, f.E, s.withContainerNote(w(f), f.Cell.Path, dl.Path))
	}
}

// pathNote is an uncertainty that belongs to one hop of a path. It is shown
// with the findings whose path contains the hop, not in the program-level
// list: "container_element_imprecision" only matters where it influenced a
// result.
func (s *state) pathNote(kind, detail string, pos ir.Pos) *Uncertainty {
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
	if w == nil || hasKind(w.notes(), KindContainerImprecision) || !(hasElementSegment(from) || hasElementSegment(to)) {
		return w
	}
	w.setNotes(appendNote(w.notes(), s.pathNote(KindContainerImprecision,
		"the elements of the container are not told apart, and writes through other references to it are not followed", w.pos.pos())))
	return w
}

// copyLocs copies from every source location to every destination.
func (s *state) copyLocs(src []Loc, dst []Loc, kind string, in *sg.Instr) {
	s.copyLocsNoted(src, dst, kind, in, nil)
}

// copyLocsNoted is copyLocs with an uncertainty attached to the hop.
func (s *state) copyLocsNoted(src []Loc, dst []Loc, kind string, in *sg.Instr, note *Uncertainty) {
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

// plainExists reports whether the element is already in the cell with a plain
// reason (no guards, no summary applied). Adding it again changes nothing, and
// the reason for the second time need not be made: most additions of a fixed
// point iteration are of this kind, and a reason is the largest thing made.
func (s *state) plainExists(l Loc, e Elem) bool {
	p, _ := pathTrunc(l.Path, s.eng.opts.K)
	cm, ok := s.cells[l.Root]
	if !ok {
		return false
	}
	set := cm[p]
	if set == nil {
		return false
	}
	if e.Kind == Param && e.path != 0 && len(set) > maxCellElems {
		e.path = 0
	}
	old, ok := set[e]
	return ok && old != nil && old.plain()
}

// addsNothing reports whether adding the element, with a plain reason (no
// guards, no summary applied), to every one of the locations would change
// nothing at all: it is there already with a plain reason, and the path and the
// cell are as they would be left (add also records that a path was cut short and
// that a cell was widened, which is why those are not skipped).
func (s *state) addsNothing(dsts []Loc, e Elem) bool {
	if len(dsts) == 0 {
		return false
	}
	for _, l := range dsts {
		if v := l.Root.V; v != nil && l.Root.Idx < 0 && l.Path == "" && !s.eng.opts.NumericTaint &&
			(v.Class == ir.ClassNumber || v.Class == ir.ClassBool) {
			continue // add leaves it out
		}
		p, trunc := pathTrunc(l.Path, s.eng.opts.K)
		if trunc {
			return false
		}
		cm, ok := s.cells[l.Root]
		if !ok {
			return false
		}
		set := cm[p]
		if set == nil {
			return false
		}
		if e.Kind == Param && e.path != 0 && len(set) > maxCellElems {
			return false
		}
		old, ok := set[e]
		if !ok || old == nil || !old.plain() {
			return false
		}
	}
	return true
}

// put adds the found elements to the destination, collapsed.
func (s *state) put(dst Loc, fs []found, w func(f found) *why) {
	for _, f := range fs {
		if s.plainExists(dst, f.E) {
			continue
		}
		s.add(dst, f.E, s.withContainerNote(w(f), f.Cell.Path, dst.Path))
	}
}

func (s *state) whyFn(in *sg.Instr, kind string) func(f found) *why {
	return func(f found) *why {
		w := &why{fn: s.fn, desc: kind, from: &ref{f.Cell, f.Orig}}
		if in != nil {
			w.pos = packPos(in.Pos)
		}
		return w
	}
}

// whyGuarded is whyFn with guards.
func (s *state) whyGuarded(in *sg.Instr, kind string, gs []Guard) func(f found) *why {
	base := s.whyFn(in, kind)
	return func(f found) *why {
		w := base(f)
		w.setGuards(gs)
		return w
	}
}

// copyLocsGuarded copies like copyLocs and records the guards of the hop.
func (s *state) copyLocsGuarded(src []Loc, dst []Loc, kind string, in *sg.Instr, gs []Guard) {
	if len(gs) == 0 {
		s.copyLocs(src, dst, kind, in)
		return
	}
	for _, d := range dst {
		for _, sl := range src {
			s.copyLoc(sl, d, s.whyGuarded(in, kind, gs))
		}
	}
}

func (s *state) whyNote(in *sg.Instr, kind string, u *Uncertainty) func(f found) *why {
	base := s.whyFn(in, kind)
	return func(f found) *why {
		w := base(f)
		w.setNotes(appendNote(w.notes(), u))
		return w
	}
}

// origin is the reason for an element that appears out of nothing (a source
// or a parameter).
func (s *state) origin(pos ir.Pos, kind string) *why {
	return &why{fn: s.fn, pos: packPos(pos), desc: kind}
}

// trace reconstructs the steps that led to the element in the cell.
func (s *state) trace(l Loc, e Elem) *rope {
	key := ref{l, e}
	if r, ok := s.traceMemo[key]; ok {
		return r
	}
	r := s.traceUncached(l, e)
	// The reason an element got into a cell never changes, so the steps do
	// not; but an element that is not there yet has none to give.
	if s.whyOf(key) != nil {
		if s.traceMemo == nil {
			s.traceMemo = map[ref]*rope{}
		}
		s.traceMemo[key] = r
	}
	return r
}

func (s *state) traceUncached(l Loc, e Elem) *rope {
	var chunks []*rope
	seen := map[ref]bool{}
	cur := ref{l, e}
	for len(chunks) < maxPathSteps {
		if seen[cur] {
			break
		}
		seen[cur] = true
		w := s.whyOf(cur)
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
		st := Step{Func: s.eng.funcID(w.fn), Desc: w.desc, pos: w.pos.pos(), Model: w.model(), guards: w.guards()}
		st.Notes = w.notes()
		out = leaf(st)
	} else if len(w.notes()) > 0 {
		out = leaf(Step{Func: s.eng.funcID(w.fn), Desc: w.desc, pos: w.pos.pos(), Notes: w.notes()})
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

// posKey names a position, for keys.
func posKey(p ir.Pos) string { return p.File + ":" + itoa(p.Offset) }

// guardAnswer is a remembered answer of guardsOfChain; it holds until a
// guard is dropped from some element (the epoch changes).
type guardAnswer struct {
	epoch    int
	gs       []Guard
	overflow bool
}

// cachedCollect is a remembered answer of collect, valid while the root has
// the version it had.
type cachedCollect struct {
	ver   int
	found []found
}
