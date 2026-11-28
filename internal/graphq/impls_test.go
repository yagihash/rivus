package graphq_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphq"
)

// the program of the tests of impls and dispatchers
//
//	P: app.I.M (X.M, Y.M), app.I.M (X.M, Z.M), app.I.M (no candidate), app.J.M (X.M), a function value (X.M)
//	Q: other.K.N (Y.M);  R: app.K.N (Y.M);  S: app.Z.Zero (no candidate)
func implProgram() []fnDef {
	return []fnDef{
		{"P", []call{
			{"p.go", 1, 2, "dyn", "app.I.M", []string{"X.M", "Y.M"}},
			{"p.go", 2, 2, "dyn", "app.I.M", []string{"X.M", "Z.M"}},
			{"p.go", 3, 2, "dyn", "app.I.M", nil},
			{"p.go", 4, 2, "dyn", "app.J.M", []string{"X.M"}},
			{"p.go", 5, 2, "val", "f", []string{"X.M"}},
		}},
		{"X.M", nil},
		{"Y.M", nil},
		{"Z.M", nil},
		{"Q", []call{{"q.go", 1, 2, "dyn", "other.K.N", []string{"Y.M"}}}},
		{"R", []call{{"q.go", 2, 2, "dyn", "app.K.N", []string{"Y.M"}}}},
		{"S", []call{{"q.go", 3, 2, "dyn", "app.Z.Zero", nil}}},
	}
}

func implEngine(t *testing.T) *graphq.Engine { return build(t, implProgram()) }

func implIDs(r *graphq.ImplsResult) map[string]int {
	m := map[string]int{}
	for _, i := range r.Impls {
		m[i.ID] = i.CallSites
	}
	return m
}

