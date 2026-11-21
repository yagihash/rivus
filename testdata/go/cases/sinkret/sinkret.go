package sinkret

import (
	"bytes"
	"io"

	"example.com/cases/lib"
)

// A function with no body in the code that has only a sink rule is not modelled
// by it: what goes in comes out, as for any other call, and the call is recorded
// as unmodelled (CORE-K).

func Open() {
	src := lib.Source() // @source(sinkret_open)
	f := lib.Open(src)  // @sink(sinkret_open)
	lib.Sink(f)         // @sink(sinkret_open)
}

func Prepare(db *lib.DB) {
	src := lib.Source()   // @source(sinkret_prepare)
	st := db.Prepare(src) // @sink(sinkret_prepare)
	lib.Sink(st)          // @sink(sinkret_prepare)
}

// io.WriteString has a model of the flow (and a sink rule for a ResponseWriter)
func WriteString() {
	var b bytes.Buffer
	io.WriteString(&b, lib.Source()) // @source(sinkret_ws)
	lib.Sink(b.String())             // @sink(sinkret_ws)
}
