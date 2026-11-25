//go:build linux && amd64

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// legacyAttrChecks: the old forms (utime, utimes, futimesat) exist on x86-64 only.
func legacyAttrChecks() {
	for _, c := range []struct {
		name string
		nr   uintptr
	}{{"utime", unix.SYS_UTIME}, {"utimes", unix.SYS_UTIMES}, {"futimesat", unix.SYS_FUTIMESAT}} {
		_, _, errno := unix.Syscall(c.nr, 0, 0, 0)
		report(c.name, errno != unix.EPERM, fmt.Sprint(errno))
	}
}
