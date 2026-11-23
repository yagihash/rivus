// Package char_decode_rune is a cell of the zoo.
//
// @cell(F/conv/char-decode-rune)
// @shape: a buffer type reads the text with utf8.DecodeRuneInString in a loop, appends byte(r) to a []byte field, and gives the text back by slicing the field
// @property: text carried through per-character reads and writes reaches the sink
package char_decode_rune

import (
	"unicode/utf8"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-decode-rune/vuln", vuln)
	mark.Case("F/conv/char-decode-rune/fixed", fixed)
	mark.Case("F/conv/char-decode-rune/decoy", decoy)
}

type buffer struct {
	arr []byte
}

func newBuffer(data string) *buffer {
	buf := make([]byte, 0, len(data))
	idx := 0
	for idx < len(data) {
		r, n := utf8.DecodeRuneInString(data[idx:])
		idx += n
		buf = append(buf, byte(r))
	}
	return &buffer{arr: buf}
}

func (b *buffer) slice(i, j int) string {
	return string(b.arr[i:j])
}

func f(s string) string {
	b := newBuffer(s)
	return b.slice(0, len(b.arr))
}

func vuln() { // @vuln
	v := mark.Src("conv-char-decode-rune-vuln") // @source(conv-char-decode-rune-vuln) @miss(silent) @ref(#198)
	mark.SinkSQL(f(v))                          // @sink(conv-char-decode-rune-vuln)
}

func fixed() { // @fixed
	mark.SinkSQL(f(mark.CleanSQL(mark.Src("conv-char-decode-rune-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-decode-rune-decoy") // @source(conv-char-decode-rune-decoy) @miss(silent) @ref(#198)
	mark.SinkSQL(f("constant"))                  // @sink(none) @nosink(conv-char-decode-rune-decoy)
	mark.SinkCmd(f(a))                           // @sink(conv-char-decode-rune-decoy)
}
