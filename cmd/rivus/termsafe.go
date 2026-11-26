package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// sanitizeForTerminal is what a child (a build script, cargo, an extractor) wrote, made safe to show on the terminal of the user (SBX-4,
// docs/DESIGN-macos-sandbox.md): every byte that is not shown as text becomes `\xNN` (the byte in 2 hexadecimal digits), so that no control sequence
// of the terminal is made. The ones changed: the C0 controls (ESC, CR, NUL, ...), DEL, the C1 controls (U+0080 to U+009F, 8-bit CSI and
// OSC among them; as the 2 bytes of their UTF-8 form), and a byte that is not valid UTF-8. LF and tab stay (a log needs them), and so does every other
// valid text, except the characters that change the order in which text is shown or hide it (below). It is the one function that the output of a child goes through: sandboxResult (macOS and Linux), tailOf, the extractor on the host.
//
// Two more kinds are made visible, as `\uXXXX` (the code point in 4 hexadecimal digits). They are not escape sequences of the terminal, and what they can do is smaller: they
// make an error message look like another one ("Trojan Source"). The bidirectional controls (U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069) reverse the order in
// which the text after them is shown, and the zero-width characters (U+200B to U+200D, U+FEFF) take up no room, so that a word can look like another one. A
// zero-width joiner inside of an emoji sequence is shown as `\u200d` too: the cost of not letting it hide anything.
func sanitizeForTerminal(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for i := 0; i < len(b); {
		r, n := utf8.DecodeRune(b[i:])
		switch {
		case r == utf8.RuneError && n <= 1: // not valid UTF-8 (a real U+FFFD is 3 bytes)
			fmt.Fprintf(&sb, `\x%02x`, b[i])
			n = 1
		case r == '\n' || r == '\t':
			sb.WriteRune(r)
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			for _, c := range b[i : i+n] {
				fmt.Fprintf(&sb, `\x%02x`, c)
			}
		case isInvisibleFormat(r):
			fmt.Fprintf(&sb, `\u%04x`, r)
		default:
			sb.Write(b[i : i+n])
		}
		i += n
	}
	return sb.String()
}

// isInvisibleFormat says whether r is a bidirectional control or a zero-width character (see sanitizeForTerminal).
func isInvisibleFormat(r rune) bool {
	return (r >= 0x200b && r <= 0x200f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) || r == 0xfeff
}
