// Package bound: the method values of two handlers. A method value (`s.HandleA`) is a wrapper function
// (`HandleA$bound`) that takes the same request type, at the place of the method it binds: it is the same
// input as HandleA, not another source. Two handlers reach the sink: one finding, and one other source
// (HandleB), not three (before: the twins of both handlers were counted as sources of their own).
package bound

import "os"

type Req struct{ Path string }

type Server struct{}

func save(p string) { os.Create(p) }

func (s *Server) HandleA(r *Req) { save(r.Path) }
func (s *Server) HandleB(r *Req) { save(r.Path) }

func Main(i int) {
	s := &Server{}
	fs := []func(*Req){s.HandleA, s.HandleB}
	fs[i](&Req{Path: "x"})
}
