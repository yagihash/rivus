// Package char_base64 is a cell of the zoo.
//
// @cell(F/conv/char-base64)
// @shape: a home-made base64 encoder (three bytes make four symbols) and decoder, with padding
// @property: text carried through per-character reads and writes reaches the sink
package char_base64

import (
	"strings"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-base64/vuln", vuln)
	mark.Case("F/conv/char-base64/fixed", fixed)
	mark.Case("F/conv/char-base64/decoy", decoy)
}

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func encode(s string) string {
	var out []byte
	for i := 0; i < len(s); i += 3 {
		var n uint32
		k := 0
		for j := 0; j < 3; j++ {
			n <<= 8
			if i+j < len(s) {
				n |= uint32(s[i+j])
				k++
			}
		}
		out = append(out, alphabet[n>>18&63], alphabet[n>>12&63])
		if k > 1 {
			out = append(out, alphabet[n>>6&63])
		} else {
			out = append(out, '=')
		}
		if k > 2 {
			out = append(out, alphabet[n&63])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}

func decode(s string) string {
	var out []byte
	for i := 0; i+3 < len(s); i += 4 {
		var n uint32
		pad := 0
		for j := 0; j < 4; j++ {
			c := s[i+j]
			n <<= 6
			if c == '=' {
				pad++
			} else {
				n |= uint32(strings.IndexByte(alphabet, c))
			}
		}
		out = append(out, byte(n>>16))
		if pad < 2 {
			out = append(out, byte(n>>8))
		}
		if pad < 1 {
			out = append(out, byte(n))
		}
	}
	return string(out)
}

func f(s string) string { return decode(encode(s)) }

func vuln() { // @vuln
	v := mark.Src("conv-char-base64-vuln") // @source(conv-char-base64-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                     // @sink(conv-char-base64-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-base64-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-base64-decoy") // @source(conv-char-base64-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))             // @sink(none) @nosink(conv-char-base64-decoy)
	mark.SinkSQL(f(a))                      // @sink(conv-char-base64-decoy)
}
