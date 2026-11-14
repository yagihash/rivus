package uncertain

import (
	"reflect"

	"example.com/cases/lib"
)

func ViaChannel() {
	s := lib.Source() // @source(chan)
	ch := make(chan string, 1)
	ch <- s
	lib.Sink(<-ch) // @sink(chan)
}

func ViaGoroutine() {
	s := lib.Source() // @source(goroutine)
	done := make(chan struct{})
	go func() {
		lib.Sink(s) // @sink(goroutine)
		close(done)
	}()
	<-done
}

func ViaSelect() {
	s := lib.Source() // @source(select)
	ch := make(chan string, 1)
	ch <- s
	select {
	case v := <-ch:
		lib.Sink(v) // @sink(select)
	default:
	}
}

func ViaSlice() {
	s := lib.Source() // @source(slice)
	parts := []string{s}
	lib.Sink(parts[0]) // @sink(slice)
}

var shared string

func SetShared(s string) { shared = s }

func FillShared() {
	s := lib.Source() // @source(global)
	SetShared(s)
}

func UseShared() {
	lib.Sink(shared) // @sink(global)
}

func ViaReflect() {
	s := lib.Source() // @source(reflect)
	v := reflect.ValueOf(s)
	lib.Sink(v.String()) // @sink(reflect)
}

func Plain() {
	s := lib.Source() // @source(plain)
	lib.Sink(s)       // @sink(plain)
}

// Opaque has no implementation in the loaded code, like most interfaces of
// the standard library seen from user code.
type Opaque interface{ Do(s string) string }

func ViaOpaque(o Opaque) {
	s := lib.Source() // @source(opaque)
	lib.Sink(o.Do(s)) // @sink(opaque)
}
