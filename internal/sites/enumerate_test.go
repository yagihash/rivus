package sites

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
)

func TestUncoveredDropsTheSinkThatTheInterfaceSaysTheSameOf(t *testing.T) {
	own := []model.Sink{{Symbol: "I.m", Args: []int{0}, Category: "ssrf", Accepts: []string{"user_input"}}}
	same := model.Sink{Symbol: "C.m", Args: []int{0}, Category: "ssrf", Accepts: []string{"user_input"}}
	other := model.Sink{Symbol: "C.m", Args: []int{1}, Category: "ssrf", Accepts: []string{"user_input"}}
	got := uncovered([]model.Sink{same, other}, own)
	if len(got) != 1 || got[0].Args[0] != 1 {
		t.Fatalf("uncovered = %+v, want only the sink of another position", got)
	}
	if got := uncovered([]model.Sink{same}, nil); len(got) != 1 {
		t.Fatalf("with no sink on the interface the sink of the implementation stays: %+v", got)
	}
}

func TestResolveNamedFindsTheParameterByItsName(t *testing.T) {
	// a call of a method: the receiver, a positional argument, then the named ones by name
	in := ir.Instr{Op: ir.OpCall, Args: []ir.LocalID{0, 1, 2, 3}, ArgNames: []string{"", "", "kind", "limit"}}
	known := namedNames{"C.m": {"kind": true, "limit": true}}
	sink := func(names ...string) model.Sink { return model.Sink{Symbol: "C.m", Named: names, Category: "c"} }

	got, ok := resolveNamed(sink("limit", "kind"), in, true, known, []string{"C.m"})
	if !ok || len(got.Args) != 2 || got.Args[0] != 2 || got.Args[1] != 1 || len(got.NamedUnresolved) != 0 {
		t.Fatalf("named: [limit, kind] = %+v, %v; want the positions 2 and 1 (the receiver is not counted)", got, ok)
	}
	if _, ok := resolveNamed(sink("other"), in, true, namedNames{"C.m": {"kind": true, "other": true}}, []string{"C.m"}); ok {
		t.Fatal("a name that the call does not pass (another call has it) is no site")
	}
	got, ok = resolveNamed(sink("kind", "kidn"), in, true, known, []string{"C.m"})
	if !ok || len(got.Args) != 0 || len(got.NamedUnresolved) != 1 || got.NamedUnresolved[0] != "kidn" {
		t.Fatalf("a name no call has = %+v, %v; want the sink read whole, with the name said", got, ok)
	}
	part := model.Sink{Symbol: "C.m", Category: "c", Parts: []model.SinkPart{{Named: "kind", Field: "url"}}}
	got, ok = resolveNamed(part, in, true, known, []string{"C.m"})
	if !ok || len(got.Parts) != 1 || got.Parts[0].Arg == nil || *got.Parts[0].Arg != 1 || got.Parts[0].Field != "url" {
		t.Fatalf("a part by name = %+v, %v", got, ok)
	}
	if got, ok := resolveNamed(model.Sink{Symbol: "C.m", Args: []int{0}}, in, true, known, nil); !ok || len(got.Args) != 1 {
		t.Fatalf("a sink without a name is as it was: %+v, %v", got, ok)
	}
}

func TestByCategoryKeepsTheOrderAndTheRowsOfACategoryTogether(t *testing.T) {
	got := byCategory([]model.Sink{{Category: "a", Args: []int{0}}, {Category: "b", Args: []int{1}}, {Category: "a", Args: []int{1}}})
	if len(got) != 2 || len(got[0]) != 2 || got[0][0].Category != "a" || got[1][0].Category != "b" {
		t.Fatalf("byCategory = %+v", got)
	}
	// the same positions under another category: one sink, as it was
	got = byCategory([]model.Sink{{Category: "a", Args: []int{1}}, {Category: "b", Args: []int{1}}})
	if len(got) != 1 || len(got[0]) != 2 || got[0][0].Category != "a" {
		t.Fatalf("the same positions are one site: %+v", got)
	}
}
