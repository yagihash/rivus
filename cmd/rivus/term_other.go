//go:build !darwin

package main

import "fmt"

// there is no question where there is no sandbox: the standard input is never taken for a terminal
func stdinIsTerminal() bool { return false }

func sandboxUsable() error { return fmt.Errorf("no sandbox on this system") }
