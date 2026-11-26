// Package dio_geturi is a cell of the zoo.
//
// @cell(F/dart/dio-geturi)
// @shape: a value goes into the URL object (*url.URL) made by url.Parse, whose String() is given to http.NewRequest and the request is sent by (*http.Client).Do: the SSRF sink of the built-in rows (the sink of the model is the NewRequest, so it and the Do are on one line; the run records the URL at the Do). A request made as a struct literal with the URL object is not a form of this cell. The client has the transport of mark, which makes no connection and records the URL
package dio_geturi

import (
	"net/http"
	"net/url"

	"zoo/mark"
)

func init() {
	mark.Case("F/dart/dio-geturi/vuln", vuln)
	mark.Case("F/dart/dio-geturi/fixed", fixed)
	mark.Case("F/dart/dio-geturi/decoy", decoy)
}

func client() *http.Client { return &http.Client{Transport: mark.Transport{}} }

func vuln() { // @vuln
	c := client()
	v := mark.Src("dart-dio-geturi-vuln") // @source(dart-dio-geturi-vuln)
	u, _ := url.Parse("http://example.invalid/?q=" + v)
	func() { r, _ := http.NewRequest("GET", u.String(), nil); c.Do(r) }() // @sink(dart-dio-geturi-vuln)
}

func fixed() { // @fixed
	c := client()
	v := mark.CleanURL(mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-dio-geturi-fixed"))))))
	u, _ := url.Parse("http://example.invalid/?q=" + v)
	func() { r, _ := http.NewRequest("GET", u.String(), nil); c.Do(r) }() // @sink(none)
}

func decoy() { // @decoy
	c := client()
	v := mark.Src("dart-dio-geturi-decoy") // @source(dart-dio-geturi-decoy)
	u, _ := url.Parse("http://example.invalid/")
	c.Do(&http.Request{Method: "GET", URL: u, Header: http.Header{"X": {v}}}) // @sink(none) @nosink(dart-dio-geturi-decoy)
	mark.SinkSQL(v)                                                           // @sink(dart-dio-geturi-decoy)
}
