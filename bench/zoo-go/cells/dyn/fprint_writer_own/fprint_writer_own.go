// Package fprint_writer_own is a cell of the zoo.
//
// @cell(F/dyn/fprint-writer-own)
// @shape: a value is written with fmt.Fprint to a writer of the project, and read back from the field that its Write stores it in
package fprint_writer_own

import (
	"fmt"

	"zoo/mark"
)

func init() {
	mark.Case("F/dyn/fprint-writer-own/vuln", vuln)
	mark.Case("F/dyn/fprint-writer-own/fixed", fixed)
	mark.Case("F/dyn/fprint-writer-own/decoy", decoy)
}

type sliceWriter struct{ data []byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func vuln() { // @vuln
	w := &sliceWriter{}
	fmt.Fprint(w, mark.Src("dyn-fprint-writer-own-vuln")) // @source(dyn-fprint-writer-own-vuln)
	mark.SinkCmd(string(w.data))                          // @sink(dyn-fprint-writer-own-vuln)
}

func fixed() { // @fixed
	w := &sliceWriter{}
	fmt.Fprint(w, mark.Src("dyn-fprint-writer-own-fixed"))
	mark.SinkCmd(mark.CleanCmd(string(w.data))) // @sink(none)
}

type namedWriter struct {
	name string
	data []byte
}

func (w *namedWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func decoy() { // @decoy
	w := &namedWriter{name: "constant"}
	fmt.Fprint(w, mark.Src("dyn-fprint-writer-own-decoy")) // @source(dyn-fprint-writer-own-decoy)
	mark.SinkCmd(w.name)                                   // @sink(none) @fp @nosink(dyn-fprint-writer-own-decoy) @fp @ref(FIELD-SINK)
	mark.SinkSQL(string(w.data))                           // @sink(dyn-fprint-writer-own-decoy)
}
