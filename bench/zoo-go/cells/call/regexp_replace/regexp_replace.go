// Package regexp_replace is a cell of the zoo.
//
// @cell(F/call/regexp-replace)
// @shape: a value comes out of a replacement of the standard library (a replacement that is a value, a function that is given and returns it)
package regexp_replace

import (
	"regexp"

	"zoo/mark"
)

func init() {
	mark.Case("F/call/regexp-replace/vuln", vuln)
	mark.Case("F/call/regexp-replace/vuln2", vuln2)
	mark.Case("F/call/regexp-replace/vuln3", vuln3)
	mark.Case("F/call/regexp-replace/fixed", fixed)
	mark.Case("F/call/regexp-replace/decoy", decoy)
}

var re = regexp.MustCompile("x")

// vuln: the function returns a value that it captured (the bytes variant).
func vuln() { // @vuln
	v := mark.Src("call-regexp-replace-vuln") // @source(call-regexp-replace-vuln)
	out := re.ReplaceAllFunc([]byte("xx"), func([]byte) []byte { return []byte(v) })
	mark.SinkCmdBytes(out) // @sink(call-regexp-replace-vuln)
}

// vuln2: the replacement is the value (ReplaceAllLiteralString).
func vuln2() { // @vuln
	v := mark.Src("call-regexp-replace-vuln2") // @source(call-regexp-replace-vuln2)
	out := re.ReplaceAllLiteralString("xx", v)
	mark.SinkCmd(out) // @sink(call-regexp-replace-vuln2)
}

// vuln3: the text is the value (ReplaceAllLiteral).
func vuln3() { // @vuln
	v := mark.SrcBytes("call-regexp-replace-vuln3") // @source(call-regexp-replace-vuln3)
	out := re.ReplaceAllLiteral(v, []byte("-"))
	mark.SinkCmdBytes(out) // @sink(call-regexp-replace-vuln3)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("call-regexp-replace-fixed"))
	out := re.ReplaceAllLiteralString("xx", v)
	mark.SinkCmd(out) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("call-regexp-replace-decoy") // @source(call-regexp-replace-decoy)
	out := re.ReplaceAllFunc([]byte("xx"), func([]byte) []byte { _ = v; return []byte("constant") })
	mark.SinkCmdBytes(out) // @sink(none) @nosink(call-regexp-replace-decoy)
	mark.SinkSQL(v)        // @sink(call-regexp-replace-decoy)
}
