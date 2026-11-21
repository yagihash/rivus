//go:build darwin || linux || freebsd

package taint

import (
	"time"

	"golang.org/x/sys/unix"
)

// threadCPU is the CPU time that the calling OS thread has used (CLOCK_THREAD_CPUTIME_ID: one
// vDSO or system call, about 100 ns to 1 us). It is read only for the `-v` column of the slow
// functions, on a goroutine that has locked its thread (runtime.LockOSThread), so the difference of two
// reads is the CPU of that goroutine: its own work and the allocation and collector assists it was made
// to do, not the background collector's workers and not the other workers' work. The second result is
// false when the system has no such clock.
func threadCPU() (time.Duration, bool) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &ts); err != nil {
		return 0, false
	}
	return time.Duration(ts.Nano()), true
}
