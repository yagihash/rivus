package main

import (
	"strings"
	"testing"
)

// The flags of the container are gone; a command that is given one stops and says what it is now (nothing is run, and nothing is ignored).
func TestOldIsolationFlagsAreRefused(t *testing.T) {
	for old, now := range map[string]string{
		"-container-timeout": "-isolation-timeout", "--container-max-output=1": "-isolation-max-output", "-container-env": "-isolation-env",
		"-container-ro-mount": "-isolation-read", "-container-memory": "gone", "-container-cpus": "gone", "-container-pids": "gone", "-rust-image": "gone",
	} {
		err := oldIsolationFlag([]string{"-lang", "rust", old, "x"})
		if err == nil || !strings.Contains(err.Error(), now) {
			t.Errorf("%s: %v, want an error that says %q", old, err, now)
		}
	}
	if err := oldIsolationFlag([]string{"-lang", "rust", "-isolation-timeout", "1m", "-no-fetch"}); err != nil {
		t.Errorf("the new flags are not refused: %v", err)
	}
}
