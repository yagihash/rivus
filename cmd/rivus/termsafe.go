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
// valid text. It is the one function that the output of a child goes through: sandboxResult (macOS and Linux), tailOf, the extractor on the host.
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
		default:
			sb.Write(b[i : i+n])
		}
		i += n
	}
	return sb.String()
}
