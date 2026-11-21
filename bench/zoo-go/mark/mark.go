//go:build !zooexec

// Package mark is what the cases of the zoo call: the sources, the sinks and the
// sanitizers. This is the file that the analysis reads: the declarations only, with
// empty bodies, so that no state of the recording is seen by the analysis (every
// case would write to one global). rivus gets the meaning of each function from
// rules.yaml. The recording is in mark_exec.go (build tag zooexec).
package mark

// NewIn makes the In of a tag.
func NewIn(tag string) In { return In{} }

// Src returns the marker of the tag.
func Src(tag string) string { return "" }

// SrcBytes is Src as bytes.
func SrcBytes(tag string) []byte { return nil }

// The sinks of each class. They record what they got and execute nothing.
func SinkCmd(s string)          {}
func SinkCmdBytes(b []byte)     {}
func SinkSQL(s string)          {}
func SinkHTML(s string)         {}
func SinkPath(base, s string)   {}
func SinkAny(v any)             {}

// The sanitizers: each replaces the dangerous characters of its class by '_'.
func CleanCmd(s string) string  { return "" }
func CleanSQL(s string) string  { return "" }
func CleanHTML(s string) string { return "" }
func CleanPath(s string) string { return "" }

// Case registers a case of the zoo (the harness runs it).
func Case(id string, fn func()) {}
