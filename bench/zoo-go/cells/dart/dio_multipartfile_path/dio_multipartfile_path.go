// Package dio_multipartfile_path is a cell of the zoo.
//
// @cell(F/dart/dio-multipartfile-path)
// @shape: a value goes into the path of the file that is read to be a part of a multipart body (os.Open, then mime/multipart): the path traversal sink of the built-in rows. os has no recording skeleton, so the same value is also given to mark.SinkPath on the line: the run sees the sink there
package dio_multipartfile_path

import (
	"bytes"
	"mime/multipart"
	"os"

	"zoo/mark"
)

func init() {
	mark.Case("F/dart/dio-multipartfile-path/vuln", vuln)
	mark.Case("F/dart/dio-multipartfile-path/fixed", fixed)
	mark.Case("F/dart/dio-multipartfile-path/decoy", decoy)
}

func part() *multipart.Writer { return multipart.NewWriter(&bytes.Buffer{}) }

func vuln() { // @vuln
	v := mark.Src("dart-dio-multipartfile-path-vuln") // @source(dart-dio-multipartfile-path-vuln)
	w := part()
	w.CreateFormFile("file", "upload.bin")
	func() { mark.SinkPath("/srv/base", v); os.Open(v) }() // @sink(dart-dio-multipartfile-path-vuln)
}

func fixed() { // @fixed
	v := mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-dio-multipartfile-path-fixed")))))
	w := part()
	w.CreateFormFile("file", "upload.bin")
	func() { mark.SinkPath("/srv/base", v); os.Open(v) }() // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("dart-dio-multipartfile-path-decoy") // @source(dart-dio-multipartfile-path-decoy)
	w := part()
	w.CreateFormFile("file", v)
	func() { mark.SinkPath("/srv/base", "out.bin"); os.Open("out.bin") }() // @sink(none) @nosink(dart-dio-multipartfile-path-decoy)
	mark.SinkSQL(v)                                                        // @sink(dart-dio-multipartfile-path-decoy)
}
