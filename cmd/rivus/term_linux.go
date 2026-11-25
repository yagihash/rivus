//go:build linux

package main

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// stdinIsTerminal says whether the standard input is a terminal: one that can be asked. A device that is not a terminal
// (/dev/null) is not (a check of the mode of the file would say it is). A test binary is never asked.
func stdinIsTerminal() bool {
	if testing.Testing() {
		return false
	}
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return err == nil
}

func sandboxUsable() error { return sandboxAvailable() }
