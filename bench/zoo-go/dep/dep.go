// Package zoodep stands for a dependency of the zoo: a module of its own (go.mod, a replace in the zoo's),
// so that its functions are Dep functions: rivus reads their bodies as it reads those of a third-party
// package, and does not analyse them as the code under test.
package zoodep

import "io"

// Wrap returns a writer that hands what is written to w (the shape of bufio.Writer, io.MultiWriter and
// the compressing and logging wrappers: a dependency's Write that calls the Write of what it wraps).
func Wrap(w io.Writer) io.Writer { return &wrapper{w: w} }

type wrapper struct{ w io.Writer }

func (x *wrapper) Write(p []byte) (int, error) { return x.w.Write(p) }

// WrapFn returns a function that calls f (the shape of the middleware of the web frameworks: the closure that the dependency makes holds the function it was given).
func WrapFn(f func(string) string) func(string) string { return func(s string) string { return f(s) } }

// Request is the request that a Router hands to a handler.
type Request struct{ Path string }

// Router is the skeleton of a router for the cells (the shape of the routers of the web frameworks): a handler is registered with a
// route, and Serve calls it with the request and the segment that the route captures (/item/{id}: the text of id).
type Router struct {
	routes map[string]func(*Request, string)
}

// NewRouter makes a Router.
func NewRouter() *Router { return &Router{routes: map[string]func(*Request, string){}} }

// Get registers the handler of the route.
func (r *Router) Get(route string, h func(*Request, string)) { r.routes[route] = h }

// Serve calls the handler of the route with the request and the segment of the path.
func (r *Router) Serve(route string, req *Request, segment string) { r.routes[route](req, segment) }
