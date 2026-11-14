// Package irshape holds the constructs the Go extractor must express in the
// IR (ADR-039). The tests read the IR of these functions.
package irshape

import (
	"fmt"
	"io"
	"strings"

	"example.com/cases/lib"
)

type T struct{ a, b string }

var G string

func Concat(id string) {
	q := "SELECT * FROM users WHERE id = " + id
	lib.Sink(q)
}

func Sprintf(id string) {
	lib.Sink(fmt.Sprintf("SELECT * FROM users WHERE id = %s", id))
}

func Builder(id string) {
	var b strings.Builder
	b.WriteString("SELECT * FROM users WHERE id = ")
	b.WriteString(id)
	lib.Sink(b.String())
}

func Fields(p *T) {
	p.a = lib.Source()
	lib.Sink(p.b)
}

func EscapingField(p *T) {
	q := &p.a
	*q = lib.Source()
}

func Elements(xs []string, m map[string]string) string {
	xs[0] = lib.Source()
	v, ok := m["k"]
	if ok {
		return v
	}
	return xs[1]
}

func ShortCircuit(a, b bool) bool { return a && b }

func Recovers() {
	defer func() { recover() }()
	panic("x")
}

func Counter() func() int {
	n := 0
	return func() int { n++; return n }
}

func SetG(s string) { G = s }
func GetG() string  { return G }

func Channels(ch chan string) string {
	ch <- lib.Source()
	select {
	case v := <-ch:
		return v
	default:
	}
	return <-ch
}

func Ranges(xs []string, m map[string]int) {
	for _, x := range xs {
		lib.Sink(x)
	}
	for k := range m {
		lib.Sink(k)
	}
}

func Asserts(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return v.(string)
}

func Dynamic(r io.Reader, f func(string), buf []byte) {
	r.Read(buf)
	f("x")
	go f("y")
}

func Generic[S ~[]E, E any](s S) E { return s[0] }

func UseGeneric() string { return Generic([]string{"a"}) }

func Straight() {
	x := lib.Source()
	lib.Sink(x)
}

func Loop(n int) {
	s := lib.Source()
	for i := 0; i < n; i++ {
		s = s + "x"
	}
	lib.Sink(s)
}

func Zero() {
	var s string
	lib.Sink(s)
}

func Either(c bool) {
	s := "a"
	if c {
		s = lib.Source()
	}
	lib.Sink(s)
}

func AddrTaken() {
	s := ""
	p := &s
	*p = lib.Source()
	lib.Sink(s)
}
