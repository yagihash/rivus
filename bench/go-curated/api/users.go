// Package api has the handlers.
package api

import (
	"curated/config"
	"curated/models"
	"curated/orm"
	"curated/services"
	"curated/valid"
	"curated/web"
	"fmt"
	"strconv"
)

// Handlers is what the routes call; its fields are set when the program starts.
type Handlers struct {
	DB       *orm.DB
	Svc      *services.Service
	Searcher []services.Searcher
}

// Direct passes a query parameter to the statement.
func (h *Handlers) Direct(c *web.Context) {
	name := c.Query("name")                                      // @source(direct)
	h.DB.Exec("SELECT * FROM users WHERE name = '" + name + "'") // @sink(direct)
}

// Constant uses the setting and a constant.
func (h *Handlers) Constant(c *web.Context) {
	_ = c.Query("name")
	h.DB.Exec("SELECT * FROM " + config.Default.Table)                      // @sink(none)
	h.DB.Exec("SELECT * FROM users LIMIT " + config.Default.Limits["rows"]) // @sink(none)
}

// Sanitized cleans the sort column.
func (h *Handlers) Sanitized(c *web.Context) {
	col := valid.Ident(c.Query("sort"))
	h.DB.Exec("SELECT * FROM users ORDER BY " + col) // @sink(none)
}

// Numeric turns the id into a number: a number cannot carry an injection.
func (h *Handlers) Numeric(c *web.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return
	}
	h.DB.Exec(fmt.Sprintf("SELECT * FROM users WHERE id = %d", id)) // @sink(none)
}

// ViaInterface asks the searchers; the first puts the text in the statement.
func (h *Handlers) ViaInterface(c *web.Context) {
	q := c.Query("q") // @source(iface_search)
	for _, s := range h.Searcher {
		s.Search(q)
	}
}

// ViaService goes through the service and the injected store.
func (h *Handlers) ViaService(c *web.Context) {
	name := c.Param("name") // @source(di_byname)
	h.Svc.Lookup(name)
}

// RenameUser passes the name (text) and the id (a number) to the store.
func (h *Handlers) RenameUser(c *web.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	h.Svc.Rename(id, c.PostForm("name"))
}

// Create fills a user (an alias of the model) from the form.
func (h *Handlers) Create(c *web.Context) {
	u := &services.User{}
	u.Name = c.PostForm("name") // @source(alias_user)
	u.Email = "none@example.com"
	services.CreateUser(h.DB, u)
}

// CreateModel fills the model type itself.
func (h *Handlers) CreateModel(c *web.Context) {
	u := &models.User{Name: "fixed", Email: "fixed@example.com"}
	_ = c.PostForm("name")
	services.CreateUser(h.DB, u)
}
