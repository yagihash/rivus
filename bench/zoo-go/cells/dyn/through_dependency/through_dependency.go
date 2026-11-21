// Package through_dependency is a cell of the zoo.
//
// @cell(F/dyn/through-dependency)
// @shape: a value written through io.Writer goes through a dependency (zoodep.Wrap, a module of its own whose body is not read) to a writer of the project, and reaches a sink in its Write
package through_dependency

import (
	"io"

	"zoo/mark"
	"zoodep"
)

func init() {
	mark.Case("F/dyn/through-dependency/vuln", vuln)
	mark.Case("F/dyn/through-dependency/fixed", fixed)
	mark.Case("F/dyn/through-dependency/decoy", decoy)
}

type shellWriter struct{}

func (shellWriter) Write(p []byte) (int, error) {
	mark.SinkCmdBytes(p) // @sink(dyn-through-dependency-vuln)
	return len(p), nil
}

func vuln() { // @vuln
	var out io.Writer = zoodep.Wrap(shellWriter{})
	_, _ = out.Write(mark.SrcBytes("dyn-through-dependency-vuln")) // @source(dyn-through-dependency-vuln) @miss(dependency_body_unread) @ref(ADR-075)
}

type fixedWriter struct{}

func (fixedWriter) Write(p []byte) (int, error) {
	mark.SinkCmd(mark.CleanCmd(string(p))) // @sink(none)
	return len(p), nil
}

func fixed() { // @fixed
	var out io.Writer = zoodep.Wrap(fixedWriter{})
	_, _ = out.Write(mark.SrcBytes("dyn-through-dependency-fixed"))
}

type quietWriter struct{}

func (quietWriter) Write(p []byte) (int, error) {
	mark.SinkCmdBytes(p) // @sink(none) @nosink(dyn-through-dependency-decoy)
	return len(p), nil
}

type loudWriter struct{}

func (loudWriter) Write(p []byte) (int, error) {
	mark.SinkSQL(string(p)) // @sink(dyn-through-dependency-decoy)
	return len(p), nil
}

func decoy() { // @decoy
	var quiet io.Writer = zoodep.Wrap(quietWriter{})
	var loud io.Writer = zoodep.Wrap(loudWriter{})
	_, _ = quiet.Write([]byte("constant"))
	_, _ = loud.Write(mark.SrcBytes("dyn-through-dependency-decoy")) // @source(dyn-through-dependency-decoy) @miss(dependency_body_unread) @ref(ADR-075)
}
