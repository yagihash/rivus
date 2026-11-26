// Package dio_download_savepath is a cell of the zoo.
//
// @cell(F/dart/dio-download-savepath)
// @shape: a value goes into the path of the file that the body of a response is written to (os.OpenFile after (*http.Client).Get): the path traversal sink of the built-in rows. os has no recording skeleton, so the same value is also given to mark.SinkPath on the line: the run sees the sink there (the file is opened for writing and not created, so the run makes none)
package dio_download_savepath

import (
	"net/http"
	"os"
	"strings"

	"zoo/mark"
)

func init() {
	mark.Case("F/dart/dio-download-savepath/vuln", vuln)
	mark.Case("F/dart/dio-download-savepath/fixed", fixed)
	mark.Case("F/dart/dio-download-savepath/decoy", decoy)
}

func client() *http.Client { return &http.Client{Transport: mark.Transport{}} }

func vuln() { // @vuln
	c := client()
	v := mark.Src("dart-dio-download-savepath-vuln") // @source(dart-dio-download-savepath-vuln)
	c.Get("http://example.invalid/file")
	func() { mark.SinkPath("/srv/base", v); os.OpenFile(v, os.O_WRONLY, 0o600) }() // @sink(dart-dio-download-savepath-vuln)
}

func fixed() { // @fixed
	c := client()
	v := mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-dio-download-savepath-fixed")))))
	c.Get("http://example.invalid/file")
	func() { mark.SinkPath("/srv/base", v); os.OpenFile(v, os.O_WRONLY, 0o600) }() // @sink(none)
}

func decoy() { // @decoy
	c := client()
	v := mark.Src("dart-dio-download-savepath-decoy") // @source(dart-dio-download-savepath-decoy)
	c.Post("http://example.invalid/file", "text/plain", strings.NewReader(v))
	func() { mark.SinkPath("/srv/base", "out.bin"); os.OpenFile("out.bin", os.O_WRONLY, 0o600) }() // @sink(none) @nosink(dart-dio-download-savepath-decoy)
	mark.SinkSQL(v)                                                                                // @sink(dart-dio-download-savepath-decoy)
}
