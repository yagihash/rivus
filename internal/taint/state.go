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

// whyRef is a why and the reference it holds, made in one piece (one
// allocation for the two, which are made millions of times).
type whyRef struct {
	w why
	r ref
}

// newWhyFrom makes a why that came from the element e of the location l.
func newWhyFrom(fn *sg.Func, desc string, l Loc, e Elem) *why {
	x := &whyRef{}
	x.r = ref{l, e}
	x.w = why{fn: fn, desc: desc, from: &x.r}
	return &x.w
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

// isParamSeed reports whether the hop is the seed of a `kind: param` source (put
// into the cell of the parameter by seedParam), not something the function did.
func (w *why) isParamSeed() bool {
	return w != nil && w.from == nil && strings.HasPrefix(w.desc, "source: parameter ")
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
	// Cut: the path of E was cut at the depth limit when it was extended to the read path (a read below a cell that
	// holds the element): the hop that carries it carries access_path_truncated. It is not part of what the element is.
	Cut bool
}

// state is the analysis state of one function.
type state struct {
	eng *Engine
	// useIndex: the instructions that use a value of this function (made when needed: callrows.go)
	useIndex map[*sg.Value][]*sg.Instr
	// sinkNotes are the uncertainties of the sink being checked (a sink behind an interface).
	sinkNotes []*Uncertainty
	fn        *sg.Func

	cells   map[Root]map[string]Set
	changed bool

	// limited are the parameters whose value this function calls through a call that is over the candidate limit
	// (Summary.LimitedParams)
	limited map[int]bool
	// unresolved are the parameters that this function holds a function in that it calls with no candidate
	// (Summary.UnresolvedParams)
	unresolved map[int]bool

	paramRoot map[Root]int // parameter and free variable roots -> index
	numbering map[*sg.Value]int
	// numericStops: by source site, where its data stopped at a number or a bool here.
	numericStops map[string]numericStop
	// numericCount is how many times an element (a real one or a symbolic one of a parameter) was dropped here because
	// its place is a number or a bool (summary.numeric_stops). A visit counts what it sees; the last complete one stays.
	numericCount int         // other causes: a length, a counter, a conversion, a comparison (real elements only)
	charCount    int         // a value read as an element of a string, bytes or runes (any element: a parameter's too)
	stores       []*sg.Instr // the stores of the function (aliasLocs looks for what a cell holds)
	// ptrStores are the stores (store, field_store) of a pointer into a cell, and valStores the stores of
	// a value that is not a pointer (a closure's environment copied into the variable that holds it):
	// the two make ptrAlias (ptrcells.go). Only in a program of a language that has closure environments.
	ptrStores []*sg.Instr
	valStores []*sg.Instr
	ptrAlias  map[Loc]map[Loc]struct{}
	copyOf    map[Loc][]Loc // a cell that holds a copy of a struct -> the cells it was copied from
	locSet    map[*sg.Value]map[Loc]bool
	locCache  map[*sg.Value][]Loc
	// the function values seeded in this function (Options.FuncValues)
	funcSeeded map[*sg.Value]bool
	locsFinal  bool // locCache may be filled: the fixed point of the locations is reached

	branches        *[]*sg.Block
	edgeGuardCache  map[[2]*sg.Block][]Guard
	guardOverflow   bool
	condCache       map[*sg.Block]*condBase
	blockGuardCache map[*sg.Block][]Guard
	rootVer         map[Root]int // number of elements added to each root: change detector
	// addLog is, for each root, the path of each element added to it, in
	// order: rootVer[r] is the length of addLog[r], so a number of it is a place
	// in the log, and what was added since is what follows.
	addLog       map[Root][]string
	callSigs     map[*sg.Instr]int
	sortedElems  map[Loc]cachedElems
	sortedPaths  map[Root]cachedPaths
	collectMemo  map[Loc][]found // active during one summary application
	collectCache map[Loc]cachedCollect
	traceMemo    map[ref]*rope
	guardMemo    map[ref]guardAnswer
	guardEpoch   int // changes when an element loses the guards recorded for it
	expandMemo   map[Elem][]expanded
	// recvBoxed: the receiver of the call being applied is an interface value that holds a
	// pointer (what it points to lives under "/*"), and the callee takes the pointer itself
	recvBoxed bool

	// opaque are the roots of the values that a call neither read nor modelled was given (noteOpaque)
	opaque map[Root]bool
	// opaqueBy is the instruction that marked a root (multiMark: more than one): a sink does not see its own mark
	opaqueBy map[Root]*sg.Instr

	suppressions map[string]*Suppression
	findings     map[findingKey]*rawFinding
	sinkReaches  map[sinkKey]*SinkReach
	uncertain    []*Uncertainty
	truncated    bool
	widened      bool
	adds         int
	exceeded     bool
	// exceededBy says which limit stopped the analysis (degBudget, degFuncTimeout, degRunTimeout)
	exceededBy uint8
}

// k is the access path depth of this function: Options.DepK in a function of a dependency (when it is set), Options.K in
// the others.
func (s *state) k() int {
	if s.fn != nil && s.fn.Dep && s.eng.opts.DepK > 0 {
		return s.eng.opts.DepK
	}
	return s.eng.opts.K
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
				cp := make(Set, len(set))
				for e := range set {
					cp[e] = nil
				}
				m[path] = cp
			}
		}
	}
	return m
}

