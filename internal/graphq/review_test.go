package graphq_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphq"
)

func stepHubs(p graphq.Path) int {
	n := 0
	for _, s := range p.Steps {
		if s.Hub {
			n++
		}
	}
	return n
}

// G-4: between two functions the edge kept is the best by its rank, not the first in the file.
//
//	A calls (a.go:1) a hub of 300 candidates, B among them; then (a.go:2) a call of 5 candidates, B among them
//	B calls a hub of 25 candidates, Z among them;  A -> C (static), C calls a hub, Z among them
func TestPathKeepsTheBestEdgeBetweenTwoFunctions(t *testing.T) {
	e := build(t, withCands([]fnDef{
		{"A", []call{
			{"a.go", 1, 1, "dyn", "W.Hub", append([]string{"B"}, cands(299, "Ph")...)},
			{"a.go", 2, 1, "dyn", "I.M", append([]string{"B"}, cands(4, "Pf")...)},
			{"a.go", 3, 1, "", "C", nil},
		}},
		{"B", []call{{"b.go", 1, 1, "dyn", "W.Z", append([]string{"Z"}, cands(24, "Pz")...)}}},
		{"C", []call{{"c.go", 1, 1, "dyn", "W.Y", append([]string{"Z"}, cands(24, "Py")...)}}},
		{"Z", nil},
	}))
	r, err := e.Path("A", "Z", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || r.Certainty != "hub" {
		t.Fatalf("%+v", r)
	}
	var got []string
	for _, p := range r.Paths {
		got = append(got, pathIDs(p))
		if stepHubs(p) != 1 {
			t.Errorf("%s has %d hubs: %+v", pathIDs(p), stepHubs(p), p.Steps)
		}
	}
	// both go through one hub: the call of 5 candidates is not a hub
	if strings.Join(got, " ") != "A>B>Z A>C>Z" && strings.Join(got, " ") != "A>C>Z A>B>Z" {
		t.Fatalf("%v", got)
	}
	for _, p := range r.Paths {
		if pathIDs(p) == "A>B>Z" && (p.Steps[0].Hub || p.Steps[0].Candidates != 5) {
			t.Errorf("the first step: %+v", p.Steps[0])
		}
	}
}

// G-5: a --via function with two IDs of one symbol; the k paths of the first stretch all end at one ID.
func TestViaDoesNotLoseTheWayThroughTheOtherID(t *testing.T) {
	var s []call
	for i := 1; i <= 6; i++ {
		s = append(s, call{"s.go", i, 1, "", fmt.Sprintf("W%d", i), nil})
	}
	s = append(s, call{"s.go", 7, 1, "", "Q", nil})
	fns := []fnDef{{"S", s}}
	for i := 1; i <= 6; i++ {
		fns = append(fns, fnDef{fmt.Sprintf("W%d", i), []call{{"w.go", i, 1, "", "V@@1", nil}}})
	}
	fns = append(fns,
		fnDef{"Q", []call{{"q.go", 1, 1, "", "V@@2", nil}}},
		// the first ID reaches the end only by a dynamic call; the second by a static one
		fnDef{"V@@1", []call{{"v.go", 1, 1, "dyn", "I.M", []string{"Z", "Other"}}}},
		fnDef{"V@@2", []call{{"v.go", 2, 1, "", "Z", nil}}},
		fnDef{"Other", nil}, fnDef{"Z", nil})
	e := build(t, fns)
	r, err := e.Path("S", "Z", graphq.Options{Via: []string{"V"}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || r.Certainty != "certain" || pathIDs(r.Paths[0]) != "S>Q>V@@2>Z" || strings.Join(r.Searched, ",") != "static" {
		t.Fatalf("%+v", r)
	}
}

// G-6: --only-impl narrows the call sites that have a candidate of the type, and leaves the others.
//
//	S calls Writer.Write (bytes.Buffer, os.File); (*bytes.Buffer).Write calls Stringer.String (a.T, b.U); (*a.T).String -> Z
func TestOnlyImplLeavesTheOtherInterfaces(t *testing.T) {
	e := build(t, withCands([]fnDef{
		{"S", []call{{"s.go", 1, 1, "dyn", "io.Writer.Write", []string{"(*bytes.Buffer).Write", "(*os.File).Write"}}}},
		{"(*bytes.Buffer).Write", []call{{"b.go", 1, 1, "dyn", "fmt.Stringer.String", []string{"(*a.T).String", "(*b.U).String"}}}},
		{"(*a.T).String", []call{{"a.go", 1, 1, "", "Z", nil}}},
		{"Z", nil},
	}))
	r, err := e.Path("S", "Z", graphq.Options{OnlyImpl: []string{"bytes.Buffer"}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || pathIDs(r.Paths[0]) != "S>(*bytes.Buffer).Write>(*a.T).String>Z" {
		t.Fatalf("%+v", r)
	}
	if len(r.Filters) != 1 || !strings.Contains(r.Filters[0], "narrowed 1 call site") || !strings.Contains(r.Filters[0], "left alone 1 call site") {
		t.Errorf("filters: %v", r.Filters)
	}
	// the narrowed call site reaches only the type: os.File is not a way
	c, _ := e.Callees("S", graphq.Options{OnlyImpl: []string{"bytes.Buffer"}})
	if hasEdge(&c.Graph, "S", "(*os.File).Write", "") != nil || hasEdge(&c.Graph, "S", "(*bytes.Buffer).Write", "") == nil {
		t.Errorf("%+v", c.Edges)
	}
	// a type that no candidate of a call site has: the site is not narrowed
	c, _ = e.Callees("S", graphq.Options{OnlyImpl: []string{"Nothing"}})
	if hasEdge(&c.Graph, "S", "(*os.File).Write", "") == nil || hasEdge(&c.Graph, "S", "(*bytes.Buffer).Write", "") == nil {
		t.Errorf("%+v", c.Edges)
	}
}

// G-7: the type of a symbol is the type, not the trait, the package, or a type argument.
func TestReceiverNames(t *testing.T) {
	cases := []struct {
		sym, typ string
		want     bool
	}{
		{"<sample::traits::Printer as sample::traits::Handler>::handle", "Printer", true},
		{"<sample::traits::Printer as sample::traits::Handler>::handle", "traits::Printer", true},
		{"<sample::traits::Printer as sample::traits::Handler>::handle", "Handler", false},
		{"<sample::traits::Printer as sample::traits::Handler>::handle", "sample::traits::Handler", false},
		{"<Vec<Foo> as sample::Tr>::run", "Foo", false},
		{"<Vec<Foo> as sample::Tr>::run", "Vec", true},
		{"<Vec<Foo> as sample::Tr>::run", "Tr", false},
		{"sample::Printer::print", "Printer", true},
		{"sample::Printer::print", "sample", false},
		{"(*x/models/user.User).GetName", "user", false},
		{"(*x/models/user.User).GetName", "User", true},
		{"(*x/models/user.User).GetName", "user.User", true},
		{"(*x/models/user.User).GetName", "x/models/user.User", true},
		{"(*x/models/user.User).GetName", "models", false},
		{"(example.com/models/org.Org).Name", "Org", true},
		{"(*bytes.Buffer).Write", "bytes.Buffer", true},
		{"(*bytes.Buffer).Write", "bytes", false},
		{"(*bytes.Buffer).Write", "Buffer", true},
		{"(*bytes.Buffer).Write", "Buf", false},
		{"X.M", "X", true},
		{"X.M", "M", false},
		{"XX.M", "X", false},
		{"package:app/a.dart#Widget.build", "Widget", true},
		{"package:app/a.dart#Widget.build", "a.dart#Widget", true},
		{"package:app/a.dart#Widget.build", "build", false},
		{"X.M", "", false},
	}
	for _, c := range cases {
		if got := graphq.ReceiverNames(c.sym, c.typ); got != c.want {
			t.Errorf("receiverNames(%q, %q) = %v, want %v", c.sym, c.typ, got, c.want)
		}
	}
}

// G-7, end to end: the trait does not narrow the call to the implementations of it.
func TestOnlyImplTraitNameNarrowsNothing(t *testing.T) {
	e := build(t, withCands([]fnDef{
		{"M", []call{{"m.rs", 1, 1, "dyn", "Handler::handle", []string{"<sample::Printer as sample::Handler>::handle", "<sample::Other as sample::Quiet>::handle"}}}},
	}))
	c, _ := e.Callees("M", graphq.Options{OnlyImpl: []string{"Handler"}})
	if len(c.Edges) != 2 || !strings.Contains(c.Filters[0], "narrowed 0 call sites") {
		t.Errorf("%+v %v", c.Edges, c.Filters)
	}
	c, _ = e.Callees("M", graphq.Options{OnlyImpl: []string{"Printer"}})
	if len(c.Edges) != 1 || !strings.Contains(c.Filters[0], "narrowed 1 call site") {
		t.Errorf("%+v %v", c.Edges, c.Filters)
	}
}

// G-8: a search that a bound cut does not say that the filters are the cause.
func TestWithoutFiltersHintWhenTheSearchWasCut(t *testing.T) {
	// S -> X -> Z (short), and S -> L1 -> ... -> L40 -> Z (longer than the longest path searched)
	fns := []fnDef{
		{"S", []call{{"s.go", 1, 1, "", "X", nil}, {"s.go", 2, 1, "", "L1", nil}}},
		{"X", []call{{"x.go", 1, 1, "", "Z", nil}}},
		{"Z", nil},
	}
	for i := 1; i <= 40; i++ {
		next := fmt.Sprintf("L%d", i+1)
		if i == 40 {
			next = "Z"
		}
		fns = append(fns, fnDef{fmt.Sprintf("L%d", i), []call{{"l.go", i, 1, "", next, nil}}})
	}
	e := build(t, fns)
	r, err := e.Path("S", "Z", graphq.Options{Avoid: []string{"X"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Reachable || r.Exhaustive || r.WithoutFilters == nil || !r.WithoutFilters.Reachable {
		t.Fatalf("%+v", r)
	}
	if anyHas(r.Hints, "cuts it") || !anyHas(r.Hints, "cannot be said that the filters") {
		t.Errorf("%v", r.Hints)
	}
	// when the search was whole, the filters are the cause
	e = build(t, []fnDef{{"S", []call{{"s.go", 1, 1, "", "X", nil}}}, {"X", []call{{"x.go", 1, 1, "", "Z", nil}}}, {"Z", nil}})
	r, _ = e.Path("S", "Z", graphq.Options{Avoid: []string{"X"}})
	if !r.Exhaustive || !anyHas(r.Hints, "cuts it") {
		t.Errorf("%v", r.Hints)
	}
}

// --via is dropped too by the question without the filters.
func TestWithoutFiltersDropsVia(t *testing.T) {
	r, _ := engine(t).Path("main", "C", graphq.Options{Via: []string{"D"}})
	if r.Reachable || r.WithoutFilters == nil || !r.WithoutFilters.Reachable {
		t.Errorf("%+v", r)
	}
}

// With more than one filter: which of them, taken off, lets the path through.
func TestWithoutFiltersNamesTheFilterThatCuts(t *testing.T) {
	r, err := engine(t).Path("main", "C", graphq.Options{Avoid: []string{"B", "Y.M"}})
	if err != nil {
		t.Fatal(err)
	}
	wf := r.WithoutFilters
	if r.Reachable || wf == nil || !wf.Reachable || len(wf.Culprits) != 1 || wf.Culprits[0] != "--avoid B" {
		t.Fatalf("%+v", r)
	}
	if !anyHas(r.Hints, "taking off only") {
		t.Errorf("%v", r.Hints)
	}
}

// listed and beyond: what the paths cover, in a field.
func TestPathSaysWhatItListed(t *testing.T) {
	e := build(t, hubProgram())
	r, _ := e.Path("A0", "Z", graphq.Options{})
	l := r.Listed
	if l == nil || l.Stage != "hub" || l.Hubs == nil || *l.Hubs != 1 || l.ShortestHops != 2 || l.MaxHops != 2+graphq.PathSlack || r.Beyond != "not_searched" {
		t.Errorf("%+v %v", l, r.Beyond)
	}
	r, _ = engine(t).Path("main", "C", graphq.Options{})
	l = r.Listed
	if l == nil || l.Stage != "static" || l.Hubs != nil || l.ShortestHops != 3 || l.MaxHops != 3+graphq.PathSlack || r.Beyond != "not_searched" {
		t.Errorf("%+v %v", l, r.Beyond)
	}
	r, _ = engine(t).Path("D", "main", graphq.Options{Avoid: []string{"A"}})
	if r.Listed != nil || r.Beyond != "" {
		t.Errorf("no path, nothing listed: %+v", r)
	}
}

// --avoid-call: one call site is cut, the function is not.
func TestAvoidCall(t *testing.T) {
	e := engine(t)
	// main -> X.M by the interface call (a.go:12:2) and by the function value (a.go:13:2)
	r, err := e.Path("main", "D", graphq.Options{AvoidCall: []string{"a.go:12:2"}})
	if err != nil || !r.Reachable || r.Paths[0].Steps[0].At != "a.go:13:2" {
		t.Fatalf("%v %+v", err, r)
	}
	if len(r.Filters) != 1 || !strings.HasPrefix(r.Filters[0], "avoid-call a.go:12:2") {
		t.Errorf("filters: %v", r.Filters)
	}
	r, _ = e.Path("main", "D", graphq.Options{AvoidCall: []string{"a.go:12:2", "a.go:13:2"}})
	if r.Reachable || len(r.Filters) != 2 || r.WithoutFilters == nil || !r.WithoutFilters.Reachable {
		t.Errorf("%+v", r)
	}
	c, _ := e.Callees("main", graphq.Options{AvoidCall: []string{"a.go:12:2"}})
	if hasEdge(&c.Graph, "main", "X.M", "interface") != nil || hasEdge(&c.Graph, "main", "Y.M", "interface") != nil || hasEdge(&c.Graph, "main", "A", "") == nil {
		t.Errorf("%+v", c.Edges)
	}
	cr, _ := e.Callers("X.M", graphq.Options{AvoidCall: []string{"a.go:12:2"}})
	if hasEdge(&cr.Graph, "main", "X.M", "interface") != nil || hasEdge(&cr.Graph, "main", "X.M", "value") == nil {
		t.Errorf("%+v", cr.Edges)
	}
	// a position that is not a call, or no position
	if _, err := e.Path("main", "D", graphq.Options{AvoidCall: []string{"A"}}); err == nil || !strings.Contains(err.Error(), "--avoid-call") {
		t.Errorf("%v", err)
	}
	if _, err := e.Path("main", "D", graphq.Options{AvoidCall: []string{"a.go:20"}}); err == nil || !strings.Contains(err.Error(), "--avoid-call") {
		t.Errorf("several calls on the line: %v", err)
	}
}

func TestAvoidCallImplsAndDispatchers(t *testing.T) {
	e := implEngine(t)
	r, err := e.Impls("app.I.M", graphq.Options{AvoidCall: []string{"p.go:1:2"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := implIDs(r); got["Y.M"] != 0 || got["X.M"] != 1 || len(r.Filters) != 1 {
		t.Errorf("%v %+v", got, r.Filters)
	}
	d, err := e.Dispatchers("X.M", graphq.Options{AvoidCall: []string{"p.go:1:2"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range d.CallSites {
		if s.At == "p.go:1:2" {
			t.Errorf("%+v", d.CallSites)
		}
	}
	if len(d.CallSites) == 0 {
		t.Errorf("%+v", d)
	}
}
