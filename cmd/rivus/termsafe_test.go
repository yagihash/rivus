package main

import "testing"

// SBX-4: what a child (a build script, cargo, an extractor) writes to its standard error is shown without a control character.
func TestSanitizeForTerminal(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"escape", "a\x1b[2Jb", `a\x1b[2Jb`},
		{"osc", "\x1b]52;c;AAAA\x07", `\x1b]52;c;AAAA\x07`},
		{"c1 csi", "a\u009bb", `a\xc2\x9bb`},
		{"c1 osc", "a\u009db", `a\xc2\x9db`},
		{"del", "a\x7fb", `a\x7fb`},
		{"cr", "a\rb", `a\x0db`},
		{"nul and bell and backspace", "\x00\x07\x08", `\x00\x07\x08`},
		{"invalid utf-8", "a\xff\xfeb\xc3", `a\xff\xfeb\xc3`},
		{"newline and tab stay", "a\n\tb\n", "a\n\tb\n"},
		{"ascii and japanese unchanged", "error: 失敗しました (exit 1)\n", "error: 失敗しました (exit 1)\n"},
		{"empty", "", ""},
	} {
		if got := sanitizeForTerminal([]byte(c.in)); got != c.want {
			t.Errorf("%s: %q -> %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestTailOfHasNoControlCharacter(t *testing.T) {
	if got := tailOf("boom \x1b]52;c;AAAA\x07"); got != `: boom \x1b]52;c;AAAA\x07` {
		t.Errorf("%q", got)
	}
}
