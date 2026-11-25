package main

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The first line of `rivus version` is the one that a script (and the test of a formula) can read: "rivus <version> ... <os>/<arch> <go>". The lines
// after it are the version of the schema and the extractors (ADR-126).
func TestVersionIsOneLine(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"version", "--version"} {
		lines := strings.Split(strings.TrimSpace(string(rivus(t, arg))), "\n")
		first := lines[0]
		if !strings.HasPrefix(first, "rivus ") || !strings.Contains(first, "/") {
			t.Errorf("rivus %s: first line %q: want rivus <version> ... <os>/<arch> <go>", arg, first)
		}
		if len(lines) != 4 || lines[1] != "ir schema "+ir.SchemaVersion || !strings.HasPrefix(lines[2], "rust extractor") || !strings.HasPrefix(lines[3], "dart extractor") {
			t.Errorf("rivus %s: want the schema and one line for each extractor after the first line, got %q", arg, lines)
		}
	}
	old := versionStamp
	defer func() { versionStamp = old }()
	versionStamp = "v1.2.3"
	if got := versionString(); !strings.HasPrefix(got, "rivus 1.2.3 ") {
		t.Errorf("a release build: %q, want it to start with \"rivus 1.2.3 \"", got)
	}
}

// A formula tests the install with `rivus help`: it exits 0 and lists the commands.
func TestHelpExitsZeroAndListsTheCommands(t *testing.T) {
	t.Parallel()
	out := string(rivus(t, "help"))
	for _, c := range []string{"analyze", "index", "graph", "version"} {
		if !strings.Contains(out, c) {
			t.Errorf("rivus help does not mention %q:\n%s", c, out)
		}
	}
}
