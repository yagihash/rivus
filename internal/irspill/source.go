package irspill

import (
	"container/list"
	"fmt"
	"sort"
	"sync"
	"time"
	"weak"

	"github.com/yagihash/rivus/internal/ir"
)

// DefaultCache is the size, in bytes of the functions as they are in memory, that the functions read lately may take.
const DefaultCache = 64 << 20

// decodedPerRecord is how many bytes a function takes in memory for each byte of its record (measured on the IR of Go
// code: the record is a small part of the structs it is read into). It is what the cache counts with.
const decodedPerRecord = 8

// SourceOptions are the choices of a DiskSource.
type SourceOptions struct {
	// Fault, when it is set, is called with the number of the reads so far (the first is 1) before each read of a
	// function from the store; the error it returns is the error of the read. It is for the tests of what the readers do
	// when a function cannot be read (the command: RIVUS_INTERNAL_IR_SPILL_FAULT).
	Fault func(read int) error
	// Cache is the most bytes the functions read lately may take in memory (0: DefaultCache; below 0: none).
	Cache int
}

// SourceStats is what a DiskSource did.
type SourceStats struct {
	Store     Stats
	Updates   int           // the calls whose candidates a reader wrote (Update): the table of the candidates that valuecalls set
	Reads     int           // the functions read and decoded (a function read twice is counted twice)
	CacheHits int           // the reads that the cache answered
	ReadTime  time.Duration // the time of the reads (the file, the decoding, the check of the record)
	PutTime   time.Duration // the time of Put (the encoding, the buffer, the writes)
}

// Builder makes a DiskSource: the extractor gives it the functions one at a time (Add), and it keeps what the
// readers ask of every function without the body (the head and the IDs of the functions it reaches) in memory and the
// function in the store. A function is not kept after Add.
type Builder struct {
	st     *Store
	opt    SourceOptions
	heads  []ir.Head
	marks  []ir.Marks
	syms   [][]string // the symbols of the calls of a function of the project
	reach  [][]ir.FuncID
	sites  *ir.SiteIndex
	intern map[ir.FuncID]ir.FuncID
	strs   map[string]string
	put    time.Duration
	slot   []int // the number in the store of the function at each place (nil: the same number)
}

// NewBuilder makes a Builder that writes to the store.
func NewBuilder(st *Store, opt SourceOptions) *Builder {
	return &Builder{st: st, opt: opt, intern: map[ir.FuncID]ir.FuncID{}, strs: map[string]string{}, sites: ir.NewSiteIndex()}
}

// Add puts the function into the store. Its number is the place it has in the order of the program.
func (b *Builder) Add(f *ir.Func) {
	begun := time.Now()
	n := b.st.Add(f)
	b.put += time.Since(begun)
	if b.slot != nil {
		panic("irspill: Add after SortByID")
	}
	if n != len(b.heads) {
		panic(fmt.Sprintf("irspill: the store gave the number %d for the function %d", n, len(b.heads)))
	}
	b.heads = append(b.heads, ir.Head{ID: f.ID, Dep: f.Dep, Symbol: f.Symbol, Recv: f.Recv})
	b.marks = append(b.marks, ir.MarksOf(f))
	var syms []string
	if !f.Dep {
		for _, s := range ir.CallSymbolsOf(f) {
			if c, ok := b.strs[s]; ok {
				s = c
			} else {
				b.strs[s] = s
			}
			syms = append(syms, s)
		}
	}
	b.syms = append(b.syms, syms)
	var ids []ir.FuncID
	ir.Targets(f, func(id ir.FuncID) {
		ids = append(ids, b.interned(id))
	})
	b.reach = append(b.reach, ids)
	b.sites.Add(n, f, b.interned)
}

// interned is the one string for the ID: the tables of the Builder do not keep the strings of the functions.
func (b *Builder) interned(id ir.FuncID) ir.FuncID {
	if c, ok := b.intern[id]; ok {
		return c
	}
	b.intern[id] = id
	return id
}

