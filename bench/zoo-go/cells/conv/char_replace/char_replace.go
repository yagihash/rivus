// Package char_replace is a cell of the zoo.
//
// @cell(F/conv/char-replace)
// @shape: every rune of the text is read by a range loop, mapped one by one (a replacement) and appended to a []rune
// @property: text carried through per-character reads and writes reaches the sink
package char_replace

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-replace/vuln", vuln)
	mark.Case("F/conv/char-replace/fixed", fixed)
	mark.Case("F/conv/char-replace/decoy", decoy)
}

func f(s string) string {
	var out []rune
	for _, r := range s {
		if r == 'S' {
			r = 's'
		}
		out = append(out, r)
	}
	return string(out)
}

func vuln() { // @vuln
	v := mark.Src("conv-char-replace-vuln") // @source(conv-char-replace-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                      // @sink(conv-char-replace-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-replace-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-replace-decoy") // @source(conv-char-replace-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))              // @sink(none) @nosink(conv-char-replace-decoy)
	mark.SinkSQL(f(a))                       // @sink(conv-char-replace-decoy)
}
