package graphq_test

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphdb"
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

// What no edge holds is said for every "no path" of --from, whatever the answer says of exhaustive.
func TestNoPathSaysThatReflectionAndTheRuntimeAreNotEdges(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || !strings.Contains(strings.Join(r.Limits, " "), "reflection") {
		t.Fatalf("%v %+v", err, r.Limits)
	}
}

func unreadFixture() []fnDef {
	return []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"B", []call{{"b.go", 1, 1, "", "T", nil}}},
		{"T", nil},
	}
}

// A package that the load could not read (a type error) is in the run: the code of it may call the goal, and no
// edge says so. "No path" is not exhaustive.
func TestNoPathIsNotExhaustiveWhenCodeOfTheProjectWasNotRead(t *testing.T) {
	testUnread = &graphdb.Unread{FailedPackages: 2, FailedExamples: []string{"example.com/p/bad", "example.com/p/worse"}}
	e := build(t, unreadFixture())
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted || r.NotExhaustiveBecause == nil ||
		r.NotExhaustiveBecause.UnreadCode.Total != 2 || len(r.NotExhaustiveBecause.UnreadCode.Examples) != 2 {
		t.Fatalf("%v %+v because %+v", err, r.Exhaustive, r.NotExhaustiveBecause)
	}
}

// An index with no record of it (made before it was recorded): it is not known that nothing was left out.
func TestNoPathIsNotExhaustiveWhenTheIndexHasNoRecordOfUnreadCode(t *testing.T) {
	testUnread = nil
	e := build(t, unreadFixture())
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || r.NotExhaustiveBecause == nil || r.NotExhaustiveBecause.UnreadCode.Total != -1 ||
		!strings.Contains(r.NotExhaustiveBecause.UnreadCode.Why, "no record") {
		t.Fatalf("%v %+v because %+v", err, r.Exhaustive, r.NotExhaustiveBecause)
	}
}

// What the build constraints leave out is not in the run: "exhaustive" stays true, and the limits say which
// configuration the answer is about.
func TestNoPathStaysExhaustiveForFilesTheConfigurationLeavesOut(t *testing.T) {
	testKeyConfig = map[string]string{"env.go.GOOS": "linux", "env.go.GOARCH": "amd64"}
	e := build(t, unreadFixture())
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || !r.Exhaustive || r.NotExhaustiveBecause != nil {
		t.Fatalf("%v %+v because %+v", err, r.Exhaustive, r.NotExhaustiveBecause)
	}
	if l := strings.Join(r.Limits, " "); !strings.Contains(l, "GOOS=linux") || !strings.Contains(l, "GOARCH=amd64") {
		t.Errorf("limits %v", r.Limits)
	}
}

// A language whose extractor does not report the code it could not read has no zero for it: "not known".
func TestNoPathIsNotExhaustiveWhenAKindOfUnreadCodeIsNotCounted(t *testing.T) {
	testUnread = &graphdb.Unread{Uncounted: []string{"dart:failed_files"}}
	e := build(t, unreadFixture())
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || r.NotExhaustiveBecause == nil || r.NotExhaustiveBecause.UnreadCode.Total != -1 ||
		len(r.NotExhaustiveBecause.UnreadCode.Examples) != 1 {
		t.Fatalf("%v %+v because %+v", err, r.Exhaustive, r.NotExhaustiveBecause)
	}
}

// GRAPH-1: a call on a dynamic value (Dart: dart:core#dynamic.invoke, which the graph holds as a builtin with no
// edge) goes nowhere that the file knows: it is an unresolved call, as a call with no candidate is.
func TestNoPathIsNotExhaustiveWhenADynamicInvokeIsReached(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", []call{{"a.go", 5, 1, "builtin", "dart:core#dynamic.invoke", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || r.Exhaustive || r.NotExhaustiveBecause == nil || r.NotExhaustiveBecause.UnresolvedCalls.Total != 1 {
		t.Fatalf("%v %+v because %+v", err, r.Exhaustive, r.NotExhaustiveBecause)
	}
}

// ... and a builtin that is only that (len, append) is not.
func TestNoPathStaysExhaustiveWhenOnlyBuiltinsAreCalled(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", []call{{"a.go", 5, 1, "builtin", "len", nil}}},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || !r.Exhaustive {
		t.Fatalf("%v %+v because %+v", err, r.Exhaustive, r.NotExhaustiveBecause)
	}
}

// GRAPH-3: "without the filters there is no path either" is a "no path" too: it is exhaustive only when nothing was left out.
func TestWithoutFiltersNoPathIsNotExhaustiveWhenTheGoalIsAMethod(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"(X).M", nil},
	})
	r, err := e.Path("S", "(X).M", graphq.Options{Avoid: []string{"A"}})
	if err != nil || r.Reachable || r.WithoutFilters == nil || r.WithoutFilters.Reachable || r.WithoutFilters.Exhaustive || r.WithoutFilters.NotExhaustiveBecause == nil {
		t.Fatalf("%v %+v wf %+v", err, r.Reachable, r.WithoutFilters)
	}
	for _, h := range r.Hints {
		if strings.Contains(h, "there is no path either") {
			t.Errorf("the hint says no path either: %q", h)
		}
	}
}

// ... and stays "no path either" when nothing was left out.
func TestWithoutFiltersNoPathStaysExhaustiveWhenNothingWasLeftOut(t *testing.T) {
	e := build(t, []fnDef{
		{"S", []call{{"s.go", 3, 1, "", "A", nil}}},
		{"A", nil},
		{"T", nil},
	})
	r, err := e.Path("S", "T", graphq.Options{Avoid: []string{"A"}})
	if err != nil || r.Reachable || r.WithoutFilters == nil || !r.WithoutFilters.Exhaustive {
		t.Fatalf("%v %+v wf %+v", err, r.Reachable, r.WithoutFilters)
	}
}

// Directories that were not read because of where they are: a range declaration in limits; exhaustive stays true.
func TestNoPathSaysThatSkippedDirectoriesAreOutsideTheRange(t *testing.T) {
	testUnread = &graphdb.Unread{SkippedDirs: 2}
	e := build(t, unreadFixture())
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || !r.Exhaustive || !strings.Contains(strings.Join(r.Limits, " "), "2 director") {
		t.Fatalf("%v %+v limits %v", err, r.Exhaustive, r.Limits)
	}
}

// Files that the build constraints leave out: a range declaration in limits; exhaustive stays true.
func TestNoPathSaysThatFilesOutsideTheBuildConfigurationAreOutsideTheRange(t *testing.T) {
	testUnread = &graphdb.Unread{OutsideBuildFiles: 3}
	e := build(t, unreadFixture())
	r, err := e.Path("S", "T", graphq.Options{})
	if err != nil || r.Reachable || !r.Exhaustive || !strings.Contains(strings.Join(r.Limits, " "), "3 file(s)") {
		t.Fatalf("%v %+v limits %v", err, r.Exhaustive, r.Limits)
	}
}
