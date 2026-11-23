package ir

import (
	"errors"
	"fmt"
	"sync"
)

// Ref names one function of a Source: its ID and its sequence number, the place it has in the order of the
// program (the order of Program.Funcs). The ID alone is not a name: a program can have several functions with one ID
// (PruneDeps is written for that), so the sequence number tells which.
type Ref struct {
	ID  FuncID
	Seq int
}

// Head is what is known of a function without its body. A reader that needs only this does not ask for the body.
type Head struct {
	// Gone says that the function was dropped (Source.Drop): there is nothing at this number.
	Gone   bool
	ID     FuncID
	Dep    bool
	Symbol string
	Recv   bool
}

// Source is where the readers of the IR get the functions of a program. The readers (the sites, the calls of function
// values, the graph, the listings of the command) do not touch Program.Funcs: they ask the Source, so that where the
// functions are kept (now in memory) is the Source's business. The part of the program that is not a function body
// (the types, the globals, what was not read) is Header.
//
// A Source is for one run, and is read by one goroutine at a time unless its documentation says more.
type Source interface {
	// Header is the program without its functions: a reader takes the types, the aliases and the lists of what was not
	// read from it, and does not read its Funcs.
	Header() *Program
	// Len is the number of functions, in the order of the program.
	Len() int
	// Head is what is known of the function at seq, without its body.
	Head(seq int) Head
	// Func is the function that ref names. It fails when the function cannot be read, or the place holds another one.
	Func(ref Ref) (*Func, error)
	// Lookup is the function that has the ID: the last one when there are several (what a map by ID has held).
	Lookup(id FuncID) (Ref, bool)
	// Each calls fn with every function in the order of the program, and stops at the first error.
	Each(fn func(ref Ref, f *Func) error) error
	// Drop says that the reader does not need the function again.
	Drop(ref Ref)
	// Release says that the reader is done with all the functions.
	Release()
}

// ReadError is a failure to read a function of a Source. A reader that cannot return an error panics with it, and
// the command stops with an internal error (exit code 1): the IR is the main input, and a function that is not
// read is not skipped silently.
type ReadError struct {
	Ref Ref
	Err error
}

func (e *ReadError) Error() string {
	return fmt.Sprintf("the IR of %s (number %d) cannot be read: %v", e.Ref.ID, e.Ref.Seq, e.Err)
}

func (e *ReadError) Unwrap() error { return e.Err }

// Must is the function of the Source at ref, or a panic with a *ReadError.
func Must(src Source, ref Ref) *Func {
	f, err := src.Func(ref)
	if err != nil {
		panic(&ReadError{Ref: ref, Err: err})
	}
	return f
}

// Get is the function that has the ID (the last one, see Lookup) and the Ref of it; nil when there is none. It panics
// with a *ReadError when the function is there and cannot be read.
func Get(src Source, id FuncID) (*Func, Ref) {
	ref, ok := src.Lookup(id)
	if !ok {
		return nil, Ref{}
	}
	return Must(src, ref), ref
}

// HeadOf is the head of the function that has the ID (the last one, see Lookup), without reading its body.
func HeadOf(src Source, id FuncID) (Head, bool) {
	ref, ok := src.Lookup(id)
	if !ok {
		return Head{}, false
	}
	return src.Head(ref.Seq), true
}

// Walk is Each for a reader that cannot return an error: a failure is a panic with a *ReadError.
func Walk(src Source, fn func(ref Ref, f *Func)) {
	err := src.Each(func(ref Ref, f *Func) error {
		fn(ref, f)
		return nil
	})
	if err != nil {
		var re *ReadError
		if errors.As(err, &re) {
			panic(re) // the Source says which function
		}
		panic(&ReadError{Err: err})
	}
}

// WalkProject calls fn with the functions of the code being analysed, not those of the dependencies (Head.Dep), in the
// order of the program, until fn returns false. The body of a dependency is not asked for. A failure to read is a
// panic with a *ReadError.
func WalkProject(src Source, fn func(ref Ref, f *Func) bool) {
	for i, n := 0, src.Len(); i < n; i++ {
		h := src.Head(i)
		if h.Gone || h.Dep {
			continue
		}
		ref := Ref{ID: h.ID, Seq: i}
		if !fn(ref, Must(src, ref)) {
			return
		}
	}
}

