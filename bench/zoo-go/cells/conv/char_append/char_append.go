// Package char_append is a cell of the zoo.
//
// @cell(F/conv/char-append)
// @shape: the bytes of the text are read one by one (s[i]) and appended to a []byte
// @property: text carried through per-character reads and writes reaches the sink
package char_append

import (
	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-append/vuln", vuln)
	mark.Case("F/conv/char-append/fixed", fixed)
	mark.Case("F/conv/char-append/decoy", decoy)
}

func f(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		out = append(out, s[i])
	}
	return string(out)
}

func vuln() { // @vuln
	v := mark.Src("conv-char-append-vuln") // @source(conv-char-append-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                     // @sink(conv-char-append-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-append-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-append-decoy") // @source(conv-char-append-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))             // @sink(none) @nosink(conv-char-append-decoy)
	mark.SinkSQL(f(a))                      // @sink(conv-char-append-decoy)
}
