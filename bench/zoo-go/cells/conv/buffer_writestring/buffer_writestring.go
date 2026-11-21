// Package buffer_writestring is a cell of the zoo.
//
// @cell(F/conv/buffer-writestring)
// @shape: io.WriteString writes the value into a bytes.Buffer, and b.String() is the sink (the callee has no body in the analysis: a rule for it)
package buffer_writestring

import (
	"bytes"
	"io"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/buffer-writestring/vuln", vuln)
	mark.Case("F/conv/buffer-writestring/fixed", fixed)
	mark.Case("F/conv/buffer-writestring/decoy", decoy)
}

func vuln() { // @vuln
	var b bytes.Buffer
	_, _ = io.WriteString(&b, mark.Src("conv-buffer-writestring-vuln")) // @source(conv-buffer-writestring-vuln) @miss(silent) @ref(CORE-K)
	mark.SinkCmd(b.String())                                            // @sink(conv-buffer-writestring-vuln)
}

func fixed() { // @fixed
	var b bytes.Buffer
	_, _ = io.WriteString(&b, mark.CleanCmd(mark.Src("conv-buffer-writestring-fixed")))
	mark.SinkCmd(b.String()) // @sink(none)
}

func decoy() { // @decoy
	var a, b bytes.Buffer
	_, _ = io.WriteString(&a, mark.Src("conv-buffer-writestring-decoy")) // @source(conv-buffer-writestring-decoy) @miss(silent) @ref(CORE-K)
	_, _ = io.WriteString(&b, "constant")
	mark.SinkCmd(b.String()) // @sink(none) @nosink(conv-buffer-writestring-decoy)
	mark.SinkSQL(a.String()) // @sink(conv-buffer-writestring-decoy)
}
