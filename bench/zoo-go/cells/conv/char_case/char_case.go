// Package char_case is a cell of the zoo.
//
// @cell(F/conv/char-case)
// @shape: every rune of the text is read by a range loop, converted with unicode.ToUpper and written with strings.Builder.WriteRune
// @property: text carried through per-character reads and writes reaches the sink
package char_case

import (
	"strings"
	"unicode"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-case/vuln", vuln)
	mark.Case("F/conv/char-case/fixed", fixed)
	mark.Case("F/conv/char-case/decoy", decoy)
}

func f(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func vuln() { // @vuln
	v := mark.Src("conv-char-case-vuln") // @source(conv-char-case-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                   // @sink(conv-char-case-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-case-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-case-decoy") // @source(conv-char-case-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("constant"))           // @sink(none) @nosink(conv-char-case-decoy)
	mark.SinkSQL(f(a))                    // @sink(conv-char-case-decoy)
}
