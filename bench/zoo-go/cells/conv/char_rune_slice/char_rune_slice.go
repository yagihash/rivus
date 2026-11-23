// Package char_rune_slice is a cell of the zoo.
//
// @cell(F/conv/char-rune-slice)
// @shape: the text is converted whole to a []rune, sliced, and converted back whole (no element is read)
// @property: text carried through per-character reads and writes reaches the sink
package char_rune_slice

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-rune-slice/vuln", vuln)
	mark.Case("F/conv/char-rune-slice/fixed", fixed)
	mark.Case("F/conv/char-rune-slice/decoy", decoy)
}

func f(s string) string {
	rs := []rune(s)
	return string(rs[:1]) + string(rs[1:])
}

func vuln() { // @vuln
	v := mark.Src("conv-char-rune-slice-vuln") // @source(conv-char-rune-slice-vuln)
	mark.SinkCmd(f(v))                         // @sink(conv-char-rune-slice-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-rune-slice-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-rune-slice-decoy") // @source(conv-char-rune-slice-decoy)
	mark.SinkCmd(f("constant"))                 // @sink(none) @nosink(conv-char-rune-slice-decoy)
	mark.SinkSQL(f(a))                          // @sink(conv-char-rune-slice-decoy)
}