// WalkProjectIf is WalkProject for the functions that want says yes to (by their number): the body of another one is
// not asked for.
func WalkProjectIf(src Source, want func(seq int) bool, fn func(ref Ref, f *Func) bool) {
	for i, n := 0, src.Len(); i < n; i++ {
		h := src.Head(i)
		if h.Gone || h.Dep || !want(i) {
			continue
		}
		ref := Ref{ID: h.ID, Seq: i}
		if !fn(ref, Must(src, ref)) {
			return
		}
	}
}

// MemSource is the Source of a program whose functions are in memory (Program.Funcs). The *Func it gives is the one
// in the program: what a reader writes to it (the candidates of the calls of function values) stays.
func MemSource(p *Program) Source { return &memSource{p: p} }

type memSource struct {
	p     *Program
	once  sync.Once
	last  map[FuncID]int
	sonce sync.Once
	sites *SiteIndex
}

func (m *memSource) siteIndex() *SiteIndex {
	m.sonce.Do(func() {
		m.sites = NewSiteIndex()
		for i, f := range m.p.Funcs {
			if f != nil {
				m.sites.Add(i, f, nil)
			}
		}
	})
	return m.sites
}

func (m *memSource) gone(seq int) bool { return m.p.Funcs[seq] == nil }

func (m *memSource) CallersOf(id FuncID) []CallSite { return m.siteIndex().Callers(id, m.gone) }
func (m *memSource) MakersOf(id FuncID) []CallSite  { return m.siteIndex().Makers(id, m.gone) }

func (m *memSource) Header() *Program { return m.p }

func (m *memSource) Len() int { return len(m.p.Funcs) }

func (m *memSource) Head(seq int) Head {
	f := m.p.Funcs[seq]
	if f == nil {
		return Head{Gone: true}
	}
	return Head{ID: f.ID, Dep: f.Dep, Symbol: f.Symbol, Recv: f.Recv}
}

func (m *memSource) Func(ref Ref) (*Func, error) {
	if ref.Seq < 0 || ref.Seq >= len(m.p.Funcs) {
		return nil, fmt.Errorf("no function at number %d", ref.Seq)
	}
	f := m.p.Funcs[ref.Seq]
	if f == nil {
		return nil, fmt.Errorf("the function at number %d was dropped", ref.Seq)
	}
	if f.ID != ref.ID {
		return nil, fmt.Errorf("the function at number %d is %s", ref.Seq, f.ID)
	}
	return f, nil
}

func (m *memSource) Lookup(id FuncID) (Ref, bool) {
	m.once.Do(func() {
		m.last = make(map[FuncID]int, len(m.p.Funcs))
		for i, f := range m.p.Funcs {
			if f != nil {
				m.last[f.ID] = i
			}
		}
	})
	i, ok := m.last[id]
	return Ref{ID: id, Seq: i}, ok
}

func (m *memSource) Each(fn func(ref Ref, f *Func) error) error {
	for i, f := range m.p.Funcs {
		if f == nil {
			continue
		}
		if err := fn(Ref{ID: f.ID, Seq: i}, f); err != nil {
			return err
		}
	}
	return nil
}

func (m *memSource) Drop(ref Ref) {
	if ref.Seq >= 0 && ref.Seq < len(m.p.Funcs) {
		m.p.Funcs[ref.Seq] = nil
	}
}

func (m *memSource) Release() {
	m.p.Funcs = nil
	m.last = nil
	m.sites = nil
}

// ConcatSource is the Source of several Sources one after the other, in the order given (the functions of one
// language, then the next). header is its Header: the part of the program that is not a function, which the caller made
// from the headers of the Sources. A function has the number it has in its Source plus the number of the functions of
// the Sources before it. When an ID is in several, Lookup is the last one.
func ConcatSource(header *Program, srcs ...Source) Source {
	if len(srcs) == 1 && header == srcs[0].Header() {
		return srcs[0]
	}
	c := &concatSource{hdr: header, srcs: srcs, base: make([]int, len(srcs)+1)}
	for i, s := range srcs {
		c.base[i+1] = c.base[i] + s.Len()
	}
	return c
}

type concatSource struct {
	hdr   *Program
	srcs  []Source
	base  []int // base[i] is the number of the first function of srcs[i]
	sited []Sited
}

// at is the Source that holds the number seq, and the number it has there.
func (c *concatSource) at(seq int) (Source, int) {
	for i := len(c.srcs) - 1; i >= 0; i-- {
		if seq >= c.base[i] {
			return c.srcs[i], seq - c.base[i]
		}
	}
	return c.srcs[0], seq
}

