package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
)

// The ids of the rule table are what `adopt_rules:` names: renaming one breaks the decisions that name it. A change of this list is a decision of its own.
func TestRuleIDsAreStable(t *testing.T) {
	want := []string{
		"sink.arg-word", "sink.name-of-resource", "sink.file-name", "sink.bytes-word", "sink.struct-field", "sink.format-values", "sink.arg-type",
		"source.field-word", "source.result-word", "sanitizer.name", "calls.func-param",
	}
	if got := RuleIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the rule ids changed:\n got %v\nwant %v", got, want)
	}
	seen := map[string]bool{}
	for _, r := range ruleTable {
		if seen[r.ID] || !strings.HasPrefix(r.ID, r.Kind+".") || r.Text == "" {
			t.Errorf("bad rule %+v", r)
		}
		seen[r.ID] = true
	}
}

// What the loader lets a decision name is the sink and source rules of the table, and nothing else.
func TestAdoptableRuleIDsAreInTheTable(t *testing.T) {
	var want []string
	for _, r := range ruleTable {
		if r.Kind == KindSink || r.Kind == KindSource {
			want = append(want, r.ID)
		}
	}
	if !reflect.DeepEqual(want, model.AdoptRuleIDs) {
		t.Fatalf("the loader accepts %v, the table has %v", model.AdoptRuleIDs, want)
	}
}

func TestClaimsOfOneDecisionPassAsTheyAre(t *testing.T) {
	merged, warns, err := ResolveClaims([]Claim{
		{"d1", "b.F", "arg1", "ssrf"},
		{"d1", "a.F", "arg0", "ssrf"},
		{"d1", "a.F", "arg0", "ssrf"}, // the same decision twice: once
	})
	if err != nil || len(warns) != 0 {
		t.Fatal(err, warns)
	}
	want := []Merged{{"a.F", "arg0", "ssrf", []string{"d1"}}, {"b.F", "arg1", "ssrf", []string{"d1"}}}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("%+v", merged)
	}
}

func TestClaimsOfTheSameCategoryAreMergedWithAWarning(t *testing.T) {
	merged, warns, err := ResolveClaims([]Claim{
		{"first", "a.F", "arg0", "resource_name"},
		{"second", "a.F", "arg0", "resource_name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 || !reflect.DeepEqual(merged[0].Decisions, []string{"first", "second"}) {
		t.Fatalf("%+v", merged)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "first and second") {
		t.Fatalf("%v", warns)
	}
}

func TestClaimsOfDifferentCategoriesFail(t *testing.T) {
	merged, _, err := ResolveClaims([]Claim{
		{"d1", "a.F", "arg0", "ssrf"},
		{"d2", "a.F", "arg0", "resource_name"},
		{"d2", "a.F", "arg1", "resource_name"}, // no conflict: another position
		{"d3", "a.F", "arg0", "ssrf"},
	})
	var ce *ConflictError
	if !errors.As(err, &ce) || len(ce.Conflicts) != 1 {
		t.Fatalf("%v", err)
	}
	c := ce.Conflicts[0]
	if c.Symbol != "a.F" || c.Position != "arg0" || !reflect.DeepEqual(c.Decisions, []string{"d1", "d2", "d3"}) || !reflect.DeepEqual(c.Categories, []string{"resource_name", "ssrf"}) {
		t.Fatalf("%+v", c)
	}
	for _, s := range []string{"d1, d2, d3", "a human decides", "categories", "exit 70"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("%q is not in %q", s, err)
		}
	}
	if len(merged) != 1 || merged[0].Position != "arg1" {
		t.Fatalf("%+v", merged)
	}
}

// The result does not depend on the order of the claims.
func TestClaimsAreOrderFree(t *testing.T) {
	a := []Claim{{"d1", "a.F", "arg0", "x"}, {"d2", "b.F", "arg0", "y"}, {"d3", "a.F", "arg1", "x"}}
	b := []Claim{a[2], a[1], a[0]}
	m1, _, _ := ResolveClaims(a)
	m2, _, _ := ResolveClaims(b)
	if !reflect.DeepEqual(m1, m2) {
		t.Fatalf("%+v\n%+v", m1, m2)
	}
}

// A merged row names all the decisions, in dictionary order, whatever the order of the claims.
func TestMergedRowNamesAllDecisionsSorted(t *testing.T) {
	merged, warns, err := ResolveClaims([]Claim{
		{"zeta", "a.F", "arg0", "x"}, {"alpha", "a.F", "arg0", "x"}, {"mid", "a.F", "arg0", "x"}, {"alpha", "a.F", "arg0", "x"},
	})
	if err != nil || len(merged) != 1 || !reflect.DeepEqual(merged[0].Decisions, []string{"alpha", "mid", "zeta"}) {
		t.Fatalf("%v %+v", err, merged)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "alpha and mid and zeta") {
		t.Fatalf("%v", warns)
	}
}