// numericStop is the first place where the data of a source site stopped at a number
// or a bool in a function.
type numericStop struct {
	label string
	pos   ir.Pos
}

// noteNumericStop records that data that came from a source (a concrete label) was about to
// become a number or a bool, where it is not followed: the place where an input is parsed
// into a number, or measured.
func (s *state) noteNumericStop(e Elem, w *why, v *sg.Value) {
	origin := e.Origin()
	if _, ok := s.numericStops[origin]; ok {
		return
	}
	pos := v.Pos
	if w != nil && w.pos != (cpos{}) {
		pos = w.pos.pos()
	}
	if s.numericStops == nil {
		s.numericStops = map[string]numericStop{}
	}
	s.numericStops[origin] = numericStop{label: e.Label(), pos: pos}
}

// countNumericStop counts one dropped element (summary.numeric_stops). v is the number or bool that it did not enter: when
// that is a character read, every element counts (a parameter's too: the text is the parameter); otherwise only the data
// of a source does (the symbolic elements of parameters are everywhere and say nothing).
func (s *state) countNumericStop(v *sg.Value, e Elem) {
	if isCharRead(v) {
		s.charCount++
	} else if e.Kind == Real {
		s.numericCount++
	}
}

// isCharRead reports whether the value is one element read out of text (not at a constant index): a byte or a rune of a string (s[i], range s,
// codeUnitAt, chars()), or of a byte or rune sequence. By its type where the language keeps byte, rune and char apart
// from the other numbers (Go, Rust), and by the instruction that made it.
func isCharRead(v *sg.Value) bool {
	if v == nil {
		return false
	}
	if constIndexRead(v) {
		return false // s[0]: one place of the text, not the text handled one character at a time
	}
	v = throughCopies(v)
	in := v.Instr
	switch string(v.Type) {
	case "byte", "rune", "char":
		return true
	}
	if in == nil {
		return false
	}
	switch in.Op {
	case ir.OpFieldLoad:
		// Rust: the value inside the Option that Iterator::next of the bytes or the chars of a str gave
		if len(in.Args) > 0 && in.Args[0] != nil {
			if nx := throughCopies(in.Args[0]); nx.Instr != nil && nx.Instr.Op == ir.OpCall && calleeSymbol(nx.Instr) == "std::iter::Iterator::next" &&
				len(nx.Instr.Args) > 0 && nx.Instr.Args[0] != nil {
				it := string(nx.Instr.Args[0].Type)
				return strings.Contains(it, "std::str::Bytes") || strings.Contains(it, "std::str::Chars") || strings.Contains(it, "std::str::CharIndices")
			}
		}
	case ir.OpIndexLoad:
		// s[i] of a string, or u[i] of the code units or the runes of one (Dart)
		return len(in.Args) > 0 && in.Args[0] != nil && (in.Args[0].Class == ir.ClassString || isTextSequence(in.Args[0]))
	case ir.OpCall:
		sym := calleeSymbol(in)
		for _, n := range charReadSymbols {
			if strings.HasSuffix(sym, n) {
				return true
			}
		}
		// the element of a loop over the code units or the runes of a string (Dart: iter.start(s.codeUnits), then iter.next)
		if sym == "iter.next" && len(in.Args) > 0 && in.Args[0] != nil {
			if st := throughCopies(in.Args[0]); st.Instr != nil && st.Instr.Op == ir.OpCall && calleeSymbol(st.Instr) == "iter.start" &&
				len(st.Instr.Args) > 0 && st.Instr.Args[0] != nil {
				return isTextSequence(st.Instr.Args[0])
			}
		}
	}
	return false
}

