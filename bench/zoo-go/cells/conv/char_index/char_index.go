// Package char_index is a cell of the zoo.
//
// @cell(F/conv/char-index)
// @shape: the first byte is read with s[0] and made a string with string(s[0]), and the rest is a slice of the text
// @property: text carried through per-character reads and writes reaches the sink
package char_index

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-index/vuln", vuln)
	mark.Case("F/conv/char-index/fixed", fixed)
	mark.Case("F/conv/char-index/decoy", decoy)
}

func f(s string) string { return string(s[0]) + s[1:] }

func vuln() { // @vuln
	v := mark.Src("conv-char-index-vuln") // @source(conv-char-index-vuln)
	mark.SinkCmd(f(v))                    // @sink(conv-char-index-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-index-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-index-decoy") // @source(conv-char-index-decoy)
	mark.SinkCmd(f("constant"))            // @sink(none) @nosink(conv-char-index-decoy)
	mark.SinkSQL(f(a))                     // @sink(conv-char-index-decoy)
}
