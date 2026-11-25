// Package callback_param_helper is a cell of the zoo.
//
// @cell(F/dart/callback-param-helper)
// @shape: the route is registered by a helper that is given the function: which function the router is given is not known at the call that the rule names (a parameter of the helper), so the parameters are not marked; the note (unresolved_callee) is in the helper, which is neither the function of the source nor the one of the sink
package callback_param_helper

import (
	"zoo/mark"
	"zoodep"
)

func init() {
	mark.Case("F/dart/callback-param-helper/vuln", vuln)
	mark.Case("F/dart/callback-param-helper/fixed", fixed)
	mark.Case("F/dart/callback-param-helper/decoy", decoy)
}

func typed(tag string) string { in := mark.NewIn(tag); return in.Value() }

func mount(router *zoodep.Router, h func(*zoodep.Request, string)) {
	router.Get("/item/{id}", h)
}

func vuln() { // @vuln
	router := zoodep.NewRouter()
	mount(router, func(req *zoodep.Request, id string) { // @source(dart-callback-param-helper-vuln)
		mark.SinkCmd(id) // @sink(dart-callback-param-helper-vuln)
	})
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-helper-vuln"))
}

func fixed() { // @fixed
	router := zoodep.NewRouter()
	mount(router, func(req *zoodep.Request, id string) {
		mark.SinkCmd(mark.CleanCmd(id)) // @sink(none)
	})
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-helper-fixed"))
}

func decoy() { // @decoy
	router := zoodep.NewRouter()
	mount(router, func(req *zoodep.Request, id string) { // @source(dart-callback-param-helper-decoy)
		mark.SinkCmd(req.Path) // @sink(none) @nosink(dart-callback-param-helper-decoy)
		mark.SinkSQL(id)       // @sink(dart-callback-param-helper-decoy)
	})
	router.Serve("/item/{id}", &zoodep.Request{Path: "/item/x"}, typed("dart-callback-param-helper-decoy"))
}
