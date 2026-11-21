package api

import (
	"curated/sys"
	"curated/web"
	"io"
)

// The writer of a rotating log: what is written to it is the data of the log, and its own fields (the name of
// the file it opens, or renames to) are not what was written. The model of io.Writer.Write puts the data in
// the content of the writer, a step below the writer: a body that reads its own field must not see it. A build
// that puts the data on the whole object makes every such field a flow (the +20 of Gitea that the light tier
// did not show: the writers of a project, called through io.Writer).

type rotatingLog struct{ filename string }

// Write opens the file by the name that the writer was made with.
func (w *rotatingLog) Write(p []byte) (int, error) {
	sys.Run(w.filename) // @sink(none)
	return len(p), nil
}

// LogRequest writes a request value to a writer that the caller does not know: the data is not the file name.
func LogRequest(c *web.Context) {
	var out io.Writer = &rotatingLog{filename: "app.log"}
	_, _ = out.Write([]byte(c.Query("msg")))
}

// dataLog is a writer whose body reads what was written: the flow is real.
type dataLog struct{ filename string }

// Write passes the data on.
func (w *dataLog) Write(p []byte) (int, error) {
	sys.Run(string(p)) // @sink(rotate_data)
	return len(p), nil
}

// LogData writes a request value to a writer that reads it.
func LogData(c *web.Context) {
	var out io.Writer = &dataLog{filename: "app.log"}
	_, _ = out.Write([]byte(c.Query("msg"))) // @source(rotate_data)
}

// rotating keeps the name of the file next to a content that it hands to another writer.
type rotating struct {
	filename string
	inner    io.Writer
}

// Write hands the data to the inner writer and renames by its own field.
func (w *rotating) Write(p []byte) (int, error) {
	n, err := w.inner.Write(p)
	sys.Run(w.filename) // @sink(none)
	return n, err
}

// LogRotating writes through the rotating writer.
func LogRotating(c *web.Context) {
	var out io.Writer = &rotating{filename: "app.log", inner: io.Discard}
	_, _ = out.Write([]byte(c.Query("msg")))
}
