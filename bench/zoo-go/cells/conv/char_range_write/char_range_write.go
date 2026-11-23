// Package char_range_write is a cell of the zoo.
//
// @cell(F/conv/char-range-write)
// @shape: every rune of the text is read by a range loop and written with strings.Builder.WriteRune
// @property: text carried through per-character reads and writes reaches the sink
package char_range_write

import (
	"strings"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-range-write/vuln", vuln)
	mark.Case("F/conv/char-range-write/fixed", fixed)
	mark.Case("F/conv/char-range-write/decoy", decoy)
}

func f(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
	}
	return b.String()
}

func vuln() { // @vuln
	v := mark.Src("conv-char-range-write-vuln") // @source(conv-char-range-write-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                          // @sink(conv-char-range-write-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-range-write-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-range-write-decoy") // @source(conv-char-range-write-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))                  // @sink(none) @nosink(conv-char-range-write-decoy)
	mark.SinkSQL(f(a))                           // @sink(conv-char-range-write-decoy)
}
