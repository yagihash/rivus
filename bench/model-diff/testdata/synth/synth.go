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
