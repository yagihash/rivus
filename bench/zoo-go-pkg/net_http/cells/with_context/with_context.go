// Package with_context is a cell of the package net/http.
//
// @cell(F/pkg/net-http/with-context)
// @shape: the context given to (*http.Request).WithContext carries what was put in it, and the request that comes back has it
package with_context

import (
	"context"
	"net/http"

	"zoo/mark"
)

type key struct{}

func init() {
	mark.Case("F/pkg/net-http/with-context/vuln", vuln)
	mark.Case("F/pkg/net-http/with-context/fixed", fixed)
	mark.Case("F/pkg/net-http/with-context/decoy", decoy)
}

func request() *http.Request {
	r, err := http.NewRequest("GET", "http://zoo.invalid/", nil)
	if err != nil {
		panic(err)
	}
	return r
}

func vuln() { // @vuln
	v := mark.Src("pkg-net-http-with-context-vuln") // @source(pkg-net-http-with-context-vuln)
	ctx := context.WithValue(context.Background(), key{}, v)
	r := request().WithContext(ctx)
	mark.SinkCmd(r.Context().Value(key{}).(string)) // @sink(pkg-net-http-with-context-vuln)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("pkg-net-http-with-context-fixed"))
	ctx := context.WithValue(context.Background(), key{}, v)
	r := request().WithContext(ctx)
	mark.SinkCmd(r.Context().Value(key{}).(string)) // @sink(none)
}

func decoy() { // @decoy
	v := mark.Src("pkg-net-http-with-context-decoy") // @source(pkg-net-http-with-context-decoy)
	a := request().WithContext(context.WithValue(context.Background(), key{}, "constant"))
	b := request().WithContext(context.WithValue(context.Background(), key{}, v))
	mark.SinkCmd(a.Context().Value(key{}).(string)) // @sink(none) @nosink(pkg-net-http-with-context-decoy)
	mark.SinkSQL(b.Context().Value(key{}).(string)) // @sink(pkg-net-http-with-context-decoy)
}
