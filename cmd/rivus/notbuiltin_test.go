package main

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

func TestNoteNotBuiltin(t *testing.T) {
	call := func(sym string, dep bool) *ir.Program {
		return &ir.Program{Funcs: []*ir.Func{{Dep: dep, Blocks: []ir.Block{{Instrs: []ir.Instr{
			{Op: ir.OpCall, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: sym}},
		}}}}}}
	}
	b, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	builtinOnly := model.Merge(b...)
	note := func(p *ir.Program, r *model.Rules) string {
		var sb strings.Builder
		noteNotBuiltin(&sb, p, r)
		return sb.String()
	}

	// actix-web is built in: a project that calls it is told nothing.
	if got := note(call("actix_web::HttpRequest::query_string", false), builtinOnly); got != "" {
		t.Errorf("actix-web is built in: want no note, got %q", got)
	}

	// the mechanism, with a framework that is not built in (a made-up one, with a rule file of its own)
	saved := notBuiltinFrameworks
	defer func() { notBuiltinFrameworks = saved }()
	notBuiltinFrameworks = []notBuiltin{{"example-fw", "example_fw::", "examples/rules/example-fw.yaml", "the entry points of the handlers"}}
	withModel := model.Merge(append(b, &model.RuleSet{Name: "example/example-fw@1", Sinks: []model.Sink{{Symbol: "example_fw::Request::redirect"}}})...)

	if got := note(call("example_fw::Request::query", false), builtinOnly); !strings.Contains(got, "example-fw") || !strings.Contains(got, "examples/rules/example-fw.yaml") {
		t.Errorf("a project that calls a framework with no model: want a note, got %q", got)
	}
	if got := note(call("example_fw::Request::query", false), withModel); got != "" {
		t.Errorf("with the model given: want no note, got %q", got)
	}
	if got := note(call("example_fw::Request::query", true), builtinOnly); got != "" {
		t.Errorf("only a dependency calls the framework: want no note, got %q", got)
	}
	if got := note(call("axum::extract::Query::foo", false), builtinOnly); got != "" {
		t.Errorf("another framework: want no note, got %q", got)
	}
}
