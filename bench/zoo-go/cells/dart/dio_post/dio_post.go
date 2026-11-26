// Package dio_post is a cell of the zoo.
//
// @cell(F/dart/dio-post)
// @shape: a value goes into the URL of http.Post (the function of the package, which sends by the default transport): the SSRF sink of the built-in rows. The default transport is replaced by the one of mark, which makes no connection and records the URL
package dio_post

import (
	"net/http"
	"strings"

	"zoo/mark"
)

func init() {
	http.DefaultTransport = mark.Transport{}
	mark.Case("F/dart/dio-post/vuln", vuln)
	mark.Case("F/dart/dio-post/fixed", fixed)
	mark.Case("F/dart/dio-post/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("dart-dio-post-vuln")                          // @source(dart-dio-post-vuln)
	http.Post("http://example.invalid/?q="+v, "text/plain", nil) // @sink(dart-dio-post-vuln)
}

func fixed() { // @fixed
	v := mark.CleanURL(mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-dio-post-fixed"))))))
	http.Post("http://example.invalid/?q="+v, "text/plain", nil) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("dart-dio-post-decoy")                                     // @source(dart-dio-post-decoy)
	http.Post("http://example.invalid/", "text/plain", strings.NewReader(v)) // @sink(none) @nosink(dart-dio-post-decoy)
	mark.SinkSQL(v)                                                          // @sink(dart-dio-post-decoy)
}
