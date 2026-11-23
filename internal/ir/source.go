package ir

import (
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

// MemSource is the Source of a program whose functions are in memory (Program.Funcs). The *Func it gives is the one
// in the program: what a reader writes to it (the candidates of the calls of function values) stays.
func MemSource(p *Program) Source { return &memSource{p: p} }

type memSource struct {
	p    *Program
	once sync.Once
	last map[FuncID]int
}

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
}
