//go:build !(darwin || linux || freebsd)

package taint

import "time"

// threadCPU: no per-thread CPU clock on this system; the `-v` line has no CPU column.
func threadCPU() (time.Duration, bool) { return 0, false }
