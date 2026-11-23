// Package char_hex is a cell of the zoo.
//
// @cell(F/conv/char-hex)
// @shape: a home-made hex encoder (two table lookups per byte, s[i]>>4 and s[i]&15) and decoder (two digits make a byte)
// @property: text carried through per-character reads and writes reaches the sink
package char_hex

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-hex/vuln", vuln)
	mark.Case("F/conv/char-hex/fixed", fixed)
	mark.Case("F/conv/char-hex/decoy", decoy)
}

const digits = "0123456789abcdef"

func encode(s string) string {
	out := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		out = append(out, digits[c>>4], digits[c&15])
	}
	return string(out)
}

func unhex(c byte) byte {
	if c >= 'a' {
		return c - 'a' + 10
	}
	return c - '0'
}

func decode(h string) string {
	out := make([]byte, 0, len(h)/2)
	for i := 0; i+1 < len(h); i += 2 {
		out = append(out, unhex(h[i])<<4|unhex(h[i+1]))
	}
	return string(out)
}

func f(s string) string { return decode(encode(s)) }

func vuln() { // @vuln
	v := mark.Src("conv-char-hex-vuln") // @source(conv-char-hex-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                  // @sink(conv-char-hex-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-hex-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-hex-decoy") // @source(conv-char-hex-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))          // @sink(none) @nosink(conv-char-hex-decoy)
	mark.SinkSQL(f(a))                   // @sink(conv-char-hex-decoy)
}
