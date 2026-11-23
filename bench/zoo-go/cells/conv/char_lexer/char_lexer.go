// Package char_lexer is a cell of the zoo.
//
// @cell(F/conv/char-lexer)
// @shape: a lexer reads the text one byte at a time (s[i]), collects the bytes of a token with append, and hands out the tokens
// @property: text carried through per-character reads and writes reaches the sink
package char_lexer

import (
	"strings"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/char-lexer/vuln", vuln)
	mark.Case("F/conv/char-lexer/fixed", fixed)
	mark.Case("F/conv/char-lexer/decoy", decoy)
}

func lex(s string) []string {
	var out []string
	var cur []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' {
			out = append(out, string(cur))
			cur = nil
			continue
		}
		cur = append(cur, c)
	}
	return append(out, string(cur))
}

func f(s string) string { return strings.Join(lex(s), " ") }

func vuln() { // @vuln
	v := mark.Src("conv-char-lexer-vuln") // @source(conv-char-lexer-vuln) @miss(silent) @ref(#185)
	mark.SinkCmd(f(v))                    // @sink(conv-char-lexer-vuln)
}

func fixed() { // @fixed
	mark.SinkCmd(f(mark.CleanCmd(mark.Src("conv-char-lexer-fixed")))) // @sink(none)
}

func decoy() { // @decoy
	a := mark.Src("conv-char-lexer-decoy") // @source(conv-char-lexer-decoy) @miss(silent) @ref(#185)
	mark.SinkCmd(f("a b"))                 // @sink(none) @nosink(conv-char-lexer-decoy)
	mark.SinkSQL(f(a))                     // @sink(conv-char-lexer-decoy)
}
