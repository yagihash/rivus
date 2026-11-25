// Package callback_param_closure is a cell of the zoo.
//
// @cell(F/dart/callback-param-closure)
// @shape: a function written where the route is registered is called by the router with the segment of the path as its second argument: the segment is user input (callback_param of the rules), the request is not what is marked
package callback_param_closure

import (
	"zoo/mark"
	"zoodep"
)

func init() {
	mark.Case("F/dart/callback-param-closure/vuln", vuln)
	mark.Case("F/dart/callback-param-closure/fixed", fixed)
	mark.Case("F/dart/callback-param-closure/decoy", decoy)
}

func typed(tag string) string { in := mark.NewIn(tag); return in.Value() }

func vuln() { // @vuln
	router := zoodep.NewRouter()
	router.Get("/item/{id}", func(req *zoodep.Request, id string) { // @source(dart-callback-param-closure-vuln)
		mark.SinkCmd(id) // @sink(dart-callback-param-closure-vuln)
	})
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-closure-vuln"))
}

func fixed() { // @fixed
	router := zoodep.NewRouter()
	router.Get("/item/{id}", func(req *zoodep.Request, id string) {
		mark.SinkCmd(mark.CleanCmd(id)) // @sink(none)
	})
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-closure-fixed"))
}

func decoy() { // @decoy
	router := zoodep.NewRouter()
	router.Get("/item/{id}", func(req *zoodep.Request, id string) { // @source(dart-callback-param-closure-decoy)
		mark.SinkCmd(req.Path) // @sink(none) @nosink(dart-callback-param-closure-decoy)
		mark.SinkSQL(id)       // @sink(dart-callback-param-closure-decoy)
	})
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-closure-decoy"))
}
