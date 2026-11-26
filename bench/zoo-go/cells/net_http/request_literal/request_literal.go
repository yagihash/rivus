// Package request_literal is a cell of the zoo.
//
// @cell(F/net-http/request-literal)
// @shape: a value goes into the URL object (*url.URL) made by url.Parse, the request is made as a struct literal with that URL object in the field URL (not by http.NewRequest) and is sent by (*http.Client).Do. The sink of the built-in rows is the argument of NewRequest, so the literal is not a sink. The client has the transport of mark, which makes no connection and records the URL
// @property: a request made as a literal with a URL from the source is reported as an SSRF sink at the Do
package request_literal

import (
	"net/http"
	"net/url"

	"zoo/mark"
)

func init() {
	mark.Case("F/net-http/request-literal/vuln", vuln)
	mark.Case("F/net-http/request-literal/fixed", fixed)
	mark.Case("F/net-http/request-literal/decoy", decoy)
}

func client() *http.Client { return &http.Client{Transport: mark.Transport{}} }

func vuln() { // @vuln
	c := client()
	v := mark.Src("net-http-request-literal-vuln") // @source(net-http-request-literal-vuln) @miss(silent) @ref(#232)
	u, _ := url.Parse("http://example.invalid/?q=" + v)
	c.Do(&http.Request{Method: "GET", URL: u}) // @sink(net-http-request-literal-vuln)
}

func fixed() { // @fixed
	c := client()
	v := mark.CleanURL(mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(mark.Src("net-http-request-literal-fixed"))))))
	u, _ := url.Parse("http://example.invalid/?q=" + v)
	c.Do(&http.Request{Method: "GET", URL: u}) // @sink(none)
}

func decoy() { // @decoy
	c := client()
	v := mark.Src("net-http-request-literal-decoy") // @source(net-http-request-literal-decoy)
	u, _ := url.Parse("http://example.invalid/")
	c.Do(&http.Request{Method: "GET", URL: u, Header: http.Header{"X": {v}}}) // @sink(none) @nosink(net-http-request-literal-decoy)
	mark.SinkSQL(v)                                                           // @sink(net-http-request-literal-decoy)
}
