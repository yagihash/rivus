package graphq_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphq"
)

// withCands adds a function without calls for each candidate the program does not define.
func withCands(fns []fnDef) []fnDef {
	have := map[string]bool{}
	for _, f := range fns {
		have[f.id] = true
	}
	for _, f := range append([]fnDef(nil), fns...) {
		for _, c := range f.calls {
			for _, n := range c.cands {
				if !have[n] {
					have[n] = true
					fns = append(fns, fnDef{n, nil})
				}
			}
		}
	}
	return fns
}

func anyHas(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// Hub paths: the program of "hubs"
//
//	A0 calls (hub A: 25 candidates, Q1 among them), and Q1 -> Z                        1 hub,  2 hops
//	A0 -> B -> C, C calls (hub D: 25 candidates, Z among them)                         1 hub,  3 hops
//	A0 calls (hub B2: 25 candidates, Q2 among them), Q2 calls (hub C2: Z among them)   2 hubs, 2 hops
//	N: a hub whose candidates reach nothing
func hubProgram() []fnDef {
	return withCands([]fnDef{
		{"A0", []call{
			{"h.go", 1, 1, "dyn", "W.A", append([]string{"Q1"}, cands(24, "Pa")...)},
			{"h.go", 2, 1, "", "B", nil},
			{"h.go", 3, 1, "dyn", "W.B", append([]string{"Q2"}, cands(24, "Pb")...)},
		}},
		{"Q1", []call{{"h.go", 10, 1, "", "Z", nil}}},
		{"B", []call{{"h.go", 11, 1, "", "C", nil}}},
		{"C", []call{{"h.go", 12, 1, "dyn", "W.D", append([]string{"Z"}, cands(24, "Pd")...)}}},
		{"Q2", []call{{"h.go", 13, 1, "dyn", "W.C", append([]string{"Z"}, cands(24, "Pc")...)}}},
		{"Z", nil},
		{"N", []call{{"h.go", 20, 1, "dyn", "W.N", cands(30, "Pn")}}},
	})
}

func TestPathThirdStageGoesThroughAHub(t *testing.T) {
	e := build(t, hubProgram())
	r, err := e.Path("A0", "Z", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || r.Certainty != "hub" || strings.Join(r.Searched, ",") != "static,dynamic,hub" || !r.Exhaustive {
		t.Fatalf("%+v", r)
	}
	// the fewest hubs first, then the shortest; the path with two hubs is not listed
	var got []string
	for _, p := range r.Paths {
		got = append(got, pathIDs(p))
		if p.Certainty != "hub" {
			t.Errorf("%+v", p)
		}
	}
	if strings.Join(got, " ") != "A0>Q1>Z A0>B>C>Z" {
		t.Fatalf("%v", got)
	}
	s := r.Paths[0].Steps[0]
	if !s.Hub || s.Candidates != 25 || s.Kind != "interface" {
		t.Errorf("the hub step: %+v", s)
	}
	if !anyHas(r.Hints, "hub") || !anyHas(r.Hints, "--only-impl") {
		t.Errorf("hints: %v", r.Hints)
	}
	// the end of the path says where it is declared
	if last := r.Paths[0].Steps[2]; last.DefinedAt == "" || last.At != "" {
		t.Errorf("last: %+v", last)
	}
	// --hubs asks for the third stage at once; the same answer, as nothing is shorter
	h, _ := e.Path("A0", "Z", graphq.Options{Hubs: true})
	if !h.Reachable || pathIDs(h.Paths[0]) != "A0>Q1>Z" || strings.Join(h.Searched, ",") != "static,dynamic,hub" {
		t.Errorf("%+v", h)
	}
}

func TestPathNoPathEvenThroughHubs(t *testing.T) {
	e := build(t, hubProgram())
	for _, c := range [][2]string{{"N", "Z"}, {"Z", "A0"}} {
		r, err := e.Path(c[0], c[1], graphq.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Reachable || len(r.Paths) != 0 || !r.Exhaustive || strings.Join(r.Searched, ",") != "static,dynamic,hub,ref" || r.Certainty != "" {
			t.Errorf("%s -> %s: %+v", c[0], c[1], r)
		}
	}
	// a path found by the dynamic calls does not search the hubs, and says so
	fns := append(hubProgram(), fnDef{"S", []call{{"s.go", 1, 1, "dyn", "I.M", []string{"M1", "M2"}}, {"s.go", 2, 1, "dyn", "W.S", cands(25, "Ps")}}}, fnDef{"M1", []call{{"s.go", 3, 1, "", "Z", nil}}}, fnDef{"M2", nil})
	e = build(t, withCands(fns))
	r, _ := e.Path("S", "Z", graphq.Options{})
	if !r.Reachable || r.Certainty != "uncertain" || strings.Join(r.Searched, ",") != "static,dynamic" || r.HubSkippedTotal != 4 || !anyHas(r.Hints, "--hubs") {
		t.Errorf("%+v", r)
	}
}

// The total of the hubs met is all of them, not as many as are named.
func TestHubSkippedTotalCountsEveryHub(t *testing.T) {
	var calls []call
	for i := 0; i < 5; i++ {
		calls = append(calls, call{"t.go", 1 + i, 1, "dyn", fmt.Sprintf("W.H%d", i), cands(25, fmt.Sprintf("Pq%d_", i))})
	}
	calls = append(calls, call{"t.go", 10, 1, "dyn", "I.M", []string{"M1", "M2"}})
	e := build(t, withCands([]fnDef{{"HF", calls}, {"M1", []call{{"t.go", 20, 1, "", "Z", nil}}}, {"M2", nil}, {"Z", nil}}))
	r, err := e.Path("HF", "Z", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || r.HubSkippedTotal != 5 {
		t.Errorf("total %d: %+v", r.HubSkippedTotal, r)
	}
}

func TestShortSymbolsEndAtABoundary(t *testing.T) {
	fns := []fnDef{
		{"example.com/models/user.GetUserByID", nil},
		{"example.com/models/user.GetUserByIDs", nil},
		{"(*example.com/models/user.User).GetName", nil},
		{"(example.com/models/org.Org).Name", nil},
		{"example.com/models/user.Close", nil},
		{"example.com/db.Close", nil},
		{"example.com/net.Close", []call{{"n.go", 1, 1, "", "ext/lib.Close", nil}}},
		{"package:app/a.dart#Widget.build", nil},
		{"package:app/b.dart#Panel.build", nil},
		{"<sample::traits::Printer as sample::traits::Handler>::handle", nil},
		{"<sample::traits::Quiet as sample::traits::Handler>::handle", nil},
		{"sample::async_flows::consume", nil},
	}
	for i := 0; i < 25; i++ {
		fns = append(fns, fnDef{fmt.Sprintf("example.com/p%d.Open", i), nil})
	}
	e := build(t, fns)
	start := func(spec string) (*graphq.CallersResult, error) { return e.Callers(spec, graphq.Options{}) }

	// the end of a name, at the boundary of an identifier: "ByID" is not the end of "GetUserByID"
	if _, err := start("ByID"); err == nil || !strings.Contains(err.Error(), "no function") || !strings.Contains(err.Error(), "GetUserByID") {
		t.Errorf("ByID: %v", err)
	}
	r, err := start("GetUserByID")
	if err != nil || r.Start[0] != "example.com/models/user.GetUserByID" || !strings.Contains(r.Query.Resolved, "the end of it") {
		t.Errorf("%v %+v", err, r.Query)
	}
	if r, err = start("user.GetUserByID"); err != nil || len(r.Start) != 1 {
		t.Errorf("%v", err)
	}
	// a method, as it is written, or with the type only
	for _, spec := range []string{"(*user.User).GetName", "User.GetName", "user.User.GetName", "GetName"} {
		if r, err = start(spec); err != nil || len(r.Start) != 1 || r.Start[0] != "(*example.com/models/user.User).GetName" {
			t.Errorf("%s: %v %+v", spec, err, r)
		}
	}
	if r, err = start("(org.Org).Name"); err != nil || len(r.Start) != 1 {
		t.Errorf("%v", err)
	}
	// several: not an error, not a pick; the code with a body first, the total, the hint
	r, err = start("Close")
	if err != nil || len(r.Start) != 0 || r.AmbiguousTotal != 4 || len(r.Ambiguous) != 4 || len(r.Nodes) != 0 {
		t.Fatalf("%v %+v", err, r)
	}
	if last := r.Ambiguous[3].(graphq.FuncChoice); last.ID != "ext/lib.Close" || !last.NoBody {
		t.Errorf("a function with no body comes last: %+v", r.Ambiguous)
	}
	if !anyHas(r.Hints, "id") {
		t.Errorf("%v", r.Hints)
	}
	// more than are listed: the total, and the way to name one
	r, _ = start("Open")
	if r.AmbiguousTotal != 25 || len(r.Ambiguous) != graphq.MaxChoices || !anyHas(r.Hints, "pkg.Name") {
		t.Errorf("%d %d %v", r.AmbiguousTotal, len(r.Ambiguous), r.Hints)
	}
	// qualified, the one is found
	if r, err = start("p3.Open"); err != nil || len(r.Start) != 1 {
		t.Errorf("%v", err)
	}
	// Dart and Rust forms
	if r, err = start("Widget.build"); err != nil || r.Start[0] != "package:app/a.dart#Widget.build" {
		t.Errorf("dart: %v", err)
	}
	if r, err = start("a.dart#Widget.build"); err != nil || len(r.Start) != 1 {
		t.Errorf("dart: %v", err)
	}
	if r, _ = start("build"); r.AmbiguousTotal != 2 {
		t.Errorf("dart: %+v", r)
	}
	if r, err = start("async_flows::consume"); err != nil || len(r.Start) != 1 {
		t.Errorf("rust: %v", err)
	}
	if r, err = start("Printer::handle"); err != nil || r.Start[0] != "<sample::traits::Printer as sample::traits::Handler>::handle" {
		t.Errorf("rust impl: %v", err)
	}
	if r, _ = start("handle"); r.AmbiguousTotal != 2 {
		t.Errorf("rust: %+v", r)
	}
	// a path with a short name that is not one: the choice is the answer
	p, err := e.Path("Close", "GetUserByID", graphq.Options{})
	if err != nil || len(p.Ambiguous) != 4 || p.Reachable || p.AmbiguousTotal != 4 {
		t.Errorf("%v %+v", err, p)
	}
}

func TestFilters(t *testing.T) {
	e := engine(t)
	// avoid: a path that must go through B has none
	r, err := e.Path("main", "C", graphq.Options{Avoid: []string{"B"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Reachable || len(r.Filters) != 1 || !strings.HasPrefix(r.Filters[0], "avoid B") {
		t.Fatalf("%+v", r)
	}
	if r.WithoutFilters == nil || !r.WithoutFilters.Reachable || r.WithoutFilters.Hops != 3 || !anyHas(r.Hints, "without the filters there is a path") {
		t.Errorf("%+v %v", r.WithoutFilters, r.Hints)
	}
	// a filter that is not what stops it: without them there is none either
	r, _ = e.Path("D", "main", graphq.Options{Avoid: []string{"A"}})
	if r.Reachable || r.WithoutFilters == nil || r.WithoutFilters.Reachable || !anyHas(r.Hints, "no path either") {
		t.Errorf("%+v", r)
	}
	// no filter: no "without_filters"
	if r, _ = e.Path("D", "main", graphq.Options{}); r.WithoutFilters != nil || len(r.Filters) != 0 {
		t.Errorf("%+v", r)
	}
	// avoid, and the others: callers and callees
	c, _ := e.Callees("A", graphq.Options{Avoid: []string{"B"}})
	if hasEdge(&c.Graph, "A", "B", "") != nil || len(c.Filters) != 1 || !anyHas(c.Hints, "filters") {
		t.Errorf("%+v", c)
	}
	cr, _ := e.Callers("C", graphq.Options{Avoid: []string{"B"}})
	if hasEdge(&cr.Graph, "B", "C", "") != nil {
		t.Errorf("%+v", cr.Edges)
	}
	// via: a way that goes through the function
	r, _ = e.Path("main", "C", graphq.Options{Via: []string{"B"}})
	if !r.Reachable || pathIDs(r.Paths[0]) != "main>A>B>C" || len(r.Filters) != 1 {
		t.Errorf("%+v", r)
	}
	r, _ = e.Path("main", "D", graphq.Options{Via: []string{"X.M"}})
	if !r.Reachable || pathIDs(r.Paths[0]) != "main>X.M>D" || r.Certainty != "uncertain" {
		t.Errorf("%+v", r)
	}
	r, _ = e.Path("main", "D", graphq.Options{Via: []string{"Y.M"}})
	if r.Reachable || r.WithoutFilters == nil || !r.WithoutFilters.Reachable {
		t.Errorf("%+v", r)
	}
	// two via functions, in order
	r, _ = e.Path("main", "C", graphq.Options{Via: []string{"A", "B"}})
	if !r.Reachable || pathIDs(r.Paths[0]) != "main>A>B>C" {
		t.Errorf("%+v", r)
	}
	r, _ = e.Path("main", "C", graphq.Options{Via: []string{"B", "A"}})
	if r.Reachable && pathIDs(r.Paths[0]) == "main>A>B>C" {
		t.Errorf("the order is part of the filter: %v", pathIDs(r.Paths[0]))
	}
	// via on callers and callees: only what goes through it
	cr, _ = e.Callers("D", graphq.Options{Depth: 2, Via: []string{"X.M"}})
	if got := ids(&cr.Graph); len(got) != 4 || got["X.M"] != 1 || got["main"] != 2 || got["F"] != 2 || got["D"] != 0 {
		t.Errorf("%+v", cr.Nodes)
	}
	cr, _ = e.Callers("D", graphq.Options{Depth: 2, Via: []string{"Y.M"}})
	if len(cr.Nodes) != 1 || len(cr.Edges) != 0 {
		t.Errorf("%+v", cr)
	}
	c, _ = e.Callees("main", graphq.Options{Depth: 3, Via: []string{"D"}})
	if hasEdge(&c.Graph, "X.M", "D", "") == nil || hasEdge(&c.Graph, "A", "B", "") != nil {
		t.Errorf("%+v", c.Edges)
	}
	// only-impl: a call through an interface reaches only the methods of the type
	cr, _ = e.Callers("X.M", graphq.Options{OnlyImpl: []string{"Y"}})
	if hasEdge(&cr.Graph, "main", "X.M", "interface") != nil || hasEdge(&cr.Graph, "main", "X.M", "value") == nil {
		t.Errorf("%+v", cr.Edges)
	}
	c, _ = e.Callees("main", graphq.Options{OnlyImpl: []string{"Y"}})
	if hasEdge(&c.Graph, "main", "Y.M", "interface") == nil || hasEdge(&c.Graph, "main", "X.M", "interface") != nil || hasEdge(&c.Graph, "main", "A", "static") == nil {
		t.Errorf("%+v", c.Edges)
	}
	r, _ = e.Path("F", "D", graphq.Options{OnlyImpl: []string{"X"}})
	if !r.Reachable || pathIDs(r.Paths[0]) != "F>X.M>D" {
		t.Errorf("%+v", r)
	}
	r, _ = e.Path("F", "D", graphq.Options{OnlyImpl: []string{"Y"}})
	if r.Reachable || r.WithoutFilters == nil || !r.WithoutFilters.Reachable || !strings.HasPrefix(r.Filters[0], "only-impl Y") {
		t.Errorf("%+v", r)
	}
	// the type is the whole word: "X" is not "XX"; no candidate of the call site is a method of it, so the site is not narrowed
	if c, _ = e.Callees("main", graphq.Options{OnlyImpl: []string{"."}}); hasEdge(&c.Graph, "main", "X.M", "interface") == nil || hasEdge(&c.Graph, "main", "Y.M", "interface") == nil {
		t.Errorf("%+v", c.Edges)
	}
	// a type no function has is said
	if c, _ = e.Callees("main", graphq.Options{OnlyImpl: []string{"Nothing"}}); len(c.Warnings) == 0 {
		t.Errorf("%+v", c)
	}
	// a filter that names nothing, or several, is an error
	if _, err := e.Path("main", "C", graphq.Options{Avoid: []string{"nope"}}); err == nil || !strings.Contains(err.Error(), "--avoid") {
		t.Errorf("%v", err)
	}
	// avoid is repeatable
	r, _ = e.Path("main", "D", graphq.Options{Avoid: []string{"X.M", "Y.M"}})
	if r.Reachable || len(r.Filters) != 2 {
		t.Errorf("%+v", r)
	}
}

func TestPathFromEntry(t *testing.T) {
	fns := []fnDef{
		{"main1", []call{{"e.go", 1, 1, "", "U", nil}}},
		{"main2", []call{{"e.go", 2, 1, "", "V", nil}}},
		{"U", []call{{"e.go", 3, 1, "", "Z", nil}}},
		{"V", []call{{"e.go", 4, 1, "", "W", nil}}},
		{"W", []call{{"e.go", 5, 1, "", "Z", nil}}},
		{"Z", nil},
		{"Dead", []call{{"e.go", 6, 1, "", "Z", nil}}},
	}
	entryIDs = map[string]bool{"main1": true, "main2": true}
	e := build(t, fns)
	r, err := e.Path("", "Z", graphq.Options{FromEntry: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || r.Entry == nil || r.Entry.Source != "entry" || r.Entry.Count != 2 || r.Entry.Definition == "" {
		t.Fatalf("%+v", r)
	}
	var got []string
	for _, p := range r.Paths {
		got = append(got, pathIDs(p))
	}
	// the first step is the entry; both entries reach it
	if strings.Join(got, " ") != "main1>U>Z main2>V>W>Z" {
		t.Errorf("%v", got)
	}
	// a function no entry reaches
	r, _ = e.Path("", "Dead", graphq.Options{FromEntry: true})
	if r.Reachable || r.Exhaustive || !r.EdgesExhausted || !anyHas(r.Hints, "no entry reaches it") || r.Entry == nil {
		t.Errorf("%+v", r)
	}
	// --from-entry with no path, the references searched too: exhaustive of the calls and the references, and the limits (reflection, frameworks) and the facts of the goal come with it
	if len(r.Limits) != 3 || !strings.Contains(strings.Join(r.Limits, " "), "reflection") || !strings.Contains(r.ExhaustiveOf, "call edges") || !strings.Contains(r.ExhaustiveOf, "references") {
		t.Errorf("limits %+v", r.Limits)
	}
	if len(r.TargetFacts) != 1 || r.TargetFacts[0].ID != "Dead" || !r.TargetFacts[0].HasBody || r.TargetFacts[0].Dep || r.TargetFacts[0].Exported || r.TargetFacts[0].StaticCallSites != 0 || r.TargetFacts[0].DynamicCallSites != 0 || r.TargetFacts[0].ValueReferences == nil || *r.TargetFacts[0].ValueReferences != 0 {
		t.Errorf("facts %+v", r.TargetFacts)
	}
	// with --no-refs the references are not searched: never exhaustive, and the limits say a stored function is not an edge
	r, _ = e.Path("", "Dead", graphq.Options{FromEntry: true, NoRefs: true})
	if r.Reachable || r.Exhaustive || !r.EdgesExhausted || len(r.Limits) != 3 || !strings.Contains(strings.Join(r.Limits, " "), "stored as a value") || strings.Contains(r.ExhaustiveOf, "references") {
		t.Errorf("no-refs %+v", r)
	}
	// a goal that is called, by a function no entry reaches: the callers are counted, and no path is still said
	r, _ = e.Path("", "Dead", graphq.Options{FromEntry: true})
	if !strings.Contains(r.Entry.Definition, "With --no-refs the references are not followed") {
		t.Errorf("definition %q", r.Entry.Definition)
	}
	// a path found keeps exhaustive and has no limits
	if r, _ = e.Path("", "Z", graphq.Options{FromEntry: true}); !r.Reachable || len(r.Limits) != 0 || len(r.TargetFacts) != 0 {
		t.Errorf("%+v", r)
	}
	// without --from-entry, nothing changes
	if r, _ = e.Path("Dead", "Z", graphq.Options{}); !r.Reachable || !r.Exhaustive || len(r.Limits) != 0 {
		t.Errorf("%+v", r)
	}
	// (a "no path" of --from says that reflection and the start of the program are not edges: one line, from 2026-11-22)
	if r, _ = e.Path("Z", "Dead", graphq.Options{}); r.Reachable || !r.Exhaustive || len(r.Limits) != 1 || r.TargetFacts != nil {
		t.Errorf("%+v", r)
	}
	// --from and --from-entry together
	if _, err := e.Path("main1", "Z", graphq.Options{FromEntry: true}); err == nil {
		t.Error("two starts")
	}
	// a library: no start, so the exported functions are the entries
	entryIDs, exportedIDs = nil, nil
	exportedIDs = map[string]bool{"V": true}
	e = build(t, fns)
	r, err = e.Path("", "Z", graphq.Options{FromEntry: true})
	if err != nil || !r.Reachable || r.Entry.Source != "exported" || r.Entry.Count != 1 || pathIDs(r.Paths[0]) != "V>W>Z" {
		t.Errorf("%v %+v", err, r)
	}
	// nothing says where it starts
	e = build(t, fns)
	if _, err := e.Path("", "Z", graphq.Options{FromEntry: true}); err == nil || !strings.Contains(err.Error(), "--from-entry") {
		t.Errorf("%v", err)
	}
}

// The facts of a goal no entry reaches: its callers are counted, though the callers are unreachable too.
func TestEntryTargetFactsCountCallers(t *testing.T) {
	fns := []fnDef{
		{"main1", nil},
		{"Handler", []call{{"e.go", 1, 1, "", "Helper", nil}}},
		{"Other", []call{{"e.go", 2, 1, "", "Helper", nil}}},
		{"Helper", nil},
	}
	entryIDs = map[string]bool{"main1": true}
	e := build(t, fns)
	r, err := e.Path("", "Helper", graphq.Options{FromEntry: true})
	if err != nil || r.Reachable || r.Exhaustive || !r.EdgesExhausted || len(r.TargetFacts) != 1 || r.TargetFacts[0].StaticCallSites != 2 {
		t.Errorf("%v %+v", err, r)
	}
}

// A --limit as large as an int can be is "all": the search asks for one path more than the limit, and that sum overflowed to the
// smallest int, so that no path was listed and the answer said that none was found.
func TestPathWithTheLargestLimit(t *testing.T) {
	e := build(t, []fnDef{
		{"A", []call{{"a.go", 1, 1, "", "B", nil}}},
		{"B", nil},
	})
	r, err := e.Path("A", "B", graphq.Options{Limit: math.MaxInt})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || len(r.Paths) != 1 {
		t.Fatalf("reachable = %v, %d paths: %+v", r.Reachable, len(r.Paths), r)
	}
}

// A read that fails while --only-impl asks about a call site is not "the site has no candidate of the type": the answer is not
// remembered (the next edge asks again), and the answer says that the filter could not be applied. Before, the error was dropped and
// the call site was counted as "left alone" for the rest of the search.
func TestOnlyImplSaysWhenTheDatabaseCannotBeRead(t *testing.T) {
	e := build(t, withCands([]fnDef{
		{"S", []call{{"s.go", 1, 1, "dyn", "io.Writer.Write", []string{"(*bytes.Buffer).Write", "(*os.File).Write"}}}},
		{"(*bytes.Buffer).Write", nil},
		{"(*os.File).Write", nil},
	}))
	passed, warnings, remembered := graphq.OnlyImplAfterClose(e, []string{"bytes.Buffer"})
	if !passed {
		t.Error("an edge is kept when the filter cannot tell")
	}
	if remembered != 0 {
		t.Errorf("%d answers remembered from a failed read", remembered)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "--only-impl") {
		t.Errorf("warnings: %v", warnings)
	}
}

// --avoid-call and --avoid cut a call; what the answer counts besides the edges (the frontier of the dynamic calls it stopped at, and the functions at
// the last depth that "have more beyond it") is the same calls: before, those two counted them without the filters, so a call that was cut was still
// listed as "stopped at" and the hint said that --depth goes further.
func TestFiltersApplyToTheFrontierAndToWhatIsBeyondTheDepth(t *testing.T) {
	cands := []string{"I1", "I2", "I3", "I4", "I5"} // more than ExpandCandidates: the call is not followed, it is a frontier
	fns := withCands([]fnDef{
		{"F", []call{{"f.go", 7, 3, "dyn", "J.N", cands}}},
		{"S", []call{{"s.go", 1, 1, "", "A", nil}}},
		{"A", []call{{"a.go", 2, 1, "", "B", nil}}},
		{"B", nil},
	})
	e := build(t, fns)

	r, err := e.Callees("F", graphq.Options{})
	if err != nil || r.FrontierTotal != 1 {
		t.Fatalf("without a filter the dynamic call is the frontier: %v %+v", err, r.Frontier)
	}
	r, err = e.Callees("F", graphq.Options{AvoidCall: []string{"f.go:7:3"}})
	if err != nil || r.FrontierTotal != 0 || len(r.Frontier) != 0 || anyHas(r.Hints, "stopped at") {
		t.Errorf("--avoid-call cut the call, and it is still a frontier: %v total=%d %+v hints=%v", err, r.FrontierTotal, r.Frontier, r.Hints)
	}
	r, err = e.Callees("F", graphq.Options{Avoid: []string{"F"}})
	if err != nil || r.FrontierTotal != 0 {
		t.Errorf("--avoid F: no call from it, and the frontier lists one: %v total=%d", err, r.FrontierTotal)
	}

	// S -> A -> B: at depth 1 A is the last level, and it calls B
	r, err = e.Callees("S", graphq.Options{Depth: 1})
	if err != nil || r.Truncated.BeyondDepth != 1 {
		t.Fatalf("without a filter A has a call beyond the depth: %v %+v", err, r.Truncated)
	}
	r, err = e.Callees("S", graphq.Options{Depth: 1, Avoid: []string{"B"}})
	if err != nil || r.Truncated.BeyondDepth != 0 || anyHas(r.Hints, "--depth 2 goes further") {
		t.Errorf("--avoid B cuts the only call of A: nothing is beyond the depth: %v %+v hints=%v", err, r.Truncated, r.Hints)
	}
	// the same for the callers: B <- A <- S
	c, err := e.Callers("B", graphq.Options{Depth: 1})
	if err != nil || c.Truncated.BeyondDepth != 1 {
		t.Fatalf("without a filter A has a caller beyond the depth: %v %+v", err, c.Truncated)
	}
	c, err = e.Callers("B", graphq.Options{Depth: 1, Avoid: []string{"S"}})
	if err != nil || c.Truncated.BeyondDepth != 0 {
		t.Errorf("--avoid S cuts the only caller of A: %v %+v", err, c.Truncated)
	}
}