// SortByID puts the functions in the order of their IDs (functions with one ID stay in the order they came in), the
// order that the Go extractor gives its program. Call it after the last Add.
func (b *Builder) SortByID() {
	n := len(b.heads)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return b.heads[order[i]].ID < b.heads[order[j]].ID })
	heads, marks, syms, reach := make([]ir.Head, n), make([]ir.Marks, n), make([][]string, n), make([][]ir.FuncID, n)
	to := make([]int, n)
	for at, from := range order {
		heads[at], marks[at], syms[at], reach[at] = b.heads[from], b.marks[from], b.syms[from], b.reach[from]
		to[from] = at
	}
	b.sites.Remap(to)
	b.heads, b.marks, b.syms, b.reach, b.slot = heads, marks, syms, reach, order
}

// Discard lets go of what was written (the extractor failed: there is no Source).
func (b *Builder) Discard() { b.st.Close() }

// Len is the number of functions added.
func (b *Builder) Len() int { return len(b.heads) }

// Source is the Source of the functions that were added. hdr is the program without its functions (the types, the
// globals, what was not read). The Builder is not used after it.
func (b *Builder) Source(hdr *ir.Program) *DiskSource {
	limit := b.opt.Cache
	if limit == 0 {
		limit = DefaultCache
	}
	b.intern, b.strs = nil, nil
	return &DiskSource{st: b.st, hdr: hdr, heads: b.heads, marks: b.marks, syms: b.syms, reach: b.reach, sites: b.sites, slot: b.slot,
		putTime: b.put, limit: limit, fault: b.opt.Fault, cached: map[int]*list.Element{}, lru: list.New()}
}

// FromProgram puts the functions of a program into a store and takes them out of the program (its Funcs are nil
// after it): the program is then the header of the Source.
func FromProgram(st *Store, p *ir.Program, opt SourceOptions) *DiskSource {
	funcs := p.Funcs
	p.Funcs = nil
	b := NewBuilder(st, opt)
	for i, f := range funcs {
		if f == nil {
			// a function that was dropped keeps its place in the order of the program
			b.AddGone()
		} else {
			b.Add(f)
		}
		funcs[i] = nil
	}
	return b.Source(p)
}

// AddGone adds a place with no function: a program that had a function dropped keeps the order of the others.
func (b *Builder) AddGone() {
	b.st.Add(&ir.Func{})
	b.heads = append(b.heads, ir.Head{Gone: true})
	b.marks = append(b.marks, 0)
	b.syms = append(b.syms, nil)
	b.reach = append(b.reach, nil)
}

// DiskSource is an ir.Source whose functions are in a Store. It is safe for several goroutines.
type DiskSource struct {
	st      *Store
	hdr     *ir.Program
	heads   []ir.Head
	marks   []ir.Marks
	syms    [][]string
	reach   [][]ir.FuncID
	sites   *ir.SiteIndex
	slot    []int // the number in the store of the function at each place (nil: the same number)
	fault   func(read int) error
	nfault  int
	putTime time.Duration

	mu   sync.Mutex
	last map[ir.FuncID]int
	// table is what valuecalls wrote to the calls: a function that is read again has it (Update). It is the
	// candidates of the calls that have some, not the functions.
	table map[int]map[place]*ir.Callee
	// alive holds, for the functions that were read, the function as long as a reader has it: a second read gives the same
	// pointer, so that a reader that compares the functions it holds (valuecalls) does not see two for one.
	alive  []weak.Pointer[ir.Func]
	cached map[int]*list.Element
	lru    *list.List // of *cacheEntry, the latest first
	used   int
	limit  int
	stats  SourceStats
	closed bool
}

// place is a call in a function: the block and the instruction.
type place struct{ block, instr int32 }

type cacheEntry struct {
	seq int
	f   *ir.Func
	w   int
}

var _ ir.Source = (*DiskSource)(nil)

func (d *DiskSource) slotOf(seq int) int {
	if d.slot == nil {
		return seq
	}
	return d.slot[seq]
}

func (d *DiskSource) Header() *ir.Program { return d.hdr }