func (c *concatSource) Header() *Program { return c.hdr }
func (c *concatSource) Len() int         { return c.base[len(c.srcs)] }

func (c *concatSource) Head(seq int) Head {
	s, i := c.at(seq)
	return s.Head(i)
}

func (c *concatSource) Func(ref Ref) (*Func, error) {
	s, i := c.at(ref.Seq)
	return s.Func(Ref{ID: ref.ID, Seq: i})
}

func (c *concatSource) Lookup(id FuncID) (Ref, bool) {
	for i := len(c.srcs) - 1; i >= 0; i-- {
		if r, ok := c.srcs[i].Lookup(id); ok {
			r.Seq += c.base[i]
			return r, true
		}
	}
	return Ref{}, false
}

func (c *concatSource) Each(fn func(ref Ref, f *Func) error) error {
	for i, s := range c.srcs {
		base := c.base[i]
		err := s.Each(func(ref Ref, f *Func) error {
			ref.Seq += base
			return fn(ref, f)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *concatSource) Drop(ref Ref) {
	s, i := c.at(ref.Seq)
	s.Drop(Ref{ID: ref.ID, Seq: i})
}

func (c *concatSource) Release() {
	for _, s := range c.srcs {
		s.Release()
	}
}

// Targets is the Targeter answer of the Source that holds seq: its own when it has one, else from the function.
func (c *concatSource) Targets(seq int) []FuncID {
	s, i := c.at(seq)
	if t, ok := s.(Targeter); ok {
		return t.Targets(i)
	}
	var ids []FuncID
	h := s.Head(i)
	Targets(Must(s, Ref{ID: h.ID, Seq: i}), func(id FuncID) { ids = append(ids, id) })
	return ids
}

// Marks is the Marker answer of the Source that holds seq: its own when it has one, else from the function.
func (c *concatSource) Marks(seq int) Marks {
	s, i := c.at(seq)
	if m, ok := s.(Marker); ok {
		return m.Marks(i)
	}
	h := s.Head(i)
	return MarksOf(Must(s, Ref{ID: h.ID, Seq: i}))
}

// CallSymbols is the CallSymbolser answer of the Source that holds seq.
func (c *concatSource) CallSymbols(seq int) []string {
	s, i := c.at(seq)
	return CallSymbols(s, i)
}

// CallersOf are the calls that reach the function, the ones of each Source one after the other.
func (c *concatSource) CallersOf(id FuncID) []CallSite {
	return c.sites(id, func(s Sited) []CallSite { return s.CallersOf(id) })
}

// MakersOf are the make_closure instructions that name the function, the ones of each Source one after the other.
func (c *concatSource) MakersOf(id FuncID) []CallSite {
	return c.sites(id, func(s Sited) []CallSite { return s.MakersOf(id) })
}

func (c *concatSource) sites(id FuncID, of func(Sited) []CallSite) []CallSite {
	if len(c.sited) == 0 {
		c.sited = make([]Sited, len(c.srcs))
		for i, s := range c.srcs {
			c.sited[i] = SitedOf(s)
		}
	}
	var out []CallSite
	for i, s := range c.sited {
		for _, x := range of(s) {
			x.Seq += int32(c.base[i])
			out = append(out, x)
		}
	}
	return out
}

// Update tells the Source that holds the function.
func (c *concatSource) Update(ref Ref, block, instr int, cal *Callee) {
	s, i := c.at(ref.Seq)
	if u, ok := s.(Updater); ok {
		u.Update(Ref{ID: ref.ID, Seq: i}, block, instr, cal)
	}
}

// SitedOf is the Source as a Sited: itself when it is one, else an index that is made by reading all its functions
// once.
func SitedOf(src Source) Sited {
	if s, ok := src.(Sited); ok {
		return s
	}
	x := NewSiteIndex()
	for i, n := 0, src.Len(); i < n; i++ {
		if h := src.Head(i); !h.Gone {
			x.Add(i, Must(src, Ref{ID: h.ID, Seq: i}), nil)
		}
	}
	return indexed{x, src}
}

type indexed struct {
	x   *SiteIndex
	src Source
}

func (i indexed) gone(seq int) bool              { return i.src.Head(seq).Gone }
func (i indexed) CallersOf(id FuncID) []CallSite { return i.x.Callers(id, i.gone) }
func (i indexed) MakersOf(id FuncID) []CallSite  { return i.x.Makers(id, i.gone) }
