// Package middleware has the handlers that run before the others.
package middleware

import (
	"curated/valid"
	"curated/web"
)

// RedirectTarget keeps the "next" parameter for the handler that logs in.
func RedirectTarget() web.HandlerFunc {
	return func(c *web.Context) {
		c.Set("next", c.Query("next")) // @source(ctxkey)
		c.Next()
	}
}

// SafeTarget keeps the "next" parameter after it is cleaned.
func SafeTarget() web.HandlerFunc {
	return func(c *web.Context) {
		c.Set("safe_next", valid.Ident(c.Query("next")))
		c.Next()
	}
}

// Auth sets the user name from a header; the handler after it uses it.
func Auth(header string) web.HandlerFunc {
	return func(c *web.Context) {
		name := c.GetHeader(header) // @source(mw_user)
		if name == "" {
			c.Abort()
			return
		}
		c.Set("user", name)
		c.Next()
	}
}

// Logger is a middleware that touches nothing.
func Logger() web.HandlerFunc {
	return func(c *web.Context) {
		c.Next()
		c.JSON(200, nil)
	}
}
