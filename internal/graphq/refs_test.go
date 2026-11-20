package graphq_test

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphq"
)

// A program with references: main registers handlers with a router that is
// not in the program (Reg.Get takes a function as an argument), and no call
// reaches them.
//
//	main -> (call) A;  main -ref(arg, via Reg.Get)-> H;  main -ref(closure, via Reg.Run)-> Lit
//	A -ref(arg, via Reg.Get)-> T;  main -ref(arg)-> H1 -ref(arg)-> T (two references)
//	main -> B1 -> B2 -ref(arg)-> T (one reference, longer)
//	Store holds S in a variable (no call);  main -> Store
func refProgram() []fnDef {
	entryIDs = map[string]bool{"main": true}
	return []fnDef{
		{"main", []call{
			{"r.go", 10, 2, "", "A", nil},
			{"r.go", 11, 2, "ref", "Reg.Get", []string{"H"}},
			{"r.go", 12, 2, "closure", "Reg.Run", []string{"Lit"}},
			{"r.go", 13, 2, "ref", "Reg.Get", []string{"H1"}},
			{"r.go", 14, 2, "", "B1", nil},
			{"r.go", 15, 2, "", "Store", nil},
		}},
		{"A", []call{{"r.go", 20, 3, "ref", "Reg.Get", []string{"T"}}}},
		{"H", nil},
		{"Lit", nil},
		{"H1", []call{{"r.go", 30, 3, "ref", "Reg.Get", []string{"T"}}}},
		{"B1", []call{{"r.go", 40, 2, "", "B2", nil}}},
		{"B2", []call{{"r.go", 41, 2, "ref", "Reg.Get", []string{"T"}}}},
		{"T", nil},
		{"Store", []call{{"r.go", 50, 4, "store", "", []string{"S"}}}},
		{"S", nil},
		{"Unref", nil},
	}
}

func stepIDs(p graphq.Path) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Func)
	}
	return out
}

