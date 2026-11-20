package api

import (
	"curated/orm"
	"curated/sys"
	"curated/web"
	"strings"
	"sync"
)

// Cache is a map kept for all requests, with a lock.
type Cache struct {
	mu sync.Mutex
	m  map[string]string
}

// Set keeps a value.
func (c *Cache) Set(k, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
}

// Get returns a value.
func (c *Cache) Get(k string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[k]
}

// Base is what the handlers of a group share.
type Base struct{ DB *orm.DB }

// Admin has a Base: Run is the method of Base.
type Admin struct{ Base }

// Run is promoted to Admin.
func (b *Base) Run(c *web.Context) {
	name := c.Query("name")                                    // @source(embed)
	b.DB.Exec("DELETE FROM users WHERE name = '" + name + "'") // @sink(embed)
}

// Generic passes the text through a generic function.
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// Upper uses the generic map.
func (b *Base) Upper(c *web.Context) {
	names := Map([]string{c.Query("a")}, strings.ToUpper)     // @source(generic)
	b.DB.Exec("SELECT * FROM t WHERE n = '" + names[0] + "'") // @sink(generic)
}

// Pipe sends the text over a channel to a goroutine.
func (b *Base) Pipe(c *web.Context) {
	ch := make(chan string, 1)
	ch <- c.Query("cmd") // @source(chan)
	go func() {
		sys.Run(<-ch) // @sink(chan)
	}()
}

// Keyed keeps the text under one key and uses another key (not the same value).
func (b *Base) Keyed(cache *Cache, c *web.Context) {
	cache.Set("name", c.Query("name"))
	b.DB.Exec("SELECT * FROM t WHERE n = '" + cache.Get("table") + "'") // @sink(none) @fp
}

// Guarded checks the text before it is used.
func (b *Base) Guarded(c *web.Context) {
	col := c.Query("col")
	switch col {
	case "name", "email":
		b.DB.Exec("SELECT " + col + " FROM users") // @sink(none) @fp
	}
}

// Both calls the sink with a constant and with the text, at two places.
func (b *Base) Both(c *web.Context) {
	sys.Run("uptime")     // @sink(none)
	arg := c.Query("arg") // @source(both)
	sys.Run("ls", arg)    // @sink(both)
}
