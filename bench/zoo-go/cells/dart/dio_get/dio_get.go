// Package dio_get is a cell of the zoo.
//
// @cell(F/dart/dio-get)
// @shape: a value goes into the URL of http.Get (the function of the package, which sends by the default transport): the SSRF sink of the built-in rows. The default transport is replaced by the one of mark, which makes no connection and records the URL
package dio_get

import (
	"net/http"
	"strings"

	"zoo/mark"
)

func init() {
	http.DefaultTransport = mark.Transport{}
	mark.Case("F/dart/dio-get/vuln", vuln)
	mark.Case("F/dart/dio-get/fixed", fixed)
	mark.Case("F/dart/dio-get/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("dart-dio-get-vuln")         // @source(dart-dio-get-vuln)
	http.Get("http://example.invalid/?q=" + v) // @sink(dart-dio-get-vuln)
}

func fixed() { // @fixed
	v := mark.CleanURL(mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-dio-get-fixed"))))))
	http.Get("http://example.invalid/?q=" + v) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("dart-dio-get-decoy")                                      // @source(dart-dio-get-decoy)
	http.Post("http://example.invalid/", "text/plain", strings.NewReader(v)) // @sink(none) @nosink(dart-dio-get-decoy)
	mark.SinkSQL(v)                                                          // @sink(dart-dio-get-decoy)
}
