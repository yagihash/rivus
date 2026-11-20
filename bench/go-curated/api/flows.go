package api

import (
	"curated/sys"
	"curated/valid"
	"curated/web"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ctxRedirect redirects to what a middleware kept.
func (h *Handlers) ctxRedirect(c *web.Context) {
	c.Redirect(c.GetString("next")) // @sink(ctxkey) @nosink(mw_user) @fp
}

// ctxSafeRedirect redirects to what a middleware kept after cleaning it.
func (h *Handlers) ctxSafeRedirect(c *web.Context) {
	c.Redirect(c.GetString("safe_next")) // @sink(none) @fp
}

// Whoami uses the name that the Auth middleware set.
func (h *Handlers) Whoami(c *web.Context) {
	name := c.GetString("user")
	h.DB.Exec("SELECT * FROM users WHERE name = '" + name + "'") // @sink(mw_user) @nosink(ctxkey) @fp
}

// Command picks a function by a name from the request and runs it.
func (h *Handlers) Command(c *web.Context) {
	ops := map[string]func(string){
		"ls":   runList,
		"echo": runEcho,
	}
	op := c.Param("op")
	arg := c.Query("arg") // @source(funcmap)
	if f, ok := ops[op]; ok {
		f(arg)
	}
}

func runList(arg string) {
	sys.Run("ls", arg) // @sink(funcmap)
}

func runEcho(arg string) {
	sys.Run("echo", valid.Quote(arg)) // @sink(none)
}

// Closure builds a handler that holds its configuration.
func (h *Handlers) Closure(prefix string) web.HandlerFunc {
	return func(c *web.Context) {
		v := c.Query("v")     // @source(closure)
		h.DB.Exec(prefix + v) // @sink(closure)
	}
}

// Message puts the text in the answer; the code of the error is a constant.
func (h *Handlers) Message(c *web.Context) {
	type resp struct{ Msg, Err string }
	r := resp{}
	r.Msg = c.Query("msg") // @source(fields)
	r.Err = "failed"
	c.HTML(r.Err) // @sink(none)
	c.HTML(r.Msg) // @sink(fields)
}

// Wrapped puts the text in an error and the error in the answer.
func (h *Handlers) Wrapped(c *web.Context) {
	name := c.Query("name") // @source(errwrap)
	err := fmt.Errorf("no such user %s: %w", name, errors.New("missing"))
	c.HTML(err.Error()) // @sink(errwrap)
}

// Tags joins what the request lists.
func (h *Handlers) Tags(c *web.Context) {
	var tags []string
	for _, k := range []string{"a", "b"} {
		tags = append(tags, c.Query(k)) // @source(slice)
	}
	sort.Strings(tags)
	h.DB.Exec("SELECT * FROM t WHERE tag IN ('" + strings.Join(tags, "','") + "')") // @sink(slice)
}

// Bound fills a struct from the body.
func (h *Handlers) Bound(c *web.Context) {
	var req struct {
		Filter struct{ Name string }
		Page   int
	}
	c.Bind(&req)                                   // @source(bind)
	h.DB.Where("name = '" + req.Filter.Name + "'") // @sink(bind)
}

// Deferred cleans up with the text.
func (h *Handlers) Deferred(c *web.Context) {
	name := c.Query("name") // @source(deferred)
	defer func() {
		sys.Run("rm", name) // @sink(deferred)
	}()
}

// Stored passes only what the database gave: not a source.
func (h *Handlers) Stored(c *web.Context) {
	_ = c.Query("id")
	var users []struct{ Name string }
	h.DB.Find(&users)
	for _, u := range users {
		sys.Run("mail", u.Name) // @sink(none)
	}
}

// CtxRedirect and CtxSafeRedirect are the handlers of the redirect routes.
func (h *Handlers) CtxRedirect() web.HandlerFunc     { return h.ctxRedirect }
func (h *Handlers) CtxSafeRedirect() web.HandlerFunc { return h.ctxSafeRedirect }