// constIndexRead reports whether the value is an element at a constant index (through copies and conversions: Rust
// `s.as_bytes()[0] as char`).
func constIndexRead(v *sg.Value) bool {
	for i := 0; i < 8 && v != nil && v.Instr != nil; i++ {
		in := v.Instr
		switch {
		case (in.Op == ir.OpCopy || in.Op == ir.OpConvert) && len(in.Args) == 1:
			v = in.Args[0]
		case in.Op == ir.OpIndexLoad && len(in.Args) > 1 && in.Args[1] != nil:
			return throughCopies(in.Args[1]).Kind == sg.KConst
		default:
			return false
		}
	}
	return false
}

// throughCopies follows copies back to the value that was made (a few steps: a copy chain is short).
func throughCopies(v *sg.Value) *sg.Value {
	for i := 0; i < 8 && v.Instr != nil && v.Instr.Op == ir.OpCopy && len(v.Instr.Args) == 1 && v.Instr.Args[0] != nil; i++ {
		v = v.Instr.Args[0]
	}
	return v
}

func calleeSymbol(in *sg.Instr) string {
	if in.Callee == nil {
		return ""
	}
	return in.Callee.Symbol
}

// isTextSequence reports whether the value is the code units or the runes of a text (Dart: s.codeUnits, s.runes; Rust: s.as_bytes()).
func isTextSequence(v *sg.Value) bool {
	v = throughCopies(v)
	if v.Instr == nil || v.Instr.Op != ir.OpCall {
		return false
	}
	sym := calleeSymbol(v.Instr)
	return strings.HasSuffix(sym, "String.codeUnits") || strings.HasSuffix(sym, "String.runes") || strings.HasSuffix(sym, "::as_bytes")
}

// charReadSymbols end the symbols of the calls that give one character (a code unit) of a text.
var charReadSymbols = []string{".codeUnitAt"}

// numericRoot reports whether the root is a number or a bool that is not followed (what `add` stops at).
func (s *state) numericRoot(r Root) bool {
	v := r.V
	return v != nil && r.Idx < 0 && !s.eng.opts.NumericTaint && (v.Class == ir.ClassNumber || v.Class == ir.ClassBool)
}

// target is where `add` puts an element, worked out once: `add`, `plainExists` and `addsNothing` all
// decide from it, so that a change of what add does (what it leaves out, how it cuts a path, when it
// widens a cell) is a change of this function and of nothing else.
type target struct {
	loc     Loc  // the location, its path cut at K
	trunc   bool // the path was cut
	numeric bool // a number or a bool that is not followed: add leaves it out (and says so for a real element)
	set     Set  // what the cell holds (nil: there is nothing there yet)
	e       Elem // the element, widened when the cell holds very many symbolic elements of one parameter
	widened bool // e was widened
}

// numericLoc reports whether the location is a number or a bool as a whole, which taint stops at
// unless asked to follow it (a rule whose sink is a size or a count needs that): it cannot carry an
// injection (a path, a query, a command, a URL).
func (s *state) numericLoc(l Loc) bool {
	return l.Path == "" && s.numericRoot(l.Root)
}

// targetOf works out where adding e at l would put it. With create the cell and its set are made, as
// add needs; without, they are only looked up.
func (s *state) targetOf(l Loc, e Elem, create bool) target {
	t := target{loc: l, e: e}
	if s.numericLoc(l) {
		t.numeric = true
		return t
	}
	t.loc.Path, t.trunc = pathTrunc(l.Path, s.k())
	if create {
		cm := s.cellsOf(l.Root)
		t.set = cm[t.loc.Path]
		if t.set == nil {
			t.set = Set{}
			cm[t.loc.Path] = t.set
		}
	} else if cm, ok := s.cells[l.Root]; ok {
		t.set = cm[t.loc.Path]
	}
	// A cell that holds very many symbolic elements of one parameter, each
	// for another part of it, says "something from this parameter": the parts
	// are merged into the whole (more than the truth, never less). Without it
	// the work grows with the product of what the cell holds and what the
	// summaries ask of it.
	if e.Kind == Param && e.path != 0 && len(t.set) > maxCellElems {
		t.e.path = 0
		t.widened = true
	}
	return t
}

