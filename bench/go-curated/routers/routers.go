// Package routers registers the routes.
package routers

import (
	"curated/api"
	"curated/middleware"
	"curated/web"
)

// Register makes the routes of the program.
func Register(e *web.Engine, h *api.Handlers, b *api.Base, cache *api.Cache) {
	e.Use(middleware.Logger())

	users := e.Group("/users")
	users.GET("/direct", h.Direct)
	users.GET("/const", h.Constant)
	users.GET("/sanitized", h.Sanitized)
	users.GET("/numeric/:id", h.Numeric)
	users.GET("/search", h.ViaInterface)
	users.GET("/lookup/:name", h.ViaService)
	users.POST("/rename/:id", h.RenameUser)
	users.POST("/create", h.Create)
	users.POST("/create-model", h.CreateModel)

	login := e.Group("/login", middleware.RedirectTarget(), middleware.SafeTarget())
	login.GET("/go", h.CtxRedirect())
	login.GET("/go-safe", h.CtxSafeRedirect())

	authed := e.Group("/me", middleware.Auth("X-User"))
	authed.GET("/whoami", h.Whoami)

	tools := e.Group("/tools")
	tools.GET("/command/:op", h.Command)
	tools.GET("/closure", h.Closure("SELECT * FROM t WHERE x = "))
	tools.GET("/message", h.Message)
	tools.GET("/wrapped", h.Wrapped)
	tools.GET("/tags", h.Tags)
	tools.POST("/bound", h.Bound)
	tools.GET("/deferred", h.Deferred)
	tools.GET("/stored", h.Stored)
	tools.GET("/embed", b.Run)
	tools.GET("/upper", b.Upper)
	tools.GET("/pipe", b.Pipe)
	tools.GET("/keyed", func(c *web.Context) { b.Keyed(cache, c) })
	tools.GET("/guarded", b.Guarded)
	tools.GET("/both", b.Both)
	tools.GET("/options", h.Options)
	tools.GET("/jobs", h.Jobs)
	tools.GET("/tree", h.Tree)
	tools.GET("/builder", h.Builder)
	tools.GET("/parsed", h.Parsed)
	tools.GET("/fixed", h.Fixed)
	(&api.Admin{Base: *b}).Run(nil)
}
