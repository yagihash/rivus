package model

import (
	"fmt"
	"strings"
	"testing"
)

type fakeResolver map[string][]string

func (f fakeResolver) Implementations(symbol string) ([]string, error) {
	impls, ok := f[symbol]
	if !ok {
		return nil, fmt.Errorf("%s is not an interface method", symbol)
	}
	return impls, nil
}

func mustParse(t *testing.T, src string) *Rules {
	t.Helper()
	rs, err := Parse(strings.NewReader(src), "test")
	if err != nil {
		t.Fatal(err)
	}
	return Merge(rs)
}

func TestExpandSinkOverImplementations(t *testing.T) {
	r := mustParse(t, `
sinks:
  - symbol: (p.Writer).Write
    match: implementations
    category: out
`)
	got := r.Expand(fakeResolver{"(p.Writer).Write": {"(*p.A).Write", "(p.B).Write"}}, 0)
	if len(got) != 1 || got[0].Count != 2 || got[0].Limited {
		t.Fatalf("expansions = %+v", got)
	}
	for _, sym := range []string{"(*p.A).Write", "(p.B).Write"} {
		sinks := r.Sinks[sym]
		if len(sinks) != 1 || sinks[0].Category != "out" || sinks[0].Match != MatchExact {
			t.Errorf("%s: sinks = %+v", sym, sinks)
		}
	}
	// The rule for the interface method itself stays: it matches the calls
	// made through the interface.
	if len(r.Sinks["(p.Writer).Write"]) != 1 {
		t.Error("the interface method rule must stay")
	}
}

func TestExpandStopsAtTheLimit(t *testing.T) {
	r := mustParse(t, `
sinks:
  - symbol: (p.I).M
    match: implementations
    max_implementations: 2
`)
	got := r.Expand(fakeResolver{"(p.I).M": {"(p.A).M", "(p.B).M", "(p.C).M"}}, 100)
	if len(got) != 1 || !got[0].Limited || got[0].Count != 3 || got[0].Max != 2 {
		t.Fatalf("expansions = %+v", got)
	}
	// No partial expansion: a half expanded rule would hide which half.
	for _, sym := range []string{"(p.A).M", "(p.B).M", "(p.C).M"} {
		if len(r.Sinks[sym]) != 0 {
			t.Errorf("%s was expanded although the limit was exceeded", sym)
		}
	}
}

func TestExpandUsesTheDefaultLimit(t *testing.T) {
	r := mustParse(t, "sinks:\n  - symbol: (p.I).M\n    match: implementations\n")
	got := r.Expand(fakeResolver{"(p.I).M": {"(p.A).M", "(p.B).M"}}, 1)
	if !got[0].Limited || got[0].Max != 1 {
		t.Fatalf("the default limit was not applied: %+v", got)
	}
}

func TestExpandReportsRulesThatCannotBeExpanded(t *testing.T) {
	r := mustParse(t, "sinks:\n  - symbol: p.NotAnInterface\n    match: implementations\n")
	got := r.Expand(fakeResolver{}, 0)
	if len(got) != 1 || got[0].Note == "" || got[0].Count != 0 {
		t.Fatalf("expansions = %+v", got)
	}
}

func TestSanitizerExpansionNeedsExplicitPermission(t *testing.T) {
	src := "sanitizers:\n  - symbol: (p.I).Clean\n    drops: [x]\n    match: implementations\n"
	if _, err := Parse(strings.NewReader(src), "t"); err == nil {
		t.Fatal("a sanitizer must not be expanded without allow_sanitizer_expansion")
	}
	r := mustParse(t, src+"    allow_sanitizer_expansion: true\n")
	r.Expand(fakeResolver{"(p.I).Clean": {"(p.A).Clean"}}, 0)
	if len(r.Sanitizers["(p.A).Clean"]) != 1 {
		t.Error("an allowed sanitizer expansion did not happen")
	}
}

func TestExpandCoversSourcesAndPropagators(t *testing.T) {
	r := mustParse(t, `
sources:
  - symbol: (p.I).Read
    labels: [user_input]
    match: implementations
propagators:
  - symbol: (p.I).Copy
    flows: [{from: arg0, to: result}]
    match: implementations
`)
	r.Expand(fakeResolver{"(p.I).Read": {"(p.A).Read"}, "(p.I).Copy": {"(p.A).Copy"}}, 0)
	if len(r.CallSources["(p.A).Read"]) != 1 || len(r.Propagators["(p.A).Copy"]) != 1 {
		t.Errorf("sources/propagators not expanded: %+v %+v", r.CallSources, r.Propagators)
	}
}

// Two interfaces that one type implements both copy their alias model to its method: the same
// argument is not a conflict, and a conflict does not take the alias models of other functions
// with it (yagihash/rivus #71).
func TestAliasModelsSurviveAnExpansionThatCopiesThemTwice(t *testing.T) {
	r := mustParse(t, `
propagators:
  - symbol: (m.A).With
    flows: [{from: recv, to: result, alias: true}]
    match: implementations
  - symbol: (m.B).With
    flows: [{from: recv, to: result, alias: true}]
    match: implementations
  - symbol: other.Unrelated
    flows: [{from: arg0, to: result, alias: true}]
`)
	r.Expand(fakeResolver{"(m.A).With": {"(*m.T).With"}, "(m.B).With": {"(*m.T).With"}}, 0)
	if err := r.Validate(); err != nil {
		t.Errorf("the same argument twice is a conflict: %v", err)
	}
	aliases, _ := r.AliasArgs()
	if _, ok := aliases["other.Unrelated"]; !ok || len(aliases) == 0 {
		t.Errorf("aliases = %v", aliases)
	}

	// different arguments are a conflict, and the other symbols keep theirs
	r = mustParse(t, `
propagators:
  - symbol: (m.A).With
    flows: [{from: arg0, to: result, alias: true}]
    match: implementations
  - symbol: (m.B).With
    flows: [{from: arg1, to: result, alias: true}]
    match: implementations
  - symbol: other.Unrelated
    flows: [{from: arg0, to: result, alias: true}]
`)
	r.Expand(fakeResolver{"(m.A).With": {"(*m.T).With"}, "(m.B).With": {"(*m.T).With"}}, 0)
	if err := r.Validate(); err == nil {
		t.Error("two different arguments for one function pass the validation after the expansion")
	}
	aliases, _ = r.AliasArgs()
	if _, ok := aliases["other.Unrelated"]; !ok {
		t.Errorf("an alias model of another function is lost: %v", aliases)
	}
}
