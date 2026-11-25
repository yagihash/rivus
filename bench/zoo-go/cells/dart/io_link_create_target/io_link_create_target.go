// Package io_link_create_target is a cell of the zoo.
//
// @cell(F/dart/io-link-create-target)
// @shape: the target of a symbolic link (os.Symlink) is a path sink of the built-in rows: a link whose target is a stored name points anywhere (zip slip by a link). The name of the link itself is the other argument. os has no recording skeleton, so the same value is also given to mark.SinkPath on the line: the run sees the sink there
package io_link_create_target

import (
	"os"
	"path/filepath"

	"zoo/mark"
)

func init() {
	mark.Case("F/dart/io-link-create-target/vuln", vuln)
	mark.Case("F/dart/io-link-create-target/fixed", fixed)
	mark.Case("F/dart/io-link-create-target/decoy", decoy)
}

func vuln() { // @vuln
	tmp, err := os.MkdirTemp("", "zoo-link")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmp)
	a := mark.Src("dart-io-link-create-target-vuln")                                   // @source(dart-io-link-create-target-vuln)
	func() { mark.SinkPath("/srv/base", a); os.Symlink(a, filepath.Join(tmp, "a")) }() // @sink(dart-io-link-create-target-vuln)
}

func fixed() { // @fixed
	tmp, err := os.MkdirTemp("", "zoo-link")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmp)
	a := mark.CleanPath(mark.Src("dart-io-link-create-target-fixed"))
	func() { mark.SinkPath("/srv/base", a); os.Symlink(a, filepath.Join(tmp, "a")) }() // @sink(none)
}

func decoy() { // @decoy
	tmp, err := os.MkdirTemp("", "zoo-link")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmp)
	v := mark.Src("dart-io-link-create-target-decoy")                                                    // @source(dart-io-link-create-target-decoy)
	func() { mark.SinkPath("/srv/base", "constant"); os.Symlink("constant", filepath.Join(tmp, "a")) }() // @sink(none) @nosink(dart-io-link-create-target-decoy)
	mark.SinkSQL(v)                                                                                      // @sink(dart-io-link-create-target-decoy)
}
