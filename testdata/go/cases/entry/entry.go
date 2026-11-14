package entry

import "example.com/cases/lib"

// A framework hands payload to Handle: the rule says so, nothing else
// connects it to an attacker.
func Handle(ctx string, payload string) { // @source(entry)
	lib.Sink(payload) // @sink(entry)
	lib.Sink(ctx)     // @sink(none)
}

type Server struct{ name string }

func (s *Server) Serve(req string) { // @source(method)
	lib.Sink(req)    // @sink(method)
	lib.Sink(s.name) // @sink(method)
}

// NotAnEntryPoint has the same shape but no rule.
func NotAnEntryPoint(ctx string, payload string) {
	lib.Sink(payload) // @sink(none)
}
