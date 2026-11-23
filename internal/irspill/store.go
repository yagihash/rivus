package irspill

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/yagihash/rivus/internal/ir"
)

// DefaultMax is the most bytes a store writes when the caller gives no other limit. The limit is cut to half of
// the free space of the directory when that is less, so that the disk does not fill up.
const DefaultMax = 8 << 30

// flushAt is the size of the buffer that the records wait in before they are written.
const flushAt = 1 << 20

// ErrNotFound is the error of Get for an ID that was not put.
var ErrNotFound = errors.New("irspill: no such function")

type loc struct {
	off int64
	n   int32 // the bytes of the whole record
}

// rec is what the store knows of one function it was given: where its record is (or the function itself, when it
// could not be written).
type rec struct {
	id  ir.FuncID
	loc loc
	mem *ir.Func
}

// Stats is what a store did.
type Stats struct {
	Funcs    int    // the functions in the store
	Bytes    int64  // the bytes in the records written (the file, and what waits to be written)
	MemFuncs int    // the functions that the store holds in memory, because it could not write them
	Fallback string // why (empty when every function was written)
}

// Store holds ir.Funcs in a file that has no name. Add gives each function a number (the order it came in), and At
// reads it back by that number: a number is a place, and two functions with one ID have two places (the IR can have
// several functions with one ID). Put and Get are the same by ID, where a function with the ID of one that is in it
// replaces it. A function that cannot be written (the limit of the size, a failed write, no file) is held in memory:
// the callers do not see the difference, and the results are the same. A read that fails (a record that does not read
// back) is an error that the caller must stop on: the IR is the main input of the analysis.
//
// The file is made in the temporary directory ($TMPDIR) with the mode 0600 and removed at once, so that nothing is
// left when the process is killed, and nothing else can open it by its name. It is not mapped (a full disk would
// give SIGBUS). What it holds is what the IR holds: names, positions, and the constants of the strings of the code.
// It is read by this package only, in the same run.
type Store struct {
	mu      sync.Mutex
	f       *os.File
	max     int64
	buf     []byte
	pend    []int // the numbers of the records in buf, in order
	flushed int64 // bytes in the file
	recs    []rec
	byID    map[ir.FuncID]int // the last number of each ID
	nmem    int
	why     string
	broken  bool // a write failed: the buffer goes to memory
	warn    io.Writer
}

// New makes a store in the temporary directory. max is the most bytes it writes (0: DefaultMax). warn gets the
// line that says that the store holds functions in memory (nil: none). It never fails: without a file, every
// function is held in memory.
func New(max int64, warn io.Writer) *Store { return NewIn(os.TempDir(), max, warn) }

// NewIn is New for a directory.
func NewIn(dir string, max int64, warn io.Writer) *Store {
	s := &Store{byID: map[ir.FuncID]int{}, warn: warn}
	if max <= 0 {
		max = DefaultMax
	}
	if free := freeSpace(dir); free >= 0 && free/2 < max {
		max = free / 2
	}
	s.max = max
	f, err := os.CreateTemp(dir, "rivus-ir-*")
	if err != nil {
		s.giveUp(fmt.Sprintf("no file: %v", err))
		return s
	}
	// the name goes at once: a process that dies leaves nothing, and nobody opens it by its name
	if err := os.Remove(f.Name()); err != nil {
		f.Close()
		s.giveUp(fmt.Sprintf("the file could not be unlinked: %v", err))
		return s
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		s.giveUp(fmt.Sprintf("the file mode: %v", err))
		return s
	}
	s.f = f
	return s
}

// giveUp stops the writing, with the reason (the first one stays), and says it once.
func (s *Store) giveUp(why string) {
	if s.why != "" {
		return
	}
	s.why = why
	if s.warn != nil {
		fmt.Fprintf(s.warn, "rivus: the IR is held in memory, not in a file (%s)\n", why)
	}
}

// Put adds the function. A function with the ID of one that is in it replaces it (in its place).
func (s *Store) Put(f *ir.Func) {
	rec := Encode(nil, f)
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.byID[f.ID]; ok {
		s.set(n, f, rec)
		return
	}
	s.add(f, rec)
}

// Add adds the function as a new one, whatever its ID, and says its number. The first number is 0.
func (s *Store) Add(f *ir.Func) int {
	rec := Encode(nil, f)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.add(f, rec)
}

func (s *Store) add(f *ir.Func, record []byte) int {
	n := len(s.recs)
	s.recs = append(s.recs, rec{id: f.ID})
	s.byID[f.ID] = n
	s.set(n, f, record)
	return n
}

