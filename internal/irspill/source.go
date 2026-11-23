package irspill

import (
	"container/list"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/yagihash/rivus/internal/ir"
)

// DefaultCache is the size, in bytes of the functions as they are in memory, that the functions read lately may take.
const DefaultCache = 64 << 20

// decodedPerRecord is how many bytes a function takes in memory for each byte of its record (measured on the IR of Go
// code: the record is a small part of the structs it is read into). It is what the cache counts with.
const decodedPerRecord = 8

// SourceOptions are the choices of a DiskSource.
type SourceOptions struct {
	// Pin says which functions a reader writes to (the candidates of the calls of function values are not in the
	// record: valuecalls sets them on the function it has read). Such a function is read once and held in memory for
	// the run, so that what was written to it is what the next reader sees, as it is in memory. nil: none.
	Pin func(*ir.Func) bool
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
	Pinned    int           // the functions held in memory because a reader writes to them (while they were not dropped)
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
	pin    []bool
	reach  [][]ir.FuncID
	intern map[ir.FuncID]ir.FuncID
	put    time.Duration
	slot   []int // the number in the store of the function at each place (nil: the same number)
}

// NewBuilder makes a Builder that writes to the store.
func NewBuilder(st *Store, opt SourceOptions) *Builder {
	return &Builder{st: st, opt: opt, intern: map[ir.FuncID]ir.FuncID{}}
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
	b.pin = append(b.pin, b.opt.Pin != nil && b.opt.Pin(f))
	var ids []ir.FuncID
	ir.Targets(f, func(id ir.FuncID) {
		if c, ok := b.intern[id]; ok {
			id = c
		} else {
			b.intern[id] = id
		}
		ids = append(ids, id)
	})
	b.reach = append(b.reach, ids)
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
	heads, pin, reach := make([]ir.Head, n), make([]bool, n), make([][]ir.FuncID, n)
	for to, from := range order {
		heads[to], pin[to], reach[to] = b.heads[from], b.pin[from], b.reach[from]
	}
	b.heads, b.pin, b.reach, b.slot = heads, pin, reach, order
}

// Len is the number of functions added.
func (b *Builder) Len() int { return len(b.heads) }

// Source is the Source of the functions that were added. hdr is the program without its functions (the types, the
// globals, what was not read). The Builder is not used after it.
func (b *Builder) Source(hdr *ir.Program) *DiskSource {
	limit := b.opt.Cache
	if limit == 0 {
		limit = DefaultCache
	}
	b.intern = nil
	return &DiskSource{st: b.st, hdr: hdr, heads: b.heads, pin: b.pin, reach: b.reach, slot: b.slot, putTime: b.put,
		limit: limit, fault: b.opt.Fault, pinned: map[int]*ir.Func{}, cached: map[int]*list.Element{}, lru: list.New()}
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
	b.pin = append(b.pin, false)
	b.reach = append(b.reach, nil)
}

// DiskSource is an ir.Source whose functions are in a Store. It is safe for several goroutines.
type DiskSource struct {
	st      *Store
	hdr     *ir.Program
	heads   []ir.Head
	pin     []bool
	reach   [][]ir.FuncID
	slot    []int // the number in the store of the function at each place (nil: the same number)
	fault   func(read int) error
	nfault  int
	putTime time.Duration

	mu     sync.Mutex
	last   map[ir.FuncID]int
	pinned map[int]*ir.Func
	cached map[int]*list.Element
	lru    *list.List // of *cacheEntry, the latest first
	used   int
	limit  int
	stats  SourceStats
	closed bool
}

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
	if f, ok := d.pinned[ref.Seq]; ok {
		d.mu.Unlock()
		return f, nil
	}
	if el, ok := d.cached[ref.Seq]; ok {
		d.lru.MoveToFront(el)
		d.stats.CacheHits++
		f := el.Value.(*cacheEntry).f
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
	if d.pin[ref.Seq] {
		if g, ok := d.pinned[ref.Seq]; ok {
			return g, nil // another goroutine was first: there is one copy that is written to
		}
		d.pinned[ref.Seq] = f
		d.stats.Pinned++
		return f, nil
	}
	if keep && d.limit > 0 {
		w := d.st.Size(d.slotOf(ref.Seq)) * decodedPerRecord
		if w == 0 {
			w = 1 << 10
		}
		d.cached[ref.Seq] = d.lru.PushFront(&cacheEntry{ref.Seq, f, w})
		d.used += w
		for d.used > d.limit && d.lru.Len() > 1 {
			el := d.lru.Back()
			e := el.Value.(*cacheEntry)
			d.lru.Remove(el)
			delete(d.cached, e.seq)
			d.used -= e.w
		}
	}
	return f, nil
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
	delete(d.pinned, ref.Seq)
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
	d.pinned, d.cached, d.last = nil, nil, nil
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
