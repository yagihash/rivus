package graphq_test

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphq"
)

// "No path" is exhaustive only when nothing was left out (the judgement of the adviser, 2026-11-22): the search
// of the edges is complete (edges_exhausted), and the file does not say that the answer may be wrong because of
// what is not in it: a call that goes nowhere that the file knows (no candidate), a function that code that was not
// read may call (a method; a function named as a value that the search did not reach).

// A call through a function value that has no candidate, in what the start reaches: the value may be anything.
func TestNoPathIsNotExhaustiveWhenAFunctionValueHasNoCandidate(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", []call{{"a.go", 5, 1, "val", "", nil}}}, // a call of a function value: no candidate
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable {
		t.Fatalf("%v %+v", err, r)
	}
	if r.Exhaustive || !r.EdgesExhausted || len(r.Limits) == 0 || r.NotExhaustiveBecause == nil || r.NotExhaustiveBecause.UnresolvedCalls.Total != 1 {
		t.Errorf("exhaustive %v, edges_exhausted %v, limits %v, because %+v", r.Exhaustive, r.EdgesExhausted, r.Limits, r.NotExhaustiveBecause)
	}
}

// The same with a call through an interface that has no candidate.
func TestNoPathIsNotExhaustiveWhenAnInterfaceCallHasNoCandidate(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "dyn", "M", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted {
		t.Fatalf("%v %+v", err, r)
	}
}

// The goal is a method: code that was not read (a dependency, the language: an implicit toString) may call it.
func TestNoPathIsNotExhaustiveWhenTheGoalIsAMethod(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"(X).String", nil},
	})
	r, err := e.Path("S", "(X).String", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted || r.NotExhaustiveBecause == nil || r.NotExhaustiveBecause.OpenFunctions.Total == 0 {
		t.Fatalf("%v %+v because %+v", err, r, r.NotExhaustiveBecause)
	}
}

// A caller of the goal (backwards) is a method: it may be called by code that was not read, and so reach the goal.
func TestNoPathIsNotExhaustiveWhenACallerOfTheGoalIsAMethod(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"(X).Run", []call{{"x.go", 1, 1, "", "T", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted {
		t.Fatalf("%v %+v", err, r)
	}
}

// A function named as a value (a closure that something not reached holds) calls the goal.
func TestNoPathIsNotExhaustiveWhenAValueReferencedFunctionReachesTheGoal(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"W", []call{{"w.go", 2, 1, "store", "", []string{"H"}}}},
		{"H", []call{{"h.go", 1, 1, "", "T", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted {
		t.Fatalf("%v %+v", err, r)
	}
}

// Nothing was left out: static calls only, no method, no value held. "No path" stays exhaustive.
func TestNoPathStaysExhaustiveWhenNothingWasLeftOut(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"B", []call{{"b.go", 1, 1, "", "T", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable {
		t.Fatalf("%v %+v", err, r)
	}
	if !r.Exhaustive || !r.EdgesExhausted || r.NotExhaustiveBecause != nil || strings.Contains(strings.Join(r.Limits, " "), "function value") {
		t.Errorf("exhaustive %v, edges_exhausted %v, because %+v, limits %v", r.Exhaustive, r.EdgesExhausted, r.NotExhaustiveBecause, r.Limits)
	}
}

// A path that is found is as it was.
func TestFoundPathIsUnchangedByTheCaveats(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "val", "", nil}, {"s.go", 4, 1, "", "T", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || !r.Reachable || !r.Exhaustive || r.NotExhaustiveBecause != nil {
		t.Fatalf("%v %+v", err, r)
	}
}
