package main

import (
	"os"
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
	f, err := os.Open("../../examples/rules/rust-actix-web.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ex, err := model.Parse(f, "rust-actix-web.yaml")
	if err != nil {
		t.Fatal(err)
	}
	withModel := model.Merge(append(b, ex)...)

	note := func(p *ir.Program, r *model.Rules) string {
		var sb strings.Builder
		noteNotBuiltin(&sb, p, r)
		return sb.String()
	}
	if got := note(call("actix_web::HttpRequest::query_string", false), builtinOnly); !strings.Contains(got, "actix-web") || !strings.Contains(got, "examples/rules/rust-actix-web.yaml") {
		t.Errorf("a project that calls actix-web with no model: want a note, got %q", got)
	}
	if got := note(call("actix_web::HttpRequest::query_string", false), withModel); got != "" {
		t.Errorf("with the model given: want no note, got %q", got)
	}
	if got := note(call("actix_web::HttpRequest::query_string", true), builtinOnly); got != "" {
		t.Errorf("only a dependency calls actix-web: want no note, got %q", got)
	}
	if got := note(call("axum::extract::Query::foo", false), builtinOnly); got != "" {
		t.Errorf("another framework: want no note, got %q", got)
	}
}