func (d *DiskSource) Len() int { return len(d.heads) }

func (d *DiskSource) Head(seq int) ir.Head { return d.heads[seq] }

// Targets are the IDs of the functions that the function at seq reaches (ir.Targets), kept from the time it was
// added, so that the question needs no read.
func (d *DiskSource) Targets(seq int) []ir.FuncID { return d.reach[seq] }

func (d *DiskSource) Lookup(id ir.FuncID) (ir.Ref, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.last == nil {
		d.last = make(map[ir.FuncID]int, len(d.heads))
		for i, h := range d.heads {
			if !h.Gone {
				d.last[h.ID] = i
			}
		}
	}
	i, ok := d.last[id]
	return ir.Ref{ID: id, Seq: i}, ok
}

func (d *DiskSource) Func(ref ir.Ref) (*ir.Func, error) { return d.read(ref, true) }

func (d *DiskSource) read(ref ir.Ref, keep bool) (*ir.Func, error) {
	if ref.Seq < 0 || ref.Seq >= len(d.heads) {
		return nil, fmt.Errorf("no function at number %d", ref.Seq)
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, fmt.Errorf("the source was released")
	}
	h := d.heads[ref.Seq]
	if h.Gone {
		d.mu.Unlock()
		return nil, fmt.Errorf("the function at number %d was dropped", ref.Seq)
	}
	if h.ID != ref.ID {
		d.mu.Unlock()
		return nil, fmt.Errorf("the function at number %d is %s", ref.Seq, h.ID)
	}
	if el, ok := d.cached[ref.Seq]; ok {
		d.lru.MoveToFront(el)
		d.stats.CacheHits++
		f := el.Value.(*cacheEntry).f
		d.mu.Unlock()
		return f, nil
	}
	if f := d.liveLocked(ref.Seq); f != nil {
		d.stats.CacheHits++
		if keep {
			d.keepLocked(ref.Seq, f)
		}
		d.mu.Unlock()
		return f, nil
	}
	d.mu.Unlock()

	begun := time.Now()
	var f *ir.Func
	var err error
	if d.fault != nil {
		d.mu.Lock()
		d.nfault++
		n := d.nfault
		d.mu.Unlock()
		err = d.fault(n)
	}
	if err == nil {
		f, err = d.st.At(d.slotOf(ref.Seq), ref.ID)
	}
	took := time.Since(begun)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stats.Reads++
	d.stats.ReadTime += took
	if keep {
		if g := d.liveLocked(ref.Seq); g != nil {
			return g, nil // another goroutine was first: there is one copy
		}
	}
	d.applyLocked(ref.Seq, f)
	if keep {
		d.keepLocked(ref.Seq, f)
	}
	return f, nil
}

// liveLocked is the function at seq that a reader still has (read by Func), or nil.
func (d *DiskSource) liveLocked(seq int) *ir.Func {
	if d.alive == nil || d.alive[seq] == (weak.Pointer[ir.Func]{}) {
		return nil
	}
	return d.alive[seq].Value()
}

// keepLocked puts the function into the cache (the latest read ones are kept up to the limit) and remembers it as the
// one that is given while a reader has it.
func (d *DiskSource) keepLocked(seq int, f *ir.Func) {
	if d.alive == nil {
		d.alive = make([]weak.Pointer[ir.Func], len(d.heads))
	}
	d.alive[seq] = weak.Make(f)
	if d.limit <= 0 {
		return
	}
	if el, ok := d.cached[seq]; ok {
		d.lru.MoveToFront(el)
		return
	}
	w := d.st.Size(d.slotOf(seq)) * decodedPerRecord
	if w == 0 {
		w = 1 << 10
	}
	d.cached[seq] = d.lru.PushFront(&cacheEntry{seq, f, w})
	d.used += w
	for d.used > d.limit && d.lru.Len() > 1 {
		el := d.lru.Back()
		e := el.Value.(*cacheEntry)
		d.lru.Remove(el)
		delete(d.cached, e.seq)
		d.used -= e.w
	}
}

