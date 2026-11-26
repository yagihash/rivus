//go:build !zooexec

// Package mark is what the cases of the zoo call: the sources, the sinks and the
// sanitizers. This is the file that the analysis reads: the declarations only, with
// empty bodies, so that no state of the recording is seen by the analysis (every
// case would write to one global). rivus gets the meaning of each function from
// rules.yaml. The recording is in mark_exec.go (build tag zooexec).
package mark

import "net/http"

// NewIn makes the In of a tag.
func NewIn(tag string) In { return In{} }

// Src returns the marker of the tag.
func Src(tag string) string { return "" }

// SrcBytes is Src as bytes.
func SrcBytes(tag string) []byte { return nil }

// SrcB is Src for a place that takes a function with a result of bool (the function of slices.IndexFunc).
func SrcB(tag string) bool { return false }

// The sinks of each class. They record what they got and execute nothing.
func SinkCmd(s string) {}

// SinkCmdB is SinkCmd for a place that takes a function with a result of bool (the function of slices.IndexFunc).
func SinkCmdB(s string) bool { return false }

func SinkCmdBytes(b []byte)   {}
func SinkSQL(s string)        {}
func SinkHTML(s string)       {}
func SinkPath(base, s string) {}
func SinkAny(v any)           {}

// The sanitizers: each replaces the dangerous characters of its class by '_'.
func CleanCmd(s string) string  { return "" }
func CleanSQL(s string) string  { return "" }
func CleanHTML(s string) string { return "" }
func CleanPath(s string) string { return "" }

// CleanURL is the sanitizer of the URL sinks (ssrf, open_redirect).
func CleanURL(s string) string { return "" }

// CleanResource is the sanitizer for the name of a cloud resource (a bucket, an object): the zoo has no judgement for that kind of sink, so
// it does what CleanCmd does when it runs, and the rules say that it neutralises resource_name.
func CleanResource(s string) string { return "" }

// Case registers a case of the zoo (the harness runs it).
func Case(id string, fn func()) {}

// SinkStmt runs the statement: the SQL of it is the sink, the Params are not (types.go).
func SinkStmt(s *Stmt) {}

// SinkStmtAny is SinkStmt for a statement that is held by an interface (the pointer is in it).
func SinkStmtAny(v any) {}

// SinkTask sends the request of the task: the URL of the HTTP request is the sink.
func SinkTask(r *CreateTask) {}

// Opaque is a function that the analysis has no body and no model of (the declaration has no body: opaque.s
// makes that legal): it moves the "body" parameter of a statement to its SQL (mark_exec.go does so).
func Opaque(s *Stmt)

// Transport is the transport of the clients of the cells that send a request (net/http): it makes no connection, and
// the run records the URL of the request as a sink (SinkAny) at the line of the call that sent it.
type Transport struct{}

// RoundTrip is the body that the analysis reads: empty (the meaning of the sink is in rules.yaml and the run).
func (Transport) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