// add puts an element into a cell. It records w as the reason the first time.
func (s *state) add(l Loc, e Elem, w *why) {
	t := s.targetOf(l, e, true)
	if t.numeric {
		s.countNumericStop(l.Root.V, e)
		if e.Kind == Real {
			s.noteNumericStop(e, w, l.Root.V)
		}
		return
	}
	if t.trunc {
		s.truncated = true
		// the hop that put the element into the cell below the cut: the cell stands for everything under it
		w = s.withCutNote(w, l.Root, t.loc.Path, pathDepth(l.Path), "stored")
	}
	if t.widened {
		s.widened = true
	}
	l, e, set := t.loc, t.e, t.set
	if old, ok := set[e]; ok {
		// The element is already here. If it came with other guards, the
		// guards recorded for it do not describe it any more.
		if old != nil && w != nil {
			differ := !(sameGuards(old.guards(), w.guards()) && sameGuards(old.calleeGuards(), w.calleeGuards()) && old.call() == w.call())
			// The guards of a hop are not all: a φ or a return up the chain
			// may hold others. If the two come from different places, the
			// guards of the whole chains are compared.
			if !differ && !old.unguarded() && !sameFrom(old.from, w.from) {
				k := ref{l, e}
				og, oo := s.guardsOfChain(l, e)
				ng, no := s.guardsOfChainFrom(w, k)
				differ = oo || no || !sameGuards(og, ng)
			}
			if differ {
				old.markUnguarded()
				s.guardEpoch++
			}
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
	if s.addLog == nil {
		s.addLog = map[Root][]string{}
	}
	// one entry for each step of the version of the root (collect reads the log by it)
	s.addLog[l.Root] = append(s.addLog[l.Root], l.Path)
}

// touches reports whether an element added at the path p of a root changes what
// collect says of the path l: it does when p is l, an object that encloses l, or
// something stored below l (the three things collectUncached reads).
func touches(l, p string) bool {
	if pathHasPrefix(p, l) {
		return true // l itself, or below it
	}
	if l == "" {
		return false
	}
	return p == "" || (len(p) < len(l) && l[len(p)] == '/' && strings.HasPrefix(l, p))
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
	if c, ok := s.collectCache[l]; ok {
		if c.ver == ver {
			return c.found
		}
		// Something was added to the root, but not necessarily to what this
		// location reads: if not, the answer stands.
		stale := false
		for _, p := range s.addLog[l.Root][c.ver:ver] {
			if touches(l.Path, p) {
				stale = true
				break
			}
		}
		if !stale {
			s.collectCache[l] = cachedCollect{ver, c.found}
			if s.collectMemo != nil {
				s.collectMemo[l] = c.found
			}
			return c.found
		}
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
			isCut := false
			if e.Kind == Param {
				// Without the bound a recursive structure would extend
				// the path in every pass, forever.
				tp, cut := pathTrunc(e.Path()+rest, s.k())
				ext = ext.WithPath(tp)
				isCut = cut
			}
			out = append(out, found{E: ext, Cell: Loc{l.Root, p}, Orig: e, Cut: isCut})
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
		if s.plainExists(dl, f.E, ref{f.Cell, f.Orig}, true) {
			continue
		}
		s.add(dl, f.E, s.withContainerNote(w(f), f.Cell.Path, dl.Path))
	}
}

// withCutNote is the hop with the access_path_truncated note added (a copy: the hop may be shared with the other
// destinations of a write). Only the hop that goes through a cut path has it: the part below the cut is taken to be
// tainted as well, so a flow that goes on from here may be there only because of the cut. The note says where (the
// function, the root, the path kept: the first levels before the cut), at which depth, and how deep the path was.
// verb is "stored" (the element was put into the cell at the cut path) or "read" (the element was read below it).
func (s *state) withCutNote(w *why, root Root, kept string, fullDepth int, verb string) *why {
	if w == nil || s.fn == nil || hasKind(w.notes(), KindAccessPathTruncated) {
		return w
	}
	c := *w
	if w.rare != nil {
		r := *w.rare
		c.rare = &r
	}
	scope := "project"
	if s.fn.Dep {
		scope = "dependency"
	}
	rn := "?"
	if root.V != nil {
		rn = root.V.Name
	}
	if len(kept) > 80 {
		kept = kept[:80] + "..."
	}
	c.setNotes(appendNote(c.notes(), s.pathNote(KindAccessPathTruncated,
		"an access path was cut at depth "+itoa(s.k())+" ("+scope+" function "+s.eng.funcID(s.fn)+"): the value was "+verb+" at "+rn+kept+
			" from "+deepText(fullDepth)+"; what is below the cut is taken to be tainted as well, so the flow from here "+
			"may exist only because of the cut (more flows than there are, never fewer)", w.pos.pos())))
	return &c
}

func deepText(n int) string {
	if n > 0 {
		return "a path " + itoa(n) + " levels deep"
	}
	return "a longer path"
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
	if len(s.opaque) > 0 {
		s.carryOpaque(src, dst)
	}
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
func (s *state) plainExists(l Loc, e Elem, from ref, hasFrom bool) bool {
	t := s.targetOf(l, e, false)
	if t.numeric {
		return false // add has something to say about it
	}
	old, ok := t.set[t.e]
	return ok && old != nil && old.plain() && s.sameChainGuards(old, from, hasFrom)
}

// sameChainGuards reports whether the plain hop old and a hop that would
// come from nf have the same guards up their chains. If not, the element
// gets here under other conditions, and it has to be added to be told so.
func (s *state) sameChainGuards(old *why, nf ref, hasNf bool) bool {
	if old.unguarded() || (old.from == nil && !hasNf) || (old.from != nil && hasNf && *old.from == nf) {
		return true
	}
	var og, ng []Guard
	var oo, no bool
	if old.from != nil {
		og, oo = s.guardsOfChain(old.from.Loc, old.from.E)
	}
	if hasNf {
		ng, no = s.guardsOfChain(nf.Loc, nf.E)
	}
	return !oo && !no && sameGuards(og, ng)
}

// addsNothing reports whether adding the element, with a plain reason (no
// guards, no summary applied), to every one of the locations would change
// nothing at all: it is there already with a plain reason, and the path and the
// cell are as they would be left (add also records that a path was cut short and
// that a cell was widened, which is why those are not skipped).
func (s *state) addsNothing(dsts []Loc, e Elem, from ref, hasFrom bool) bool {
	if len(dsts) == 0 {
		return false
	}
	for _, l := range dsts {
		t := s.targetOf(l, e, false)
		if t.numeric {
			continue // add leaves it out
		}
		// add records that a path was cut and that a cell was widened
		if t.trunc || t.widened || t.set == nil {
			return false
		}
		old, ok := t.set[t.e]
		if !ok || old == nil || !old.plain() || !s.sameChainGuards(old, from, hasFrom) {
			return false
		}
	}
	return true
}

// put adds the found elements to the destination, collapsed.
func (s *state) put(dst Loc, fs []found, w func(f found) *why) {
	for _, f := range fs {
		if s.plainExists(dst, f.E, ref{f.Cell, f.Orig}, true) {
			continue
		}
		s.add(dst, f.E, s.withContainerNote(w(f), f.Cell.Path, dst.Path))
	}
}

func (s *state) whyFn(in *sg.Instr, kind string) func(f found) *why {
	return func(f found) *why {
		w := newWhyFrom(s.fn, kind, f.Cell, f.Orig)
		if in != nil {
			w.pos = packPos(in.Pos)
		}
		if f.Cut {
			w = s.withCutNote(w, f.Cell.Root, f.E.Path(), -1, "read")
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
	cut := false
	for len(chunks) < maxPathSteps {
		if seen[cur] {
			break
		}
		seen[cur] = true
		w := s.whyOf(cur)
		if w == nil {
			// An element read from the global store has no reason here: the
			// function that wrote it does (CORE-B).
			if g := cur.Loc.Root.V; g != nil && g.Kind == sg.KGlobal {
				if ws := s.eng.globals.writerSteps(g, cur.Loc.Path, cur.E); ws != nil {
					chunks = append(chunks, ws)
				}
			}
			break
		}
		chunks = append(chunks, s.stepsOf(w))
		if w.from == nil {
			break
		}
		cur = *w.from
		if len(chunks) == maxPathSteps && !seen[cur] {
			cut = true // the walk back to the source stops here: what is before is not in the path
		}
	}
	var out *rope
	if cut {
		n := newUncertainty(KindDepthLimit, "path_steps_omitted: the path was followed back for "+itoa(maxPathSteps)+
			" reasons and no further; the steps before this one (up to the source) were left out", "")
		out = leaf(newRec("", "the steps before this one were left out", cpos{}, nil, []*Uncertainty{n}))
	}
	for i := len(chunks) - 1; i >= 0; i-- {
		out = cat(out, chunks[i])
	}
	return out
}

func (s *state) stepsOf(w *why) *rope {
	var out *rope
	if w.desc != "" && !skipStep(w.desc) {
		out = leaf(newRec(s.eng.funcID(w.fn), w.desc, w.pos, w.model(), w.notes()))
	} else if len(w.notes()) > 0 {
		out = leaf(newRec(s.eng.funcID(w.fn), w.desc, w.pos, nil, w.notes()))
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

func sameFrom(a, b *ref) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
