package taint

import (
	"sync"
	"sync/atomic"
)

// strID is the number of a string in the table of interned strings. The same
// string always has the same number, so two of them are equal when their
// numbers are; the empty string is 0. The numbers depend on which goroutine
// got there first, and nothing may use them for order.
type strID uint32

const (
	internChunk    = 1 << 12
	internMaxChunk = 1 << 18 // 2^30 strings
)

var interner struct {
	ids    sync.Map // string -> strID; read without a lock
	mu     sync.Mutex
	n      uint32 // next number
	chunks [internMaxChunk]atomic.Pointer[[internChunk]string]
}

func init() { interner.n = 1 } // 0 is ""

// intern returns the number of s.
func intern(s string) strID {
	if s == "" {
		return 0
	}
	if v, ok := interner.ids.Load(s); ok {
		return v.(strID)
	}
	interner.mu.Lock()
	defer interner.mu.Unlock()
	if v, ok := interner.ids.Load(s); ok {
		return v.(strID)
	}
	n := interner.n
	c := interner.chunks[n/internChunk].Load()
	if c == nil {
		c = new([internChunk]string)
		interner.chunks[n/internChunk].Store(c)
	}
	// written before the number is published through the map
	c[n%internChunk] = s
	interner.n++
	interner.ids.Store(s, strID(n))
	return strID(n)
}

// str is the string of a number.
func (i strID) str() string {
	if i == 0 {
		return ""
	}
	return interner.chunks[i/internChunk].Load()[i%internChunk]
}

// InternedStrings is the number of distinct strings in the table (for the
// report of the phases).
func InternedStrings() int {
	interner.mu.Lock()
	defer interner.mu.Unlock()
	return int(interner.n)
}
