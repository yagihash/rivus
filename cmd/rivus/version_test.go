package main

import (
	"strings"
	"testing"
)

// `rivus version` is one line that a script (and the test of a formula) can read: "rivus <version> ... <os>/<arch> <go>".
func TestVersionIsOneLine(t *testing.T) {
	for _, arg := range []string{"version", "--version"} {
		out := strings.TrimSpace(string(rivus(t, arg)))
		if strings.Contains(out, "\n") || !strings.HasPrefix(out, "rivus ") || !strings.Contains(out, "/") {
			t.Errorf("rivus %s = %q: want one line, rivus <version> ... <os>/<arch> <go>", arg, out)
		}
	}
	old := version
	defer func() { version = old }()
	version = "v1.2.3"
	if got := versionString(); !strings.HasPrefix(got, "rivus 1.2.3 ") {
		t.Errorf("a release build: %q, want it to start with \"rivus 1.2.3 \"", got)
	}
}
