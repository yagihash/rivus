// Package stdlib_writer_own is a cell of the zoo.
//
// @cell(F/dyn/stdlib-writer-own)
// @shape: the project implements io.Writer, and a value written through the interface reaches a sink in its Write
package stdlib_writer_own

import (
	"io"

	"zoo/mark"
)

func init() {
	mark.Case("F/dyn/stdlib-writer-own/vuln", vuln)
	mark.Case("F/dyn/stdlib-writer-own/fixed", fixed)
	mark.Case("F/dyn/stdlib-writer-own/decoy", decoy)
}

type shellWriter struct{}

func (shellWriter) Write(p []byte) (int, error) {
	mark.SinkCmdBytes(p) // @sink(dyn-stdlib-writer-own-vuln)
	return len(p), nil
}

func vuln() { // @vuln
	var out io.Writer = shellWriter{}
	_, _ = out.Write(mark.SrcBytes("dyn-stdlib-writer-own-vuln")) // @source(dyn-stdlib-writer-own-vuln)
}

type fixedWriter struct{}

func (fixedWriter) Write(p []byte) (int, error) {
	mark.SinkCmd(mark.CleanCmd(string(p))) // @sink(none)
	return len(p), nil
}

func fixed() { // @fixed
	var out io.Writer = fixedWriter{}
	_, _ = out.Write(mark.SrcBytes("dyn-stdlib-writer-own-fixed"))
}

type quietWriter struct{}

func (quietWriter) Write(p []byte) (int, error) {
	mark.SinkCmdBytes(p) // @sink(none) @nosink(dyn-stdlib-writer-own-decoy)
	return len(p), nil
}

type loudWriter struct{}

func (loudWriter) Write(p []byte) (int, error) {
	mark.SinkSQL(string(p)) // @sink(dyn-stdlib-writer-own-decoy)
	return len(p), nil
}

func decoy() { // @decoy
	var quiet io.Writer = quietWriter{}
	var loud io.Writer = loudWriter{}
	_, _ = quiet.Write([]byte("constant"))
	_, _ = loud.Write(mark.SrcBytes("dyn-stdlib-writer-own-decoy")) // @source(dyn-stdlib-writer-own-decoy)
}
