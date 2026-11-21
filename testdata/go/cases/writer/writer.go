package writer

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"example.com/cases/lib"
)

// What is written to a writer is the content of the writer (a step of its own below it), not the
// writer: a field of the writer (a file name) is not what was written to it, and a read of the whole
// writer, or of its content, still sees it (ADR-082).

// --- a writer of the project, called through the interface: the body reads its own field ---

type logFile struct{ name string }

func (w *logFile) Write(p []byte) (int, error) {
	lib.Sink(w.name)
	return len(p), nil
}

// no finding: the data written is not the name of the file
func FieldDecoy() {
	var out io.Writer = &logFile{name: "app.log"}
	_, _ = out.Write([]byte(lib.Source()))
}

type dataFile struct{ name string }

func (w *dataFile) Write(p []byte) (int, error) {
	lib.Sink(string(p)) // @sink(writer_data)
	return len(p), nil
}

func Data() {
	var out io.Writer = &dataFile{name: "app.log"}
	_, _ = out.Write([]byte(lib.Source())) // @source(writer_data)
}

// --- the content is read back: from the whole writer, and from the writer's own methods ---

func BufferString() {
	var b bytes.Buffer
	b.WriteString(lib.Source()) // @source(writer_buffer)
	lib.Sink(b.String())        // @sink(writer_buffer)
}

func BuilderPassed() {
	var sb strings.Builder
	sb.WriteString(lib.Source()) // @source(writer_builder)
	use(&sb)
}

func use(sb *strings.Builder) {
	lib.Sink(sb.String()) // @sink(writer_builder)
}

func Fprintf() {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s", lib.Source()) // @source(writer_fprintf)
	lib.Sink(b.String())                // @sink(writer_fprintf)
}

func WriteString() {
	var b bytes.Buffer
	_, _ = io.WriteString(&b, lib.Source()) // @source(writer_iowrite)
	lib.Sink(b.String())                    // @sink(writer_iowrite)
}

// the buffer is a field of a writer of the project: what is written is below the field
type wrap struct {
	name string
	buf  bytes.Buffer
}

func (w *wrap) Write(p []byte) (int, error) { return w.buf.Write(p) }

func (w *wrap) Out() string { return w.buf.String() }

func Wrapped() {
	w := &wrap{name: "x"}
	_, _ = w.Write([]byte(lib.Source())) // @source(writer_wrapped)
	lib.Sink(w.Out())                    // @sink(writer_wrapped)
}

// no finding: the name of the wrapper is not what was written
func WrappedDecoy() {
	w := &wrap{name: "x"}
	_, _ = w.Write([]byte(lib.Source()))
	lib.Sink(w.name)
}
