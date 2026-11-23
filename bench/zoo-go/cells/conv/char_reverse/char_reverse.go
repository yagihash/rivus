// Package char_reverse is a cell of the zoo.
//
// @cell(F/conv/char-reverse)
// @shape: the text is reversed rune by rune (rs[i] read from the end, appended), and reversed once more
// @property: text carried through per-character reads and writes reaches the sink
package char_reverse

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-reverse/vuln", vuln)
	mark.Case("F/conv/char-reverse/fixed", fixed)
	mark.Case("F/conv/char-reverse/decoy", decoy)
}

func reverse(s string) string {
	rs := []rune(s)
	out := make([]rune, 0, len(rs))
	for i := len(rs) - 1; i >= 0; i-- {
		out = append(out, rs[i])
	}
	return string(out)
}

func f(s string) string { return reverse(reverse(s)) }

func vuln() { // @vuln
	v := mark.Src("conv-char-reverse-vuln") // @source(conv-char-reverse-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                      // @sink(conv-char-reverse-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-reverse-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-reverse-decoy") // @source(conv-char-reverse-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))              // @sink(none) @nosink(conv-char-reverse-decoy)
	mark.SinkSQL(f(a))                       // @sink(conv-char-reverse-decoy)
}
