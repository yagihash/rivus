//go:build darwin || linux || freebsd

package taint

import (
	"runtime"
	"testing"
	"time"
)

// The per-thread CPU clock moves with work and not with sleeping.
func TestThreadCPU(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c0, ok := threadCPU()
	if !ok {
		t.Skip("no per-thread CPU clock")
	}
	time.Sleep(50 * time.Millisecond)
	c1, _ := threadCPU()
	if c1-c0 > 20*time.Millisecond {
		t.Errorf("sleeping used %v of CPU", c1-c0)
	}
	// Spin until the thread's own clock has moved by 10 ms. The wall time that takes depends on the
	// load of the machine, so the wall limit is only a guard against a clock that never moves.
	x := 0
	var c2 time.Duration
	for end := time.Now().Add(60 * time.Second); time.Now().Before(end); x++ {
		c2, _ = threadCPU()
		if c2-c1 >= 10*time.Millisecond {
			break
		}
	}
	if c2-c1 < 10*time.Millisecond {
		t.Errorf("the thread CPU clock did not move by 10 ms in 60 s of work (%v, %d)", c2-c1, x)
	}
}
