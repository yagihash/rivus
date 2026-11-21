// Package http_middleware_chain is a cell of the zoo.
//
// @cell(F/dyn/http-middleware-chain)
// @shape: http.Handlers are chained (the outer one calls next.ServeHTTP); the value is a field of the inner handler
package http_middleware_chain

import (
	"net/http"
	"net/http/httptest"

	"zoo/mark"
)

func init() {
	mark.Case("F/dyn/http-middleware-chain/vuln", vuln)
	mark.Case("F/dyn/http-middleware-chain/fixed", fixed)
	mark.Case("F/dyn/http-middleware-chain/decoy", decoy)
}

func serve(h http.Handler) {
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

type logging struct{ next http.Handler }

func (l logging) ServeHTTP(w http.ResponseWriter, r *http.Request) { l.next.ServeHTTP(w, r) }

type inner struct{ v string }

func (i inner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mark.SinkCmd(i.v) // @sink(dyn-http-middleware-chain-vuln)
}

func vuln() { // @vuln
	serve(logging{next: inner{v: mark.Src("dyn-http-middleware-chain-vuln")}}) // @source(dyn-http-middleware-chain-vuln) @miss(silent) @ref(R-04,CORE-J)
}

type loggingFixed struct{ next http.Handler }

func (l loggingFixed) ServeHTTP(w http.ResponseWriter, r *http.Request) { l.next.ServeHTTP(w, r) }

type innerFixed struct{ v string }

func (i innerFixed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mark.SinkCmd(i.v) // @sink(none)
}

func fixed() { // @fixed
	serve(loggingFixed{next: innerFixed{v: mark.CleanCmd(mark.Src("dyn-http-middleware-chain-fixed"))}})
}

type loggingDecoy struct{ next http.Handler }

func (l loggingDecoy) ServeHTTP(w http.ResponseWriter, r *http.Request) { l.next.ServeHTTP(w, r) }

type innerDecoy struct{ a, b string }

func (i innerDecoy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mark.SinkCmd(i.b) // @sink(none) @nosink(dyn-http-middleware-chain-decoy)
	mark.SinkSQL(i.a) // @sink(dyn-http-middleware-chain-decoy)
}

func decoy() { // @decoy
	serve(loggingDecoy{next: innerDecoy{a: mark.Src("dyn-http-middleware-chain-decoy"), b: "constant"}}) // @source(dyn-http-middleware-chain-decoy) @miss(silent) @ref(R-04,CORE-J)
}
