package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

func TestSkippedBodyFindsACalleeThatTheExtractorCouldNotRead(t *testing.T) {
	e := &Engine{prog: &sg.Program{IR: &ir.Program{FnSkipped: []ir.FnSkipped{
		{Unit: "d", Func: "d::skipped", Dependency: true, Why: "w"},
		{Unit: "p", Func: "p::skipped", Why: "w"},
	}}}}
	if f := e.skippedBody([]string{"x", "d::skipped"}, ""); f == nil || !f.Dependency {
		t.Errorf("by the ID of a callee: %+v", f)
	}
	if f := e.skippedBody(nil, "p::skipped"); f == nil || f.Dependency {
		t.Errorf("by the symbol: %+v", f)
	}
	if f := e.skippedBody([]string{"d::read"}, "d::read"); f != nil {
		t.Errorf("a function that was read: %+v", f)
	}
	if (&Engine{prog: &sg.Program{IR: &ir.Program{}}}).skippedBody([]string{"a"}, "a") != nil {
		t.Error("nothing skipped")
	}
}
