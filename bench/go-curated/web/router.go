package web

// Engine holds the routes.
type Engine struct {
	RouterGroup
	routes map[string][]HandlerFunc
}

// RouterGroup is a prefix and the middleware that goes with it.
type RouterGroup struct {
	engine   *Engine
	prefix   string
	handlers []HandlerFunc
}

// New makes an engine.
func New() *Engine {
	e := &Engine{routes: map[string][]HandlerFunc{}}
	e.RouterGroup = RouterGroup{engine: e}
	return e
}

// Use adds middleware to the group.
func (g *RouterGroup) Use(h ...HandlerFunc) { g.handlers = append(g.handlers, h...) }

// Group makes a sub group that has the middleware of this one.
func (g *RouterGroup) Group(prefix string, h ...HandlerFunc) *RouterGroup {
	n := &RouterGroup{engine: g.engine, prefix: g.prefix + prefix}
	n.handlers = append(append(n.handlers, g.handlers...), h...)
	return n
}

// Handle registers a route.
func (g *RouterGroup) Handle(method, path string, h ...HandlerFunc) {
	chain := append(append([]HandlerFunc{}, g.handlers...), h...)
	g.engine.routes[method+" "+g.prefix+path] = chain
}

// GET and POST register a route.
func (g *RouterGroup) GET(path string, h ...HandlerFunc)  { g.Handle("GET", path, h...) }
func (g *RouterGroup) POST(path string, h ...HandlerFunc) { g.Handle("POST", path, h...) }

// Serve runs the chain of a route for a request.
func (e *Engine) Serve(method, path string, req *Request) *Context {
	c := &Context{Req: req, handlers: e.routes[method+" "+path], index: -1}
	c.Next()
	return c
}
