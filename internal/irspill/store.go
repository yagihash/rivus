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

// Stats is what a store did.
type Stats struct {
	Funcs    int    // the functions in the store
	Bytes    int64  // the bytes in the records written (the file, and what waits to be written)
	MemFuncs int    // the functions that the store holds in memory, because it could not write them
	Fallback string // why (empty when every function was written)
}

// Store holds ir.Funcs in a file that has no name, and gives each back by its ID. A function that cannot be written
// (the limit of the size, a failed write, no file) is held in memory: Put and Get do not tell the difference, and the
// results are the same. A Get that fails (a record that does not read back) is an error that the caller must stop on:
// the IR is the main input of the analysis.
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
	flushed int64 // bytes in the file
	index   map[ir.FuncID]loc
	order   []ir.FuncID
	mem     map[ir.FuncID]*ir.Func
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
	s := &Store{index: map[ir.FuncID]loc{}, mem: map[ir.FuncID]*ir.Func{}, warn: warn}
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

// Put adds the function. A function with the ID of one that is in it replaces it.
func (s *Store) Put(f *ir.Func) {
	rec := Encode(nil, f)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.index[f.ID]; !ok {
		if _, ok := s.mem[f.ID]; !ok {
			s.order = append(s.order, f.ID)
		}
	}
	delete(s.index, f.ID)
	delete(s.mem, f.ID)
	if s.f == nil || s.why != "" {
		s.mem[f.ID] = f
		return
	}
	if s.flushed+int64(len(s.buf))+int64(len(rec)) > s.max {
		s.giveUp(fmt.Sprintf("the limit of %d bytes is reached", s.max))
		s.mem[f.ID] = f
		return
	}
	s.index[f.ID] = loc{off: s.flushed + int64(len(s.buf)), n: int32(len(rec))}
	s.buf = append(s.buf, rec...)
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
			s.buf = s.buf[:0]
			return
		} else {
			s.broken = true
			s.giveUp(fmt.Sprintf("write: %v", err))
		}
	}
	// the records that wait go to memory
	for off := 0; off < len(s.buf); {
		fn, n, err := Decode(s.buf[off:])
		if err != nil {
			// cannot happen: the buffer holds what Encode made
			panic("irspill: " + err.Error())
		}
		delete(s.index, fn.ID)
		s.mem[fn.ID] = fn
		off += n
	}
	s.buf = s.buf[:0]
}

// Get reads the function back. Each call gives a new copy.
func (s *Store) Get(id ir.FuncID) (*ir.Func, error) {
	s.mu.Lock()
	if f, ok := s.mem[id]; ok {
		s.mu.Unlock()
		return f, nil
	}
	l, ok := s.index[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if l.off+int64(l.n) > s.flushed {
		s.flush()
		if f, ok := s.mem[id]; ok {
			s.mu.Unlock()
			return f, nil
		}
	}
	file := s.f
	s.mu.Unlock()
	if file == nil {
		return nil, fmt.Errorf("irspill: reading %s: the store is closed", id)
	}
	rec := make([]byte, l.n)
	if _, err := file.ReadAt(rec, l.off); err != nil {
		return nil, fmt.Errorf("irspill: reading %s: %w", id, err)
	}
	f, _, err := Decode(rec)
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
	_, a := s.index[id]
	_, b := s.mem[id]
	return a || b
}

// IDs are the IDs in the order they were put.
func (s *Store) IDs() []ir.FuncID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ir.FuncID(nil), s.order...)
}

// Each reads the functions in the order they were put, one at a time.
func (s *Store) Each(fn func(*ir.Func) error) error {
	for _, id := range s.IDs() {
		f, err := s.Get(id)
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
	return Stats{Funcs: len(s.order), Bytes: s.flushed + int64(len(s.buf)), MemFuncs: len(s.mem), Fallback: s.why}
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
