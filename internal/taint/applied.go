package taint

import (
	"math/bits"
	"sync"

	"github.com/yagihash/rivus/internal/sg"
)

// appliedSet is the candidates of one call (by their place in the list of candidates of the call) that the
// analysis applied: a set, so that the number does not depend on how many times a function was analysed again
// or in which order the workers came to the call.
type appliedSet struct {
	mu   sync.Mutex
	bits []uint64
}

// noteApplied records that the summary (or the model) of candidate i of n of the call ci was applied.
// Only the calls that go through an interface, a trait or a function value are recorded (ADR-127,
// summary.dispatch_candidates_applied).
// Invariant: the number of candidates of a call is fixed before Run(); the candidates (function values) that a
// call gains while the iteration runs are not counted.
func (e *Engine) noteApplied(ci *sg.Instr, i, n int) {
	v, ok := e.applied.Load(ci)
	if !ok {
		v, _ = e.applied.LoadOrStore(ci, &appliedSet{bits: make([]uint64, (n+63)/64)})
	}
	a := v.(*appliedSet)
	a.mu.Lock()
	// the list of a call does not change during a run (ci.Candidates and Callee.ValueCands are set before it), but
	// the set grows to the place instead of leaving a candidate out, whatever n was when it was made
	for w := i / 64; w >= len(a.bits); {
		a.bits = append(a.bits, 0)
	}
	a.bits[i/64] |= 1 << (uint(i) % 64)
	a.mu.Unlock()
}

// dispatchApplied is the number of candidates that the analysis applied, over the calls through an interface, a
// trait or a function value, each (call, candidate) once. A candidate that was not applied is not counted: the
// functions of a dependency of a call that has a model (the model says what they do), a candidate with no body,
// a call that no function reached. It is not the number of candidates in the IR.
func (e *Engine) dispatchApplied() int {
	total := 0
	e.applied.Range(func(_, v any) bool {
		a := v.(*appliedSet)
		a.mu.Lock()
		for _, w := range a.bits {
			total += bits.OnesCount64(w)
		}
		a.mu.Unlock()
		return true
	})
	return total
}