func TestPathFourthStageReferences(t *testing.T) {
	e := build(t, refProgram())
	r, err := e.Path("main", "H", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || len(r.Paths) != 1 || r.Certainty != "reference" || r.Relation != "calls+refs" ||
		strings.Join(r.Searched, ",") != "static,dynamic,hub,ref" || r.Listed == nil || r.Listed.Stage != "ref" || r.Listed.Refs == nil || *r.Listed.Refs != 1 || r.Beyond != "not_searched" {
		t.Fatalf("%+v", r)
	}
	p := r.Paths[0]
	if p.Relation != "calls+refs" || p.Certainty != "reference" || p.Note == "" || p.Refs != 1 || p.Hops != 1 || strings.Join(stepIDs(p), ">") != "main>H" {
		t.Fatalf("%+v", p)
	}
	// the step is the place where the function is named, not a call
	st := p.Steps[0]
	if st.Kind != "ref" || st.At != "r.go:11:2" || st.ViaCallee != "Reg.Get" || st.RefKind != "arg" || st.Note == "" || len(st.Sites) != 0 || st.Candidates != 0 || st.Hub {
		t.Errorf("%+v", st)
	}
	// the last step is the function reached
	if last := p.Steps[1]; last.Func != "H" || last.At != "" || last.Kind != "" {
		t.Errorf("%+v", last)
	}
	// a closure that captures a value, made where it is passed on
	r, _ = e.Path("main", "Lit", graphq.Options{})
	if !r.Reachable || r.Paths[0].Steps[0].RefKind != "closure" || r.Paths[0].Steps[0].ViaCallee != "Reg.Run" || r.Paths[0].Steps[0].At != "r.go:12:1" {
		t.Errorf("closure: %+v", r)
	}
	// a function held by an instruction that is no call: no callee
	r, _ = e.Path("main", "S", graphq.Options{})
	if !r.Reachable || r.Paths[0].Steps[1].Func != "Store" && r.Paths[0].Steps[0].Func != "Store" {
		t.Fatalf("store: %+v", r)
	}
	last := r.Paths[0].Steps[len(r.Paths[0].Steps)-2]
	if last.Kind != "ref" || last.RefKind != "other" || last.ViaCallee != "" || last.At != "r.go:50:4" {
		t.Errorf("store step: %+v", last)
	}
	// --no-refs leaves the stage out, and says so
	r, _ = e.Path("main", "H", graphq.Options{NoRefs: true})
	if r.Reachable || strings.Join(r.Searched, ",") != "static,dynamic,hub" || !r.Exhaustive || r.Relation != "calls" || strings.Contains(r.ExhaustiveOf, "references") {
		t.Errorf("no-refs: %+v", r)
	}
	// a path of calls is found before the references are looked at
	r, _ = e.Path("main", "B2", graphq.Options{})
	if !r.Reachable || r.Certainty != "certain" || r.Relation != "calls" || r.Paths[0].Relation != "calls" || r.Paths[0].Note != "" || strings.Join(r.Searched, ",") != "static" {
		t.Errorf("calls first: %+v", r)
	}
	// --refs begins at the fourth stage
	r, _ = e.Path("main", "B2", graphq.Options{Refs: true})
	if !r.Reachable || r.Listed.Stage != "ref" || r.Certainty != "certain" {
		t.Errorf("--refs: %+v", r.Listed)
	}
	r, _ = e.Path("main", "Unref", graphq.Options{})
	if r.Reachable || strings.Join(r.Searched, ",") != "static,dynamic,hub,ref" || !r.Exhaustive || !strings.Contains(r.ExhaustiveOf, "references") {
		t.Errorf("none: %+v", r)
	}
}

// The order: the fewest references, then the fewest hubs, then the shortest.
func TestPathReferencesOrder(t *testing.T) {
	e := build(t, refProgram())
	r, err := e.Path("main", "T", graphq.Options{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range r.Paths {
		got = append(got, strings.Join(stepIDs(p), ">"))
	}
	// one reference first (the shorter, then the longer), the path with two references is not listed
	if strings.Join(got, " ") != "main>A>T main>B1>B2>T" || r.Listed.Refs == nil || *r.Listed.Refs != 1 || r.Listed.ShortestHops != 2 {
		t.Errorf("%v %+v", got, r.Listed)
	}
	for _, p := range r.Paths {
		if p.Refs != 1 || p.Certainty != "reference" {
			t.Errorf("%+v", p)
		}
	}
}

func TestCallersCalleesRefs(t *testing.T) {
	e := build(t, refProgram())
	// by default the references are not followed, and the answer says how many are left out
	r, err := e.Callers("H", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Edges) != 0 || r.Omitted["refs"] != 1 || !anyHas(r.Hints, "--refs") {
		t.Errorf("%+v", r)
	}
	r, err = e.Callers("H", graphq.Options{Refs: true})
	if err != nil {
		t.Fatal(err)
	}
	ed := hasEdge(&r.Graph, "main", "H", "ref")
	if ed == nil || ed.At != "r.go:11:2" || ed.ViaCallee != "Reg.Get" || ed.RefKind != "arg" || ed.Candidates != 0 || len(ed.Sites) != 0 || r.Omitted["refs"] != 0 || !anyHas(r.Filters, "refs added") {
		t.Fatalf("%+v %+v", ed, r)
	}
	// T is named by three functions
	r, _ = e.Callers("T", graphq.Options{Refs: true})
	if len(r.Edges) != 3 {
		t.Errorf("%+v", r.Edges)
	}
	// callees: the functions main holds as values
	c, err := e.Callees("main", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(&c.Graph, "main", "H", "") != nil || c.Omitted["refs"] != 3 {
		t.Errorf("%+v %+v", c.Edges, c.Omitted)
	}
	c, _ = e.Callees("main", graphq.Options{Refs: true})
	if ed := hasEdge(&c.Graph, "main", "H1", "ref"); ed == nil || ed.At != "r.go:13:2" || hasEdge(&c.Graph, "main", "Lit", "ref") == nil || hasEdge(&c.Graph, "main", "A", "static") == nil {
		t.Errorf("%+v", c.Edges)
	}
	// a reference is not a call of a function value
	for _, ed := range c.Edges {
		if ed.Kind == "ref" && ed.Candidates != 0 {
			t.Errorf("%+v", ed)
		}
	}
}

// A function that is only named as a value is told apart from a call of a function value: the candidates of the call stay "value".
func TestRefsAndValueCallsAreNotMixed(t *testing.T) {
	entryIDs = map[string]bool{"main": true}
	e := build(t, []fnDef{
		{"main", []call{{"m.go", 1, 1, "ref", "Reg.Get", []string{"H"}}, {"m.go", 2, 1, "val", "f", []string{"H"}}}},
		{"H", nil},
	})
	r, _ := e.Callers("H", graphq.Options{Refs: true})
	if hasEdge(&r.Graph, "main", "H", "ref") == nil || hasEdge(&r.Graph, "main", "H", "value") == nil || len(r.Edges) != 2 {
		t.Errorf("%+v", r.Edges)
	}
	// the call is a better edge than the reference between the same two functions
	p, _ := e.Path("main", "H", graphq.Options{})
	if !p.Reachable || p.Certainty != "uncertain" || p.Relation != "calls" || p.Paths[0].Steps[0].Kind != "value" {
		t.Errorf("%+v", p)
	}
}

func TestEntryNoPathWithReferences(t *testing.T) {
	entryIDs = map[string]bool{"main": true}
	e := build(t, []fnDef{
		{"main", nil},
		{"Owner", []call{{"o.go", 7, 3, "ref", "Reg.Get", []string{"Z"}}, {"o.go", 8, 3, "ref", "Reg.Post", []string{"Z"}}}},
		{"Z", nil},
	})
	// the references are searched: no entry reaches Z even so (Owner is not reached)
	r, err := e.Path("", "Z", graphq.Options{FromEntry: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Reachable || !r.Exhaustive || !r.EdgesExhausted || strings.Join(r.Searched, ",") != "static,dynamic,hub,ref" || len(r.Limits) != 3 || !strings.Contains(strings.Join(r.Limits, " "), "reflection") {
		t.Fatalf("%+v", r)
	}
	if len(r.TargetFacts) != 1 || r.TargetFacts[0].ValueReferences == nil || *r.TargetFacts[0].ValueReferences != 2 ||
		len(r.TargetFacts[0].ValueReferenceOwners) != 1 || r.TargetFacts[0].ValueReferenceOwners[0].Func != "Owner" || r.TargetFacts[0].ValueReferenceOwners[0].Count != 2 {
		t.Errorf("%+v", r.TargetFacts)
	}
	// without the references: the old answer, not exhaustive
	r, _ = e.Path("", "Z", graphq.Options{FromEntry: true, NoRefs: true})
	if r.Reachable || r.Exhaustive || !r.EdgesExhausted || len(r.Limits) != 3 || !strings.Contains(strings.Join(r.Limits, " "), "stored as a value") || r.TargetFacts[0].ValueReferences == nil {
		t.Errorf("%+v", r)
	}
	// a handler registered by the entry is reached, the first step is the entry
	entryIDs = map[string]bool{"main": true}
	e = build(t, []fnDef{
		{"main", []call{{"w.go", 30, 2, "ref", "Route.Get", []string{"Activity"}}}},
		{"Activity", nil},
	})
	r, _ = e.Path("", "Activity", graphq.Options{FromEntry: true})
	if !r.Reachable || r.Paths[0].Steps[0].Func != "main" || r.Paths[0].Steps[0].At != "w.go:30:2" || r.Paths[0].Steps[0].ViaCallee != "Route.Get" || r.Entry == nil {
		t.Errorf("%+v", r)
	}
}

// What the taint analysis reaches (sg.Program.Reachable), the graph reaches from the entries by the calls and the references.
func TestReferencesCoverReachable(t *testing.T) {
	e := build(t, refProgram())
	want := lastProgram.Reachable()
	if want == nil {
		t.Fatal("no reachable set")
	}
	n := 0
	for f := range want {
		r, err := e.Path("", f.ID, graphq.Options{FromEntry: true})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Reachable {
			t.Errorf("Reachable has %s, the graph has no path", f.ID)
		}
		n++
	}
	if n == 0 {
		t.Fatal("empty")
	}
	// the other way: what the graph reaches by a reference that Reachable does not follow is a closure that captured a value
	for _, id := range []string{"A", "H", "H1", "T", "B1", "B2", "Store", "S", "Lit"} {
		r, _ := e.Path("", id, graphq.Options{FromEntry: true})
		if !r.Reachable {
			t.Errorf("%s: no path", id)
		}
		var in bool
		for f := range want {
			in = in || f.ID == id
		}
		if !in && id == "Lit" {
			// a closure that captured a value and was passed to a call: sg.Program.Reachable does not follow it (its
			// make_closure is not a function value, KFunc), the graph does, as a reference of kind closure
			t.Logf("%s: the graph reaches it by a closure, Reachable does not", id)
		} else if !in {
			t.Errorf("%s: the graph reaches it, Reachable does not", id)
		}
	}
	for f := range want {
		if f.ID == "Unref" {
			t.Error("Unref is reached")
		}
	}
}
