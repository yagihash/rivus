package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const hostile = "\x1b]0;owned\x07\x1b[2J name‮"

// The text that the project decides (a file name that a build script touched, a name in Cargo.lock, a path, the message of a package that
// does not load) must not reach the terminal as it is (SBX-4): an escape sequence there can change the title, clear the screen or hide a line.
func TestErrorLineHasNoControlCharacter(t *testing.T) {
	got := errorLine(fmt.Errorf("Cargo.lock has dependencies that do not come from crates.io. Not allowed:\n  %s 1.0: git+https://x/y#abc", hostile))
	for _, bad := range []string{"\x1b", "\x07", "‮"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q is in the line: %q", bad, got)
		}
	}
	if !strings.HasPrefix(got, "rivus: ") || !strings.HasSuffix(got, "\n") {
		t.Errorf("the form of the line: %q", got)
	}
	if strings.Count(got, "\n") != 2 {
		t.Errorf("the newlines of the message are kept (two): %q", got)
	}
	// an error that is wrapped says the same
	if w := errorLine(fmt.Errorf("stage: %w", errors.New(hostile))); strings.Contains(w, "\x1b") {
		t.Errorf("wrapped: %q", w)
	}
}

func TestSayAttributeChangesHasNoControlCharacter(t *testing.T) {
	var sb strings.Builder
	sayAttributeChanges(&sb, []string{"chmod /work/" + hostile, "chown /p/ok"})
	got := sb.String()
	if strings.ContainsAny(got, "\x1b\x07") || strings.Contains(got, "‮") {
		t.Errorf("a control character reaches the terminal: %q", got)
	}
	if !strings.Contains(got, "chown /p/ok") || !strings.Contains(got, "the sandbox refused") {
		t.Errorf("what is said: %q", got)
	}
}

func TestSayWarningsHasNoControlCharacter(t *testing.T) {
	var sb strings.Builder
	sayWarnings(&sb, []string{"ok", "a " + hostile})
	got := sb.String()
	if strings.ContainsAny(got, "\x1b\x07") || strings.Contains(got, "‮") {
		t.Errorf("a control character reaches the terminal: %q", got)
	}
	if strings.Count(got, "rivus: warning: ") != 2 {
		t.Errorf("one line for each: %q", got)
	}
}
