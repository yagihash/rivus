package main

import (
	"testing"
	"time"
)

// A version command whose child keeps the output open (a shell wrapper that starts a process and ends) does not hold rivus until that
// process ends: after the command ended, the output is read for a second at most. Before, runVersion waited for the child (here 6 s; with a
// child that does not end, the minute of the time limit). The child of the test ends by itself.
func TestRunVersionDoesNotWaitForAChildThatHoldsTheOutput(t *testing.T) {
	start := time.Now()
	got := runVersion("/bin/sh", "-c", "sleep 6 & echo v1.2.3")
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("runVersion returned after %v: it waited for the child that holds the output", d)
	}
	if got != "v1.2.3" {
		t.Errorf("runVersion = %q, want the version that the command printed", got)
	}
}
