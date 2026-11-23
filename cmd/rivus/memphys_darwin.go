//go:build darwin

package main

import "golang.org/x/sys/unix"

// physicalMemory is the memory of the machine in bytes.
func physicalMemory() (int64, error) {
	n, err := unix.SysctlUint64("hw.memsize")
	return int64(n), err
}
