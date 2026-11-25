// Package callback_param_named is a cell of the zoo.
//
// @cell(F/dart/callback-param-named)
// @shape: a function that has a name (and a variable that holds a function written once) is given to the router in place of a function literal: it is found as well, and its parameter after the request is user input; the other two arguments of the same name are not
package callback_param_named

import (
	"zoo/mark"
	"zoodep"
)

func init() {
	mark.Case("F/dart/callback-param-named/vuln", vuln)
	mark.Case("F/dart/callback-param-named/fixed", fixed)
	mark.Case("F/dart/callback-param-named/decoy", decoy)
}

func typed(tag string) string { in := mark.NewIn(tag); return in.Value() }

func showItem(req *zoodep.Request, id string) { // @source(dart-callback-param-named-vuln)
	mark.SinkCmd(id) // @sink(dart-callback-param-named-vuln)
}

func vuln() { // @vuln
	router := zoodep.NewRouter()
	router.Get("/item/{id}", showItem)
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-named-vuln"))
}

func showItemFixed(req *zoodep.Request, id string) {
	mark.SinkCmd(mark.CleanCmd(id)) // @sink(none)
}

func fixed() { // @fixed
	router := zoodep.NewRouter()
	router.Get("/item/{id}", showItemFixed)
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-named-fixed"))
}

func decoy() { // @decoy
	router := zoodep.NewRouter()
	handler := func(req *zoodep.Request, id string) { // @source(dart-callback-param-named-decoy)
		mark.SinkCmd(req.Path) // @sink(none) @nosink(dart-callback-param-named-decoy)
		mark.SinkSQL(id)       // @sink(dart-callback-param-named-decoy)
	}
	router.Get("/item/{id}", handler)
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-named-decoy"))
}
