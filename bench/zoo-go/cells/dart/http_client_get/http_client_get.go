// Package http_client_get is a cell of the zoo.
//
// @cell(F/dart/http-client-get)
// @shape: a value goes into the URL of (*http.Client).Get: the SSRF sink of the built-in rows. The client has the transport of mark, which makes no connection and records the URL
package http_client_get

import (
	"net/http"

	"zoo/mark"
)

func init() {
	mark.Case("F/dart/http-client-get/vuln", vuln)
	mark.Case("F/dart/http-client-get/fixed", fixed)
	mark.Case("F/dart/http-client-get/decoy", decoy)
}

func client() *http.Client { return &http.Client{Transport: mark.Transport{}} }

func vuln() { // @vuln
	c := client()
	v := mark.Src("dart-http-client-get-vuln") // @source(dart-http-client-get-vuln)
	c.Get("http://example.invalid/?q=" + v)    // @sink(dart-http-client-get-vuln)
}

func fixed() { // @fixed
	c := client()
	v := mark.CleanURL(mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-http-client-get-fixed"))))))
	c.Get("http://example.invalid/?q=" + v) // @sink(none)
}

func decoy() { // @decoy
	c := client()
	v := mark.Src("dart-http-client-get-decoy") // @source(dart-http-client-get-decoy)
	r, _ := http.NewRequest("GET", "http://example.invalid/", nil)
	r.Header.Set("x", v)
	c.Do(r)         // @sink(none) @nosink(dart-http-client-get-decoy)
	mark.SinkSQL(v) // @sink(dart-http-client-get-decoy)
}
