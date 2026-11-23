package ir

import "sort"

// Marks are what is known of a function without reading its body, for a reader that needs only some of the functions:
// a reader that looks for the calls of function values reads the functions that have one, and the others are not read.
type Marks uint8

const (
	// MarkValueCall: the function has a call of a function value that is given an argument (Callee.Kind CalleeValue).
	MarkValueCall Marks = 1 << iota
	// MarkArgNames: the function has a call that passes its arguments by name (Instr.ArgNames).
	MarkArgNames
)

// MarksOf are the marks of the function.
func MarksOf(f *Func) Marks {
	var m Marks
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Instrs {
			in := &f.Blocks[bi].Instrs[ii]
			if in.Op != OpCall || in.Callee == nil {
				continue
			}
			if in.Callee.Kind == CalleeValue && len(in.Args) > 0 {
				m |= MarkValueCall
			}
			if len(in.ArgNames) > 0 {
				m |= MarkArgNames
			}
		}
	}
	return m
}

// CallSymbolsOf are the symbols of the calls the function makes (not the calls of function values), each once, in the
// order of first use.
func CallSymbolsOf(f *Func) []string {
	var out []string
	seen := map[string]bool{}
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Instrs {
			in := &f.Blocks[bi].Instrs[ii]
			if in.Op != OpCall || in.Callee == nil || in.Callee.Kind == CalleeValue || in.Callee.Symbol == "" || seen[in.Callee.Symbol] {
				continue
			}
			seen[in.Callee.Symbol] = true
			out = append(out, in.Callee.Symbol)
		}
	}
	return out
}

// Marker is a Source that knows the marks of its functions without reading them.
type Marker interface {
	Marks(seq int) Marks
}

// WalkMarked calls fn with the functions that have one of the marks in want, in the order of the program: the other
// functions are not read when the Source is a Marker. A failure to read is a panic with a *ReadError.
func WalkMarked(src Source, want Marks, fn func(ref Ref, f *Func)) {
	mk, _ := src.(Marker)
	for i, n := 0, src.Len(); i < n; i++ {
		h := src.Head(i)
		if h.Gone {
			continue
		}
		if mk != nil && mk.Marks(i)&want == 0 {
			continue
		}
		ref := Ref{ID: h.ID, Seq: i}
		f := Must(src, ref)
		if mk == nil && MarksOf(f)&want == 0 {
			continue
		}
		fn(ref, f)
	}
}

// CallSymbolser is a Source that knows the symbols of the calls of the functions of the project (not of the
// dependencies) without reading them (CallSymbolsOf).
type CallSymbolser interface {
	CallSymbols(seq int) []string
}

// CallSymbols are the symbols of the calls that the function at seq makes (CallSymbolsOf); the function is read only
// when the Source does not know them.
func CallSymbols(src Source, seq int) []string {
	if c, ok := src.(CallSymbolser); ok {
		return c.CallSymbols(seq)
	}
	h := src.Head(seq)
	return CallSymbolsOf(Must(src, Ref{ID: h.ID, Seq: seq}))
}

// CallSite is a call or a make_closure instruction: the function (its number in the Source), the block and the
// instruction.
type CallSite struct {
	Seq, Block, Instr int32
}

// SiteIndex says where the functions are called and where the closures are made: the calls that reach a function are
// the static ones (Callee.Func) and the candidates of the calls (Callee.Candidates), and the places of a closure are
// the make_closure instructions that name it. Both are in the order of the program, then of the instructions.
type SiteIndex struct {
	callers map[FuncID][]CallSite
	makers  map[FuncID][]CallSite
}

// NewSiteIndex is an empty index.
func NewSiteIndex() *SiteIndex {
	return &SiteIndex{callers: map[FuncID][]CallSite{}, makers: map[FuncID][]CallSite{}}
}

// Add lists the calls and the make_closure instructions of the function at seq. The numbers are added in the order of
// the program (see Remap for another order). intern, when it is not nil, gives the one string for an ID, so that the
// keys of the index do not keep the strings of the function.
func (x *SiteIndex) Add(seq int, f *Func, intern func(FuncID) FuncID) {
	id := func(s FuncID) FuncID {
		if intern != nil {
			return intern(s)
		}
		return s
	}
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Instrs {
			in := &f.Blocks[bi].Instrs[ii]
			site := CallSite{int32(seq), int32(bi), int32(ii)}
			switch in.Op {
			case OpMakeClosure:
				k := id(in.Func)
				x.makers[k] = append(x.makers[k], site)
			case OpCall:
				if in.Callee == nil {
					continue
				}
				if in.Callee.Func != "" {
					k := id(in.Callee.Func)
					x.callers[k] = append(x.callers[k], site)
				}
				for _, c := range in.Callee.Candidates {
					k := id(c)
					x.callers[k] = append(x.callers[k], site)
				}
			}
		}
	}
}

// Remap gives the functions other numbers: the function that was added as old has the number to[old]. The sites of an
// ID are again in the order of the numbers.
func (x *SiteIndex) Remap(to []int) {
	for _, m := range []map[FuncID][]CallSite{x.callers, x.makers} {
		for _, sites := range m {
			for i := range sites {
				sites[i].Seq = int32(to[sites[i].Seq])
			}
			sort.SliceStable(sites, func(i, j int) bool { return less(sites[i], sites[j]) })
		}
	}
}

func less(a, b CallSite) bool {
	if a.Seq != b.Seq {
		return a.Seq < b.Seq
	}
	if a.Block != b.Block {
		return a.Block < b.Block
	}
	return a.Instr < b.Instr
}

// Callers are the calls that reach the function with the ID. gone says that a function is not in the program any more
// (Source.Drop): its calls are left out.
func (x *SiteIndex) Callers(id FuncID, gone func(seq int) bool) []CallSite {
	return live(x.callers[id], gone)
}

// Makers are the make_closure instructions that name the function with the ID.
func (x *SiteIndex) Makers(id FuncID, gone func(seq int) bool) []CallSite {
	return live(x.makers[id], gone)
}

func live(sites []CallSite, gone func(seq int) bool) []CallSite {
	if gone == nil {
		return sites
	}
	for i, s := range sites {
		if gone(int(s.Seq)) {
			out := append([]CallSite(nil), sites[:i]...)
			for _, t := range sites[i+1:] {
				if !gone(int(t.Seq)) {
					out = append(out, t)
				}
			}
			return out
		}
	}
	return sites
}

// Sited is a Source that knows where the functions are called and where the closures are made without reading the
// functions (SiteIndex).
type Sited interface {
	// CallersOf are the calls that reach the function with the ID, in the order of the program.
	CallersOf(id FuncID) []CallSite
	// MakersOf are the make_closure instructions that name the function with the ID.
	MakersOf(id FuncID) []CallSite
}

// Updater is a Source that does not keep what a reader writes to a function it has given (the candidates that valuecalls
// sets on a call): the reader tells it, and it gives the function with that when it reads the function again.
type Updater interface {
	// Update says that the call at the block and the instruction of the function now has this Callee. The Callee is
	// copied.
	Update(ref Ref, block, instr int, c *Callee)
}

// Update tells the Source that the call at the block and the instruction of the function now has the Callee that the
// function gives it. A Source that keeps its functions in memory has what was written: nothing to do.
func Update(src Source, ref Ref, f *Func, block, instr int) {
	if u, ok := src.(Updater); ok {
		u.Update(ref, block, instr, f.Blocks[block].Instrs[instr].Callee)
	}
}
