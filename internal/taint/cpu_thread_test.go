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
	x := 0
	for end := time.Now().Add(30 * time.Millisecond); time.Now().Before(end); {
		x++
	}
	c2, _ := threadCPU()
	if c2-c1 < 10*time.Millisecond {
		t.Errorf("30 ms of work used %v of CPU (%d)", c2-c1, x)
	}
}