// applyLocked gives the function what was written to its calls (Update).
func (d *DiskSource) applyLocked(seq int, f *ir.Func) {
	for at, c := range d.table[seq] {
		cp := *c
		f.Blocks[at.block].Instrs[at.instr].Callee = &cp
	}
}

// Update keeps what a reader wrote to a call: the candidates that valuecalls set. The function that is read again has
// them. A function that a reader has now (read by Func) is given them too.
func (d *DiskSource) Update(ref ir.Ref, block, instr int, c *ir.Callee) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || ref.Seq < 0 || ref.Seq >= len(d.heads) {
		return
	}
	if d.table == nil {
		d.table = map[int]map[place]*ir.Callee{}
	}
	t := d.table[ref.Seq]
	if t == nil {
		t = map[place]*ir.Callee{}
		d.table[ref.Seq] = t
	}
	at := place{int32(block), int32(instr)}
	if _, ok := t[at]; !ok {
		d.stats.Updates++
	}
	cp := *c
	t[at] = &cp
	if f := d.liveLocked(ref.Seq); f != nil && f.Blocks[block].Instrs[instr].Callee != c {
		cp2 := *c
		f.Blocks[block].Instrs[instr].Callee = &cp2
	}
}

// Marks are what is known of the function at seq (ir.Marker).
func (d *DiskSource) Marks(seq int) ir.Marks { return d.marks[seq] }

// CallSymbols are the symbols of the calls of the function at seq, when it is a function of the project (ir.CallSymbolser).
func (d *DiskSource) CallSymbols(seq int) []string { return d.syms[seq] }

func (d *DiskSource) gone(seq int) bool { return d.heads[seq].Gone }

// CallersOf are the calls that reach the function with the ID (ir.Sited).
func (d *DiskSource) CallersOf(id ir.FuncID) []ir.CallSite {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil // released: the functions are not there, and the index was let go of
	}
	return d.sites.Callers(id, d.gone)
}

// MakersOf are the make_closure instructions that name the function with the ID (ir.Sited).
func (d *DiskSource) MakersOf(id ir.FuncID) []ir.CallSite {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil // released: the functions are not there, and the index was let go of
	}
	return d.sites.Makers(id, d.gone)
}

func (d *DiskSource) Each(fn func(ref ir.Ref, f *ir.Func) error) error {
	for i := range d.heads {
		h := d.heads[i]
		if h.Gone {
			continue
		}
		ref := ir.Ref{ID: h.ID, Seq: i}
		f, err := d.read(ref, false)
		if err != nil {
			return &ir.ReadError{Ref: ref, Err: err}
		}
		if err := fn(ref, f); err != nil {
			return err
		}
	}
	return nil
}

func (d *DiskSource) Drop(ref ir.Ref) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if ref.Seq < 0 || ref.Seq >= len(d.heads) {
		return
	}
	d.heads[ref.Seq].Gone = true
	d.reach[ref.Seq] = nil
	delete(d.table, ref.Seq)
	d.syms[ref.Seq] = nil
	if el, ok := d.cached[ref.Seq]; ok {
		d.used -= el.Value.(*cacheEntry).w
		d.lru.Remove(el)
		delete(d.cached, ref.Seq)
	}
	if d.last != nil && d.last[ref.ID] == ref.Seq {
		delete(d.last, ref.ID)
	}
}

// Release lets go of the functions: what is in memory, and the file.
func (d *DiskSource) Release() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stats = d.statsLocked()
	d.closed = true
	d.table, d.alive, d.cached, d.last, d.sites = nil, nil, nil, nil, nil
	d.lru.Init()
	d.st.Close()
}

// Stats says what the source did. It is valid after Release.
func (d *DiskSource) Stats() SourceStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return d.stats
	}
	return d.statsLocked()
}

func (d *DiskSource) statsLocked() SourceStats {
	if d.closed {
		return d.stats
	}
	s := d.stats
	s.Store = d.st.Stats()
	s.PutTime = d.putTime
	return s
}
