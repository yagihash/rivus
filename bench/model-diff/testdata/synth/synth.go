// Package synth is a library for the tests of model-diff: every function does something a model can get right or wrong.
package synth

import "errors"

func Join(a, b string) string { return a + b }

func Upper(s string) string { return s + "!" }

// Drop ignores its argument: a model that says the argument reaches the result is wider than the body.
func Drop(s string) string { _ = s; return "x" }

// Copy writes the source into dst.
func Copy(dst []byte, src string) int { return copy(dst, src) }

// Split puts its input in the error.
func Split(s string) (string, error) { return s, errors.New(s) }

type Box struct{ v string }

func NewBox(s string) *Box { return &Box{v: s} }

func (b *Box) Set(s string) { b.v = s }

func (b *Box) Get() string { return b.v }

func Apply(f func(string) string, s string) string { return f(s) }

func Map[T any](x T) T { return x }

// Run and Query are sinks (they do nothing here: a sink of a dependency is not read).
func Run(cmd string, args []string) {}

func Query(q, param string) string { return q }

type Opts struct{ URL, Name string }

func Open(o *Opts) {}

// Walk calls fn with root, as filepath.Walk does.
func Walk(root string, fn func(path string) error) error { return fn(root) }

// Esc and Pair are what a sanitizer row is read against: Esc carries its argument to the result, Pair carries both.
func Esc(s string) string { return s + "&amp;" }

func Pair(a, b string) string { return a + b }

// Read returns data; Fetch takes a struct that the source cannot be put into.
func Read() string { return "data" }

func Fetch(o *Opts, s string) string { return s }

// Req holds text in an exported field, and Hidden only in an unexported one: the source can be put into a Req (the field Path), not
// into a Hidden.
type Req struct {
	Path string
	N    int
}

func (r *Req) Clean() string { return r.Path }

func Clone(r *Req) string { return r.Path }

type Hidden struct{ v string }

func (h *Hidden) Val() string { return h.v }

func Use(h *Hidden, s string) string { return s }

// Name does not read the field that the source is put into.
func (r *Req) Name() string { return "req" }
