// Package zoodep stands for a dependency of the zoo: a module of its own (go.mod, a replace in the zoo's),
// so that its functions are Dep functions: rivus reads their bodies as it reads those of a third-party
// package, and does not analyse them as the code under test.
package zoodep

import "io"

// Wrap returns a writer that hands what is written to w (the shape of bufio.Writer, io.MultiWriter and
// the compressing and logging wrappers: a dependency's Write that calls the Write of what it wraps).
func Wrap(w io.Writer) io.Writer { return &wrapper{w: w} }

type wrapper struct{ w io.Writer }

func (x *wrapper) Write(p []byte) (int, error) { return x.w.Write(p) }
