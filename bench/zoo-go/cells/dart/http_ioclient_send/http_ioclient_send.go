// Package http_ioclient_send is a cell of the zoo.
//
// @cell(F/dart/http-ioclient-send)
// @shape: a value goes into the request that is sent by (*http.Client).Do of a variable typed *http.Client (the concrete type), not by an interface that has Do. The client has the transport of mark, which makes no connection and records the URL
package http_ioclient_send

import (
	"net/http"

	"zoo/mark"
)

func init() {
	mark.Case("F/dart/http-ioclient-send/vuln", vuln)
	mark.Case("F/dart/http-ioclient-send/fixed", fixed)
	mark.Case("F/dart/http-ioclient-send/decoy", decoy)
}

func vuln() { // @vuln
	var c *http.Client = &http.Client{Transport: mark.Transport{}}
	v := mark.Src("dart-http-ioclient-send-vuln")                                             // @source(dart-http-ioclient-send-vuln)
	func() { r, _ := http.NewRequest("GET", "http://example.invalid/?q="+v, nil); c.Do(r) }() // @sink(dart-http-ioclient-send-vuln)
}

func fixed() { // @fixed
	var c *http.Client = &http.Client{Transport: mark.Transport{}}
	v := mark.CleanURL(mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("dart-http-ioclient-send-fixed"))))))
	func() { r, _ := http.NewRequest("GET", "http://example.invalid/?q="+v, nil); c.Do(r) }() // @sink(none)
}

func decoy() { // @decoy
	var c *http.Client = &http.Client{Transport: mark.Transport{}}
	v := mark.Src("dart-http-ioclient-send-decoy") // @source(dart-http-ioclient-send-decoy)
	r, _ := http.NewRequest("GET", "http://example.invalid/", nil)
	c.Do(r)         // @sink(none) @nosink(dart-http-ioclient-send-decoy)
	mark.SinkSQL(v) // @sink(dart-http-ioclient-send-decoy)
}