// set puts the record of f at the number n. The caller holds the lock.
func (s *Store) set(n int, f *ir.Func, rec []byte) {
	r := &s.recs[n]
	if r.mem != nil {
		s.nmem--
	}
	r.mem, r.loc = nil, loc{}
	if s.f == nil || s.why != "" {
		r.mem = f
		s.nmem++
		return
	}
	if s.flushed+int64(len(s.buf))+int64(len(rec)) > s.max {
		s.giveUp(fmt.Sprintf("the limit of %d bytes is reached", s.max))
		r.mem = f
		s.nmem++
		return
	}
	r.loc = loc{off: s.flushed + int64(len(s.buf)), n: int32(len(rec))}
	s.buf = append(s.buf, rec...)
	s.pend = append(s.pend, n)
	if len(s.buf) >= flushAt {
		s.flush()
	}
}

// flush writes the buffer. When the write fails, the functions that were in it go to memory (the buffer is
// the records, so they are read back from it). The caller holds the lock.
func (s *Store) flush() {
	if len(s.buf) == 0 {
		return
	}
	if s.f != nil && !s.broken {
		if _, err := s.f.WriteAt(s.buf, s.flushed); err == nil {
			s.flushed += int64(len(s.buf))
			s.buf, s.pend = s.buf[:0], s.pend[:0]
			return
		} else {
			s.broken = true
			s.giveUp(fmt.Sprintf("write: %v", err))
		}
	}
	// the records that wait go to memory
	off := 0
	for _, n := range s.pend {
		fn, k, err := Decode(s.buf[off:])
		if err != nil {
			// cannot happen: the buffer holds what Encode made
			panic("irspill: " + err.Error())
		}
		// a record that was replaced after it was put does not wait here any more: its place has the new one
		if s.recs[n].loc.off == s.flushed+int64(off) && s.recs[n].mem == nil {
			s.recs[n].mem, s.recs[n].loc = fn, loc{}
			s.nmem++
		}
		off += k
	}
	s.buf, s.pend = s.buf[:0], s.pend[:0]
}

// Get reads the function back. Each call gives a new copy.
func (s *Store) Get(id ir.FuncID) (*ir.Func, error) {
	s.mu.Lock()
	n, ok := s.byID[id]
	s.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	return s.At(n, id)
}

// At reads the function that has the number n back (each call gives a new copy), and checks that it is the function
// id. A number that was not given, and a record that is of another function, are errors.
func (s *Store) At(n int, id ir.FuncID) (*ir.Func, error) {
	s.mu.Lock()
	if n < 0 || n >= len(s.recs) {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	r := s.recs[n]
	if r.mem != nil {
		s.mu.Unlock()
		if r.mem.ID != id {
			return nil, fmt.Errorf("irspill: %s: %w: the place holds %s", id, ErrCorrupt, r.mem.ID)
		}
		return r.mem, nil
	}
	if r.loc.off+int64(r.loc.n) > s.flushed {
		s.flush()
		if r = s.recs[n]; r.mem != nil {
			s.mu.Unlock()
			return r.mem, nil
		}
	}
	file := s.f
	s.mu.Unlock()
	if file == nil {
		return nil, fmt.Errorf("irspill: reading %s: the store is closed", id)
	}
	buf := make([]byte, r.loc.n)
	if _, err := file.ReadAt(buf, r.loc.off); err != nil {
		return nil, fmt.Errorf("irspill: reading %s: %w", id, err)
	}
	f, _, err := Decode(buf)
	if err != nil {
		return nil, fmt.Errorf("irspill: %s: %w", id, err)
	}
	if f.ID != id {
		return nil, fmt.Errorf("irspill: %s: %w: the record is of %s", id, ErrCorrupt, f.ID)
	}
	return f, nil
}

// Has says whether the function is in the store.
func (s *Store) Has(id ir.FuncID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.byID[id]
	return ok
}

// IDs are the IDs in the order they were put (one for each function: an ID that was added twice is twice here).
func (s *Store) IDs() []ir.FuncID {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]ir.FuncID, len(s.recs))
	for i := range s.recs {
		ids[i] = s.recs[i].id
	}
	return ids
}

// Each reads the functions in the order they were put, one at a time.
func (s *Store) Each(fn func(*ir.Func) error) error {
	for n, id := range s.IDs() {
		f, err := s.At(n, id)
		if err != nil {
			return err
		}
		if err := fn(f); err != nil {
			return err
		}
	}
	return nil
}

// Stats says what the store holds.
func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{Funcs: len(s.recs), Bytes: s.flushed + int64(len(s.buf)), MemFuncs: s.nmem, Fallback: s.why}
}

// Close closes the file (which goes away with it). Get does not work after it, for the functions in the file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	s.buf = nil
	return err
}

// Size is the bytes of the record of the function that has the number n (0 when it is held in memory or not there).
func (s *Store) Size(n int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < 0 || n >= len(s.recs) {
		return 0
	}
	return int(s.recs[n].loc.n)
}
