// Package web is a small web framework in the shape of gin and macaron: a
// context that carries the request, a router with groups and middleware, and
// handlers that are function values.
package web

import "net/url"

// Request is what arrives.
type Request struct {
	Values url.Values
	Header map[string]string
	Params map[string]string
}

// Context is what a handler gets.
type Context struct {
	Req      *Request
	keys     map[string]any
	handlers []HandlerFunc
	index    int
	Out      []string
}

// HandlerFunc is a handler or a middleware.
type HandlerFunc func(*Context)

// Query, Param, PostForm and GetHeader are sources (rules.yaml).
func (c *Context) Query(k string) string     { return c.Req.Values.Get(k) }
func (c *Context) Param(k string) string     { return c.Req.Params[k] }
func (c *Context) PostForm(k string) string  { return c.Req.Values.Get(k) }
func (c *Context) GetHeader(k string) string { return c.Req.Header[k] }
func (c *Context) Bind(dst any) error        { return nil }

// Set and Get keep a value for the handlers that come later.
func (c *Context) Set(k string, v any) {
	if c.keys == nil {
		c.keys = map[string]any{}
	}
	c.keys[k] = v
}

func (c *Context) Get(k string) (any, bool) {
	v, ok := c.keys[k]
	return v, ok
}

// GetString is Get for a string.
func (c *Context) GetString(k string) string {
	if v, ok := c.keys[k]; ok {
		s, _ := v.(string)
		return s
	}
	return ""
}

// Redirect is a sink (open redirect); HTML is a sink (xss).
func (c *Context) Redirect(to string) { c.Out = append(c.Out, "Location: "+to) }
func (c *Context) HTML(body string)   { c.Out = append(c.Out, body) }

// JSON writes a status: it is not a sink.
func (c *Context) JSON(code int, v any) { c.Out = append(c.Out, "json") }

// Next runs the rest of the chain.
func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort stops the chain.
func (c *Context) Abort() { c.index = len(c.handlers) }
