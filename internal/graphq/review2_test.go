package graphq_test

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphq"
)

// G-9: --avoid-call that is moved to the call before the column says which call
// it cut (the position, the callee), and that it was moved.
func TestAvoidCallSaysWhatItResolvedTo(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 5, 3, "", "B", nil}, {"s.go", 5, 10, "", "C", nil}}},
		{"B", nil}, {"C", nil},
	})
	r, err := e.Path("S", "C", graphq.Options{AvoidCall: []string{"s.go:5:12"}})
	if err != nil || r.Reachable || len(r.Filters) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	f := r.Filters[0]
	for _, want := range []string{"s.go:5:10", "of C", "no call begins where s.go:5:12 says", "1 call site cut"} {
		if !strings.Contains(f, want) {
			t.Errorf("%q lacks %q", f, want)
		}
	}
	// a call that begins at the column is not said to be moved
	r, _ = e.Path("S", "C", graphq.Options{AvoidCall: []string{"s.go:5:10"}})
	if strings.Contains(r.Filters[0], "no call begins") || !strings.Contains(r.Filters[0], "s.go:5:10") {
		t.Errorf("%q", r.Filters[0])
	}
}

// G-10: the calls at one place (a desugared "a()?") are all cut, and their number is said.
func TestAvoidCallCutsEveryCallAtThePlace(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 5, 3, "", "B", nil}, {"s.go", 5, 3, "", "C", nil}}},
		{"B", []call{{"b.go", 1, 1, "", "Z", nil}}},
		{"C", []call{{"c.go", 1, 1, "", "Z", nil}}},
		{"Z", nil},
	})
	r, err := e.Path("S", "Z", graphq.Options{})
	if err != nil || !r.Reachable {
		t.Fatalf("%v %+v", err, r)
	}
	r, err = e.Path("S", "Z", graphq.Options{AvoidCall: []string{"s.go:5:3"}})
	if err != nil || r.Reachable {
		t.Fatalf("the other call at the place is left: %v %+v", err, r)
	}
	if len(r.Filters) != 1 || !strings.Contains(r.Filters[0], "2 call sites cut") {
		t.Errorf("%v", r.Filters)
	}
}

// G-11: a call with no position has no "at" (not file:0:0), and says so.
func TestStepWithNoPositionHasNoAt(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 0, 0, "", "Z", nil}}},
		{"Z", nil},
	})
	r, err := e.Path("S", "Z", graphq.Options{})
	if err != nil || !r.Reachable {
		t.Fatalf("%v %+v", err, r)
	}
	st := r.Paths[0].Steps[0]
	if st.At != "" || !st.AtUnknown {
		t.Errorf("%+v", st)
	}
}

// G-12: two filters that block the same only path: taking one off does not let a path through.
func TestWithoutFiltersHintWhenNoOneFilterIsTheCause(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 1, 1, "", "A", nil}}},
		{"A", []call{{"a.go", 1, 1, "", "B", nil}}},
		{"B", []call{{"b.go", 1, 1, "", "Z", nil}}},
		{"Z", nil},
	})
	r, err := e.Path("S", "Z", graphq.Options{Avoid: []string{"A", "B"}})
	if err != nil || r.Reachable || r.WithoutFilters == nil || !r.WithoutFilters.Reachable {
		t.Fatalf("%v %+v", err, r)
	}
	if len(r.WithoutFilters.Culprits) != 0 {
		t.Fatalf("%v", r.WithoutFilters.Culprits)
	}
	if !anyHas(r.Hints, "any one of them alone does not let one through") || anyHas(r.Hints, "take one off") {
		t.Errorf("%v", r.Hints)
	}
}

// G-14: from the entries, "no path" is never "exhaustive" (a method of a type that a dependency
// calls through an interface has no caller here), the edges were still searched, and the target
// says it is a method.
func TestEntryNoPathIsNeverExhaustiveAndSaysMethod(t *testing.T) {
	entryIDs = map[string]bool{"main": true}
	e := build(t, []fnDef{
		{"main", nil},
		{"(*example.com/p.listFlag).Set", nil},
		{"example.com/p.helper", nil},
	})
	r, err := e.Path("", "(*example.com/p.listFlag).Set", graphq.Options{FromEntry: true})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted {
		t.Fatalf("%v %+v", err, r)
	}
	if len(r.TargetFacts) != 1 || !r.TargetFacts[0].HasReceiver || !anyHas(r.Hints, "is a method") {
		t.Errorf("%+v %v", r.TargetFacts, r.Hints)
	}
	r, _ = e.Path("", "example.com/p.helper", graphq.Options{FromEntry: true})
	if r.Exhaustive || len(r.TargetFacts) != 1 || r.TargetFacts[0].HasReceiver || anyHas(r.Hints, "is a method") {
		t.Errorf("%+v %v", r.TargetFacts, r.Hints)
	}
	// a start named: the answer is of the question, and stays exhaustive
	r, _ = e.Path("main", "example.com/p.helper", graphq.Options{})
	if r.Reachable || !r.Exhaustive {
		t.Errorf("%+v", r)
	}
}

// G-15 / G-16: a reference whose instruction has no position has no "at"; a
// reference step says its position is the statement's.
func TestRefStepPositionPrecision(t *testing.T) {
	entryIDs = map[string]bool{"main": true}
	e := build(t, []fnDef{
		{"main", []call{
			{"r.go", 11, 2, "ref", "Reg.Get", []string{"H"}},
			{"r.go", 12, 2, "closure0", "Reg.Run", []string{"Lit"}},
		}},
		{"H", nil}, {"Lit", nil},
	})
	r, _ := e.Path("main", "H", graphq.Options{})
	st := r.Paths[0].Steps[0]
	if st.At != "r.go:11:2" || st.AtPrecision != "statement" || st.AtUnknown {
		t.Errorf("%+v", st)
	}
	if !anyHas(r.Hints, "statement that holds the reference") {
		t.Errorf("%v", r.Hints)
	}
	// the closure made by an instruction with no position (G-15, before go/ssa's literal position was used)
	r, _ = e.Path("main", "Lit", graphq.Options{})
	st = r.Paths[0].Steps[0]
	if st.At != "" || !st.AtUnknown || st.RefKind != "closure" {
		t.Errorf("%+v", st)
	}
}
