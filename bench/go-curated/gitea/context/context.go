// Package context is the request context of a Gitea-shaped handler: the request
// of net/http is inside it, and the handlers take it as their first parameter.
package context

import (
	"net/http"

	"curated/gitea/models"
)

// Context is what a handler gets.
type Context struct {
	Req  *http.Request
	Doer *models.User
	Out  []string
}

// JSONRedirect and ServerError answer; they are not sinks.
func (c *Context) JSONRedirect(to string)  { c.Out = append(c.Out, "redirect") }
func (c *Context) ServerError(err error)   { c.Out = append(c.Out, "error") }
func (c *Context) JSONErrorAuto(err error) { c.Out = append(c.Out, "error") }
