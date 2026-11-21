// Package iface_write_passed is a cell of the zoo.
//
// @cell(F/dyn/iface-write-passed)
// @shape: a pointer is passed to a function as an io.Writer, written to there, and read from the original pointer
package iface_write_passed

import (
	"bytes"
	"fmt"
	"io"

	"zoo/mark"
)

func init() {
	mark.Case("F/dyn/iface-write-passed/vuln", vuln)
	mark.Case("F/dyn/iface-write-passed/fixed", fixed)
	mark.Case("F/dyn/iface-write-passed/decoy", decoy)
}

func fill(w io.Writer, s string) { fmt.Fprint(w, s) }

func vuln() { // @vuln
	var b bytes.Buffer
	fill(&b, mark.Src("dyn-iface-write-passed-vuln")) // @source(dyn-iface-write-passed-vuln)
	mark.SinkCmd(b.String())                          // @sink(dyn-iface-write-passed-vuln)
}

func fixed() { // @fixed
	var b bytes.Buffer
	fill(&b, mark.Src("dyn-iface-write-passed-fixed"))
	mark.SinkCmd(mark.CleanCmd(b.String())) // @sink(none)
}

func decoy() { // @decoy
	var b, other bytes.Buffer
	fill(&b, mark.Src("dyn-iface-write-passed-decoy")) // @source(dyn-iface-write-passed-decoy)
	mark.SinkCmd(other.String())                       // @sink(none) @nosink(dyn-iface-write-passed-decoy)
	mark.SinkSQL(b.String())                           // @sink(dyn-iface-write-passed-decoy)
}
