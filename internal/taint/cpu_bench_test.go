//go:build darwin || linux || freebsd

package taint

import (
	"runtime"
	"testing"
)

// BenchmarkThreadCPULock is what compute adds to every function under -v: one lock and unlock of the
// thread and one clock read at the start (and one more for a slow function).
func BenchmarkThreadCPULock(b *testing.B) {
	for i := 0; i < b.N; i++ {
		runtime.LockOSThread()
		threadCPU()
		runtime.UnlockOSThread()
	}
}
