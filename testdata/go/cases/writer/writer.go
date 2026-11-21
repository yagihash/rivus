package writer

import (
	"bytes"
	"fmt"
	"io"
	"net/http/httptest"
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

// --- a writer of the project, written to by a function that does not call its Write: the data is in
// the fields the Write body stores it in (what the model does not apply the body of must not lose it) ---

type sliceW struct{ data []byte }

func (w *sliceW) Write(p []byte) (int, error) { w.data = append(w.data, p...); return len(p), nil }

func OwnFprintf() {
	w := &sliceW{}
	fmt.Fprintf(w, "%s", lib.Source()) // @source(writer_own_fprintf)
	lib.Sink(string(w.data))           // @sink(writer_own_fprintf)
}

type bufW struct{ buf bytes.Buffer }

func (w *bufW) Write(p []byte) (int, error) { return w.buf.Write(p) }

func OwnWriteString() {
	w := &bufW{}
	_, _ = io.WriteString(w, lib.Source()) // @source(writer_own_iowrite)
	lib.Sink(w.buf.String())               // @sink(writer_own_iowrite)
}

func Recorder() {
	rec := httptest.NewRecorder()
	fmt.Fprint(rec, lib.Source()) // @source(writer_recorder)
	lib.Sink(rec.Body.String())   // @sink(writer_recorder)
}

func BufferFprint() {
	var b bytes.Buffer
	fmt.Fprint(&b, lib.Source()) // @source(writer_buffer_fprint)
	lib.Sink(b.String())         // @sink(writer_buffer_fprint)
}

func OwnCopy() {
	w := &sliceW{}
	_, _ = io.Copy(w, strings.NewReader(lib.Source())) // @source(writer_own_copy)
	lib.Sink(string(w.data))                           // @sink(writer_own_copy)
}

// --- the writer is passed to a function as an interface (a pointer inside the interface value), and
// is written to there: the write must reach what the pointer points to (ADR-087) ---

func fill(w io.Writer, s string) { fmt.Fprint(w, s) }

func PassedBuffer() {
	var b bytes.Buffer
	fill(&b, lib.Source()) // @source(writer_passed_buffer)
	lib.Sink(b.String())   // @sink(writer_passed_buffer)
}

func PassedBuilder() {
	var sb strings.Builder
	fill(&sb, lib.Source()) // @source(writer_passed_builder)
	lib.Sink(sb.String())   // @sink(writer_passed_builder)
}

func PassedOwn() {
	w := &sliceW{}
	fill(w, lib.Source())    // @source(writer_passed_own)
	lib.Sink(string(w.data)) // @sink(writer_passed_own)
}

func fillf(w io.Writer, s string) { fmt.Fprintf(w, "%s", s) }

func PassedOwnFprintf() {
	w := &sliceW{}
	fillf(w, lib.Source())   // @source(writer_passed_own_f)
	lib.Sink(string(w.data)) // @sink(writer_passed_own_f)
}

func ws(w io.Writer, s string) { _, _ = io.WriteString(w, s) }

func PassedWriteString() {
	var b bytes.Buffer
	ws(&b, lib.Source()) // @source(writer_passed_ws)
	lib.Sink(b.String()) // @sink(writer_passed_ws)
}

// no finding: another buffer is read
func PassedDecoy() {
	var b, other bytes.Buffer
	fill(&b, lib.Source())
	lib.Sink(other.String()) // @sink(none)
}

// no finding: the writer is held by a pointer in a field of a context, and what is written to it is
// not what the context's other pointer (the request) holds
type reqT struct{ form string }

type ctxT struct {
	req  *reqT
	resp *sliceW
}

func PassedHeldDecoy() {
	c := &ctxT{req: &reqT{form: "x"}, resp: &sliceW{}}
	fill(c.resp, lib.Source())
	lib.Sink(c.req.form) // @sink(none)
}

func PassedHeld() {
	c := &ctxT{req: &reqT{form: "x"}, resp: &sliceW{}}
	fill(c.resp, lib.Source())    // @source(writer_passed_held)
	lib.Sink(string(c.resp.data)) // @sink(writer_passed_held)
}

// --- a method of the project called through an interface, and the original pointer read ---

type box struct{ v string }

func (b *box) Set(s string) { b.v = s }

type setter interface{ Set(string) }

func SetterWrite() {
	b := &box{}
	var s setter = b
	s.Set(lib.Source()) // @source(writer_setter)
	lib.Sink(b.v)       // @sink(writer_setter)
}

// no finding: another field of the box is read (the body of Set is read: only v is written)
type twoField struct{ v, name string }

func (t *twoField) Set(s string) { t.v = s }

func SetterFieldDecoy() {
	t := &twoField{}
	var s setter = t
	s.Set(lib.Source())
	lib.Sink(t.name) // @sink(none)
}

// no finding: another box
func SetterDecoy() {
	b, other := &box{}, &box{}
	var s setter = b
	s.Set(lib.Source())
	lib.Sink(other.v) // @sink(none)
}

func setVia(s setter, v string) { s.Set(v) }

func SetterPassed() {
	b := &box{}
	setVia(b, lib.Source()) // @source(writer_setter_passed)
	lib.Sink(b.v)           // @sink(writer_setter_passed)
}