func TestImplsIsTheUnionOfTheCandidatesOfTheCallSites(t *testing.T) {
	r, err := implEngine(t).Impls("app.I.M", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := implIDs(r)
	if len(got) != 3 || got["X.M"] != 2 || got["Y.M"] != 1 || got["Z.M"] != 1 {
		t.Errorf("impls: %v", got)
	}
	if r.Method != "app.I.M" || r.CallSites != 3 || r.ZeroCandidateSites != 1 || r.ImplsTotal != 3 {
		t.Errorf("counts: %+v", r)
	}
	if !strings.Contains(r.Basis, "union") || !strings.Contains(r.Basis, "not every type") {
		t.Errorf("basis: %q", r.Basis)
	}
	if r.Relation != "calls" || r.Flows != "not_checked" || r.Filters == nil || r.Truncated.Unit != "impls" || r.Truncated.Count != 0 {
		t.Errorf("common: %+v", r.Common)
	}
	if r.Impls[0].ID != "X.M" {
		t.Errorf("the most called first: %v", r.Impls)
	}
	hasZero := false
	for _, h := range r.Hints {
		hasZero = hasZero || strings.Contains(h, "no candidate")
	}
	if !hasZero {
		t.Errorf("hints do not mention the call sites without candidate: %v", r.Hints)
	}
	// the call of a function value is not a call through an interface method
	if _, ok := got["f"]; ok {
		t.Errorf("%v", got)
	}
}

func TestImplsShortNamesAndChoices(t *testing.T) {
	e := implEngine(t)
	r, err := e.Impls("I.M", graphq.Options{})
	if err != nil || r.Method != "app.I.M" || !strings.Contains(r.Query.Resolved, "end of it") {
		t.Fatalf("%v %+v", err, r.Query)
	}
	// two interface methods end with "M", and two with "K.N"
	for spec, want := range map[string]int{"M": 2, "N": 2, "K.N": 2} {
		r, err := e.Impls(spec, graphq.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if r.AmbiguousTotal != want || len(r.Ambiguous) != want || len(r.Impls) != 0 {
			t.Errorf("%s: %+v", spec, r.Choices)
		}
		if c := r.Ambiguous[0].(graphq.MethodChoice); c.CallSites == 0 {
			t.Errorf("%+v", c)
		}
	}
	// qualified, it is one
	if r, err := e.Impls("other.K.N", graphq.Options{}); err != nil || r.Method != "other.K.N" || len(r.Impls) != 1 {
		t.Errorf("%v %+v", err, r)
	}
}

func TestImplsLimitSaysTheTotalAndTheMakeUp(t *testing.T) {
	r, err := implEngine(t).Impls("app.I.M", graphq.Options{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Impls) != 1 || r.ImplsTotal != 3 || r.Truncated.Count != 2 || r.Truncated.Limit != 1 {
		t.Errorf("%+v %+v", r.Impls, r.Truncated)
	}
	n := 0
	for _, p := range r.ByPackage {
		n += p.Count
	}
	if n+r.ByPackageOther != 3 {
		t.Errorf("by_package does not count all of them: %+v", r.ByPackage)
	}
}

func TestImplsWithFilters(t *testing.T) {
	e := implEngine(t)
	r, err := e.Impls("app.I.M", graphq.Options{Avoid: []string{"Y.M"}})
	if err != nil {
		t.Fatal(err)
	}
	got := implIDs(r)
	if len(got) != 2 || got["Y.M"] != 0 || len(r.Filters) != 1 || !strings.Contains(r.Filters[0], "avoid") {
		t.Errorf("%v %v", got, r.Filters)
	}
	// the call sites of the method are still those of the graph
	if r.CallSites != 3 || r.ZeroCandidateSites != 1 {
		t.Errorf("%+v", r)
	}
	// avoiding the function that holds the calls leaves nothing, and says so
	r, err = e.Impls("app.I.M", graphq.Options{Avoid: []string{"P"}})
	if err != nil || len(r.Impls) != 0 || len(r.Filters) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	if _, err := e.Impls("app.I.M", graphq.Options{Via: []string{"P"}}); err == nil {
		t.Error("--via is not for impls")
	}
}

func TestImplsWithNothingToListSaysWhy(t *testing.T) {
	e := implEngine(t)
	// a method whose call sites all have no candidate
	r, err := e.Impls("app.Z.Zero", graphq.Options{})
	if err != nil || len(r.Impls) != 0 || r.CallSites != 1 || r.ZeroCandidateSites != 1 || r.Reason == "" {
		t.Errorf("%v %+v", err, r)
	}
	// a function that no call through an interface names: no error, and the reason
	r, err = e.Impls("X.M", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Impls) != 0 || r.CallSites != 0 || r.Reason == "" || !strings.Contains(strings.Join(r.Hints, " "), "dispatchers") {
		t.Errorf("%+v", r)
	}
	var se *graphq.SpecError
	if _, err := e.Impls("No.Such", graphq.Options{}); !errors.As(err, &se) {
		t.Errorf("an unknown name: %v", err)
	}
}

func TestDispatchers(t *testing.T) {
	e := implEngine(t)
	r, err := e.Dispatchers("X.M", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.CallSitesTotal != 4 || len(r.CallSites) != 4 || r.Truncated.Count != 0 || len(r.Start) != 1 {
		t.Fatalf("%+v", r)
	}
	// the fewest candidates first; each with its place, its function, its kind and the interface method
	first := r.CallSites[0]
	if first.Candidates != 1 || first.In != "P" || first.At != "p.go:4:2" || first.Kind != "interface" || first.Callee != "app.J.M" {
		t.Errorf("%+v", first)
	}
	byKind := map[string]int{}
	for _, s := range r.CallSites {
		byKind[s.Kind]++
	}
	if byKind["interface"] != 3 || byKind["value"] != 1 {
		t.Errorf("%v", byKind)
	}
	counts := map[string]int{}
	for _, c := range r.ByCallee {
		counts[c.Callee] = c.CallSites
	}
	if counts["app.I.M"] != 2 || counts["app.J.M"] != 1 {
		t.Errorf("%v", counts)
	}
	if !strings.Contains(r.Basis, "candidates") || r.Relation != "calls" || r.Flows != "not_checked" {
		t.Errorf("%+v", r.Common)
	}
	// limit, and the total
	r, _ = e.Dispatchers("X.M", graphq.Options{Limit: 1})
	if len(r.CallSites) != 1 || r.Truncated.Count != 3 || r.CallSitesTotal != 4 {
		t.Errorf("%+v %+v", r.CallSites, r.Truncated)
	}
	// a filter
	r, _ = e.Dispatchers("X.M", graphq.Options{Avoid: []string{"P"}})
	if len(r.CallSites) != 0 || len(r.Filters) != 1 {
		t.Errorf("%+v", r)
	}
	// no dynamic call lists it
	r, err = e.Dispatchers("P", graphq.Options{})
	if err != nil || len(r.CallSites) != 0 || len(r.Hints) == 0 {
		t.Errorf("%v %+v", err, r)
	}
	// a short name that names several, a call, an unknown name
	if r, err := e.Dispatchers("M", graphq.Options{}); err == nil && r.AmbiguousTotal == 0 && len(r.CallSites) == 0 && len(r.Start) == 0 {
		t.Errorf("M: %+v", r)
	}
	if _, err := e.Dispatchers("p.go:1", graphq.Options{}); err == nil {
		t.Error("a call is not an implementation")
	}
	if _, err := e.Dispatchers("No.Such", graphq.Options{}); err == nil {
		t.Error("an unknown name")
	}
}

func TestImplsAndDispatchersJSON(t *testing.T) {
	e := implEngine(t)
	r, _ := e.Impls("app.I.M", graphq.Options{})
	b, _ := json.Marshal(r)
	var m map[string]any
	json.Unmarshal(b, &m)
	for _, k := range []string{"schema", "relation", "flows", "basis", "zero_candidate_sites", "call_sites", "impls", "truncated", "filters", "hints", "omitted"} {
		if _, ok := m[k]; !ok {
			t.Errorf("impls has no %q: %s", k, b)
		}
	}
	d, _ := e.Dispatchers("X.M", graphq.Options{})
	b, _ = json.Marshal(d)
	m = map[string]any{}
	json.Unmarshal(b, &m)
	for _, k := range []string{"schema", "relation", "flows", "basis", "call_sites", "call_sites_total", "truncated", "filters", "hints", "by_callee"} {
		if _, ok := m[k]; !ok {
			t.Errorf("dispatchers has no %q: %s", k, b)
		}
	}
}

// Calls with the same number of candidates are listed by place, the line and the column as numbers: before, the places were compared as text,
// and line 10 came before line 9 (and --limit cut the list at a place that is not the first in the file).
func TestDispatchersAreOrderedByPlaceAsNumbers(t *testing.T) {
	e := build(t, []fnDef{
		{"P", []call{
			{"p.go", 10, 5, "dyn", "app.I.M", []string{"X.M"}},
			{"p.go", 9, 5, "dyn", "app.I.M", []string{"X.M"}},
			{"p.go", 9, 12, "dyn", "app.I.M", []string{"X.M"}},
			{"p.go", 9, 4, "dyn", "app.I.M", []string{"X.M"}},
		}},
		{"X.M", nil},
	})
	r, err := e.Dispatchers("X.M", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range r.CallSites {
		got = append(got, s.At)
	}
	want := []string{"p.go:9:4", "p.go:9:5", "p.go:9:12", "p.go:10:5"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the places in order: %v, want %v", got, want)
	}
}
