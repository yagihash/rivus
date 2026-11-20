package api

import (
	"curated/sys"
	"curated/web"
	"errors"
	"strings"
)

// Query is built with options, as gorm and xorm callers do.
type Query struct{ name, order string }

// Option sets a part of the query.
type Option func(*Query)

// WithName sets the name.
func WithName(n string) Option { return func(q *Query) { q.name = n } }

// WithOrder sets the order.
func WithOrder(o string) Option { return func(q *Query) { q.order = o } }

func build(opts ...Option) *Query {
	q := &Query{order: "id"}
	for _, o := range opts {
		o(q)
	}
	return q
}

// Options passes the name by an option; the order is a constant.
func (h *Handlers) Options(c *web.Context) {
	q := build(WithName(c.Query("name")), WithOrder("name")) // @source(options) @miss
	h.DB.Exec("SELECT * FROM t WHERE n = '" + q.name + "'")  // @sink(options)
	h.DB.Exec("SELECT * FROM t ORDER BY " + q.order)         // @sink(none)
}

// Job is a task that holds a function.
type Job struct {
	Name string
	Run  func(string)
}

// Jobs keeps a function in a field and calls it.
func (h *Handlers) Jobs(c *web.Context) {
	jobs := []Job{
		{Name: "rm", Run: func(a string) { sys.Run("rm", a) }}, // @sink(job)
		{Name: "nop", Run: func(a string) {}},
	}
	arg := c.Query("arg") // @source(job)
	for _, j := range jobs {
		j.Run(arg)
	}
}

// Node is a tree.
type Node struct {
	Val  string
	Kids []*Node
}

func walk(n *Node, prefix string, out *[]string) {
	if n == nil {
		return
	}
	*out = append(*out, prefix+n.Val)
	for _, k := range n.Kids {
		walk(k, prefix+"/", out)
	}
}

// Tree puts the text in a tree and walks it (a recursion).
func (h *Handlers) Tree(c *web.Context) {
	root := &Node{Val: "root", Kids: []*Node{{Val: c.Query("leaf")}}} // @source(tree)
	var all []string
	walk(root, "", &all)
	sys.Run("ls", all...) // @sink(tree)
}

// Builder writes the text to a builder.
func (h *Handlers) Builder(c *web.Context) {
	var sb strings.Builder
	sb.WriteString("SELECT * FROM t WHERE n = '")
	sb.WriteString(c.Query("n")) // @source(builder)
	sb.WriteString("'")
	h.DB.Exec(sb.String()) // @sink(builder)
}

func parse(s string) (string, error) {
	if s == "" {
		return "", errors.New("empty")
	}
	return strings.TrimSpace(s), nil
}

// Parsed takes the text from a function that returns it and an error.
func (h *Handlers) Parsed(c *web.Context) {
	name, err := parse(c.Query("n")) // @source(parsed)
	if err != nil {
		h.DB.Exec("SELECT 1 WHERE '" + err.Error() + "' = ''") // @sink(none)
		return
	}
	h.DB.Exec("SELECT * FROM t WHERE n = '" + name + "'") // @sink(parsed)
}

// Fixed uses a constant that is the same text as a parameter of another handler.
func (h *Handlers) Fixed(c *web.Context) {
	_ = c.Query("name")
	name := "admin"
	h.DB.Exec("SELECT * FROM t WHERE n = '" + name + "'") // @sink(none)
}
