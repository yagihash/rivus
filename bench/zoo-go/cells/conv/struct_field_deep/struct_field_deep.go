// Package struct_field_deep is a cell of the zoo.
//
// @cell(F/conv/struct-field-deep)
// @shape: the request of a generated API (a pointer to a struct that holds a pointer to a struct that holds a oneof, an interface whose value is a wrapper) is given to the call, and the rule of the sink names the URL, five steps down (docs/DESIGN-field-sinks.md)
package struct_field_deep

import "zoo/mark"

func init() {
	mark.Case("F/conv/struct-field-deep/vuln", vuln)
	mark.Case("F/conv/struct-field-deep/fixed", fixed)
	mark.Case("F/conv/struct-field-deep/decoy", decoy)
}

func build(h *mark.HTTPRequest) *mark.CreateTask {
	return &mark.CreateTask{Task: &mark.Task{MessageType: &mark.TaskHTTPRequest{HTTPRequest: h}}}
}

func vuln() { // @vuln
	h := &mark.HTTPRequest{}
	h.URL = mark.Src("conv-struct-field-deep-vuln") // @source(conv-struct-field-deep-vuln)
	mark.SinkTask(build(h))                         // @sink(conv-struct-field-deep-vuln)
}

func fixed() { // @fixed
	h := &mark.HTTPRequest{}
	h.URL = mark.CleanCmd(mark.Src("conv-struct-field-deep-fixed"))
	mark.SinkTask(build(h)) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("conv-struct-field-deep-decoy") // @source(conv-struct-field-deep-decoy)
	h := &mark.HTTPRequest{URL: "https://example.com/"}
	h.Body = t
	mark.SinkTask(build(h)) // @sink(none) @fp @nosink(conv-struct-field-deep-decoy) @fp @ref(FIELD-SINK)
	mark.SinkCmd(t)         // @sink(conv-struct-field-deep-decoy)
}
