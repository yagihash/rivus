package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// A flag that does not exist is a bad flag (exit code 2, as main.go says and as `rivus analyze -bogus` gives), also for the commands that
// parse their own flags: before, they gave 1, the code of a failure at run time, and the message was said twice.
func TestBadFlagExitCodeIsUsage(t *testing.T) {
	for _, args := range [][]string{
		{"cache", "list", "-bogus"},
		{"cache", "clean", "-bogus"},
		{"setup", "rust", "-bogus"},
		{"setup", "dart", "-bogus"},
		{"version", "-bogus"},
	} {
		cmd := exec.Command(binary, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != exitUsage {
			t.Errorf("rivus %v: %v, want exit code %d", args, err, exitUsage)
		}
		if n := strings.Count(stderr.String(), "flag provided but not defined"); n != 1 {
			t.Errorf("rivus %v: the message is said %d times, want once:\n%s", args, n, stderr.String())
		}
	}
	// -h is not an error
	if err := exec.Command(binary, "cache", "list", "-h").Run(); err != nil {
		t.Errorf("rivus cache list -h: %v", err)
	}
}
