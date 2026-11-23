// Package char_range_append is a cell of the zoo.
//
// @cell(F/conv/char-range-append)
// @shape: every rune of the text is read by a range loop and byte(r) is appended to a []byte, which is converted back whole
// @property: text carried through per-character reads and writes reaches the sink
package char_range_append

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-range-append/vuln", vuln)
	mark.Case("F/conv/char-range-append/fixed", fixed)
	mark.Case("F/conv/char-range-append/decoy", decoy)
}

func f(s string) string {
	var out []byte
	for _, r := range s {
		out = append(out, byte(r))
	}
	return string(out)
}

func vuln() { // @vuln
	v := mark.Src("conv-char-range-append-vuln") // @source(conv-char-range-append-vuln) @miss(silent) @ref(#198)
	mark.SinkSQL(f(v))                           // @sink(conv-char-range-append-vuln)
}

func fixed() { // @fixed
	mark.SinkSQL(f(mark.CleanSQL(mark.Src("conv-char-range-append-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-range-append-decoy") // @source(conv-char-range-append-decoy) @miss(silent) @ref(#198)
	mark.SinkSQL(f("constant"))                   // @sink(none) @nosink(conv-char-range-append-decoy)
	mark.SinkCmd(f(a))                            // @sink(conv-char-range-append-decoy)
}
