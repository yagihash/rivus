package graphq_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/graphq"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// A program written by hand, as the IR has it: a function is its ID and its calls.
type call struct {
	file      string
	line, col int
	// kind: "" static, "builtin", "dyn" (an interface), "val" (a function value);
	// "ref": a static call of callee that takes the functions of cands as arguments (named as values);
	// "closure": the same with closures that capture a value; "closure0": the same, the instruction that makes the closure has no position; "store": the functions of cands are
	// held by an instruction that is not a call (the callee is ignored)
	kind   string
	callee string // the function (static), or the name of the method (dynamic)
	cands  []string
}

type fnDef struct {
	id    string
	calls []call
}

// the functions that are a start of the program, and the exported ones (a library's own API), for the next build
var entryIDs, exportedIDs map[string]bool

var offset = 0

// the program the last build made, for the tests that compare it with sg.Program.Reachable
var lastProgram *sg.Program

// instrs are the instructions of a call: the ones that make the function values it takes, and the call.
func (c call) instrs(fn *ir.Func) []ir.Instr {
	if c.kind != "ref" && c.kind != "closure" && c.kind != "closure0" && c.kind != "store" {
		return []ir.Instr{c.instr()}
	}
	var out []ir.Instr
	var args []ir.LocalID
	for _, f := range c.cands {
		id := ir.LocalID(len(fn.Locals))
		fn.Locals = append(fn.Locals, ir.Local{Class: ir.ClassFunc})
		mc := ir.Instr{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{id}, Func: ir.FuncID(f)}
		if c.kind == "closure" || c.kind == "closure0" {
			mc.Args = []ir.LocalID{0}
		}
		if c.kind == "closure" {
			mc.Pos = ir.Pos{File: c.file, Line: c.line, Col: c.col - 1, Offset: 5000 + int(id)}
		}
		out = append(out, mc)
		args = append(args, id)
	}
	offset++
	pos := ir.Pos{File: c.file, Line: c.line, Col: c.col, Offset: offset}
	if c.kind == "store" {
		dst := ir.LocalID(len(fn.Locals))
		fn.Locals = append(fn.Locals, ir.Local{Class: ir.ClassFunc})
		return append(out, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{dst}, Args: args, Pos: pos})
	}
	return append(out, ir.Instr{Op: ir.OpCall, Args: args, Callee: &ir.Callee{Kind: ir.CalleeStatic, Func: ir.FuncID(c.callee), Symbol: c.callee}, Pos: pos})
}

func (c call) instr() ir.Instr {
	offset++
	in := ir.Instr{Op: ir.OpCall, Pos: ir.Pos{File: c.file, Line: c.line, Col: c.col, Offset: offset}}
	var cands []ir.FuncID
	for _, s := range c.cands {
		cands = append(cands, ir.FuncID(s))
	}
	switch c.kind {
	case "":
		in.Callee = &ir.Callee{Kind: ir.CalleeStatic, Func: ir.FuncID(c.callee), Symbol: c.callee}
	case "builtin":
		in.Callee = &ir.Callee{Kind: ir.CalleeStatic, Symbol: "builtin." + c.callee}
	case "dyn":
		in.Callee = &ir.Callee{Kind: ir.CalleeDynamic, Symbol: c.callee, Candidates: cands}
		in.Args = []ir.LocalID{0}
	case "val":
		in.Callee = &ir.Callee{Kind: ir.CalleeValue, Symbol: c.callee, Candidates: cands}
		in.Args = []ir.LocalID{0}
	}
	return in
}

func build(t *testing.T, fns []fnDef, ss ...*sites.Site) *graphq.Engine {
	t.Helper()
	offset = 0
	p := &ir.Program{}
	for i, f := range fns {
		file := "a.go"
		if len(f.calls) > 0 {
			file = f.calls[0].file
		}
		// "V@@1": the function of the ID "V@@1", whose symbol is "V" (a method and its wrapper)
		sym := f.id
		if i := strings.Index(sym, "@@"); i >= 0 {
			sym = sym[:i]
		}
		fn := &ir.Func{ID: ir.FuncID(f.id), Symbol: sym, Pos: ir.Pos{File: file, Line: 1 + i, Col: 6, Offset: 1000 + i}}
		fn.Entry, fn.Exported = entryIDs[f.id], exportedIDs[f.id]
		fn.Params = []ir.Param{{Name: "p", Class: ir.ClassFunc}}
		fn.Locals = []ir.Local{{Class: ir.ClassFunc}}
		fn.Roots = []ir.BlockID{0}
		b := ir.Block{Term: ir.Term{Kind: ir.TermReturn}, Instrs: []ir.Instr{{Op: ir.OpParam, Dsts: []ir.LocalID{0}, Index: 0}}}
		for _, c := range f.calls {
			b.Instrs = append(b.Instrs, c.instrs(fn)...)
		}
		fn.Blocks = []ir.Block{b}
		p.Funcs = append(p.Funcs, fn)
	}
	entryIDs, exportedIDs = nil, nil
	g, err := sg.Build(p)
	if err != nil {
		t.Fatal(err)
	}
	lastProgram = g
	path := filepath.Join(t.TempDir(), "g.db")
	_, err = graphdb.Write(path, &graphdb.Input{Graph: g, Sites: ss, Meta: graphdb.Meta{RivusVersion: "t", CreatedAt: time.Now(), SourceHash: "h", Dir: "/proj"}})
	if err != nil {
		t.Fatal(err)
	}
	db, err := graphdb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return graphq.New(db)
}

func cands(n int, prefix string) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%s%d", prefix, i))
	}
	return out
}

// the program of the tests
//
//	main -> A (static), len (builtin), I.M (dynamic: X, Y), f() (value: X)
//	A -> B -> C, B -> A (a cycle), C -> C (itself)
//	X.M -> D;  Y.M -> (nothing)
//	two calls on one line (a.go:20)
//	F: a dynamic call with 5 candidates (a frontier), one with 2 (followed)
//	Hub: a dynamic call with 25 candidates, one of which (H3) calls E
//	T is called by U0..U9
func program() []fnDef {
	fns := []fnDef{
		{"main", []call{
			{"a.go", 10, 2, "", "A", nil},
			{"a.go", 11, 2, "builtin", "len", nil},
			{"a.go", 12, 2, "dyn", "I.M", []string{"X.M", "Y.M"}},
			{"a.go", 13, 2, "val", "f", []string{"X.M"}},
		}},
		{"A", []call{{"a.go", 14, 2, "", "B", nil}, {"a.go", 15, 2, "builtin", "append", nil}}},
		{"B", []call{{"a.go", 16, 2, "", "C", nil}, {"a.go", 17, 2, "", "A", nil}}},
		{"C", []call{{"a.go", 18, 2, "", "C", nil}}},
		{"X.M", []call{{"a.go", 19, 2, "", "D", nil}}},
		{"Y.M", nil},
		{"D", nil},
		{"TwoOnALine", []call{{"a.go", 20, 5, "", "A", nil}, {"a.go", 20, 15, "", "D", nil}}},
		{"F", []call{
			{"a.go", 30, 2, "dyn", "J.N", cands(5, "P")},
			{"a.go", 31, 2, "dyn", "K.N", []string{"X.M", "Y.M"}},
			{"a.go", 32, 2, "dyn", "L.N", nil},
		}},
		{"Hub", []call{{"a.go", 40, 2, "dyn", "W.Write", cands(25, "H")}}},
		{"H3", []call{{"a.go", 41, 2, "", "E", nil}}},
		{"E", nil},
		{"T", nil},
		{"Dup", []call{{"a.go", 50, 3, "", "D", nil}, {"a.go", 50, 3, "", "E", nil}}},
	}
	for _, h := range cands(25, "H") {
		if h != "H3" {
			fns = append(fns, fnDef{h, nil})
		}
	}
	for i := 0; i < 10; i++ {
		fns = append(fns, fnDef{fmt.Sprintf("U%d", i), []call{{"b.go", 1 + i, 2, "", "T", nil}}})
	}
	return fns
}

func engine(t *testing.T) *graphq.Engine {
	return build(t, program(), &sites.Site{ID: "s1:aaa", Role: "sink", Kind: "call", Func: "main", Symbol: "A", Position: ir.Position{File: "a.go", Line: 10, Column: 2, Offset: 1}})
}

func amb(v any) graphq.CallSiteOut { return v.(graphq.CallSiteOut) }

func ids(g *graphq.Graph) map[string]int {
	m := map[string]int{}
	for _, n := range g.Nodes {
		m[n.ID] = n.Depth
	}
	return m
}

func hasEdge(g *graphq.Graph, from, to, kind string) *graphq.EdgeOut {
	for i, e := range g.Edges {
		if e.From == from && e.To == to && (kind == "" || e.Kind == kind) {
			return &g.Edges[i]
		}
	}
	return nil
}

func TestCallersStaticAndDynamic(t *testing.T) {
	e := engine(t)
	r, err := e.Callers("X.M", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// X.M is a candidate of the dynamic call of main (two candidates) and of its value call (one)
	d := hasEdge(&r.Graph, "main", "X.M", "interface")
	v := hasEdge(&r.Graph, "main", "X.M", "value")
	if d == nil || d.Candidates != 2 || v == nil || v.Candidates != 1 {
		t.Fatalf("edges: %+v", r.Edges)
	}
	if hasEdge(&r.Graph, "F", "X.M", "interface").Candidates != 2 {
		t.Errorf("F: %+v", r.Edges)
	}
	if d.At != "a.go:12:2" {
		t.Errorf("at: %v", d.At)
	}
	// the static caller carries no candidates
	r, _ = e.Callers("A", graphq.Options{})
	if s := hasEdge(&r.Graph, "main", "A", "static"); s == nil || s.Candidates != 0 {
		t.Errorf("%+v", r.Edges)
	}
	if hasEdge(&r.Graph, "B", "A", "static") == nil || hasEdge(&r.Graph, "TwoOnALine", "A", "static") == nil {
		t.Errorf("%+v", r.Edges)
	}
	// the site of the call is on the edge
	if s := hasEdge(&r.Graph, "main", "A", ""); len(s.Sites) != 1 || s.Sites[0] != "s1:aaa" {
		t.Errorf("sites: %+v", s)
	}
}

func TestCallersDepthAndTruncation(t *testing.T) {
	e := engine(t)
	r, _ := e.Callers("D", graphq.Options{})
	if _, ok := ids(&r.Graph)["X.M"]; !ok || len(r.Nodes) != 4 { // D, X.M, TwoOnALine, Dup
		t.Fatalf("%+v", r.Nodes)
	}
	if r.Truncated.BeyondDepth != 1 { // X.M has callers; TwoOnALine has none
		t.Errorf("beyond depth: %+v", r.Truncated)
	}
	r, _ = e.Callers("D", graphq.Options{Depth: 2})
	if ids(&r.Graph)["main"] != 2 || ids(&r.Graph)["F"] != 2 {
		t.Errorf("%+v", r.Nodes)
	}
	// the depth is bounded, and it is said
	r, _ = e.Callers("D", graphq.Options{Depth: 99})
	if r.Query.Depth != graphq.MaxDepth || len(r.Warnings) == 0 {
		t.Errorf("%+v %v", r.Query, r.Warnings)
	}
	// a limit of edges: what is cut is counted
	r, _ = e.Callers("T", graphq.Options{Limit: 3})
	if len(r.Edges) != 3 || r.Truncated.Count != 7 || r.Truncated.Limit != 3 || r.Truncated.Unit != "edges" {
		t.Errorf("%d edges, %+v", len(r.Edges), r.Truncated)
	}
	r, _ = e.Callers("T", graphq.Options{})
	if len(r.Edges) != 10 || r.Truncated.Count != 0 {
		t.Errorf("%+v", r.Truncated)
	}
	// a cycle, and a function that calls itself: each is a node once
	r, _ = e.Callers("C", graphq.Options{Depth: 6})
	if hasEdge(&r.Graph, "C", "C", "") == nil || len(r.Nodes) != 5 { // C, B, A, main, TwoOnALine
		t.Errorf("%+v", r.Nodes)
	}
	seen := map[string]bool{}
	for _, n := range r.Nodes {
		if seen[n.ID] {
			t.Errorf("%s twice", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestCalleesFollowsStaticAndStopsAtDynamic(t *testing.T) {
	e := engine(t)
	r, err := e.Callees("main", graphq.Options{Depth: 3})
	if err != nil {
		t.Fatal(err)
	}
	// a dynamic call with at most 3 candidates is followed (D is reached through X.M)
	if hasEdge(&r.Graph, "main", "X.M", "interface") == nil || hasEdge(&r.Graph, "X.M", "D", "static") == nil || ids(&r.Graph)["D"] != 2 {
		t.Errorf("%+v", r.Edges)
	}
	if len(r.Frontier) != 0 {
		t.Errorf("%+v", r.Frontier)
	}
	// builtins are left out, and counted: len in main, append in A
	if r.Omitted["builtin"] != 2 {
		t.Errorf("omitted: %v", r.Omitted)
	}
	for _, n := range r.Nodes {
		if n.Kind == "builtin" {
			t.Errorf("a builtin: %+v", n)
		}
	}
	r, _ = e.Callees("main", graphq.Options{Depth: 3, IncludeBuiltin: true})
	if hasEdge(&r.Graph, "main", "builtin:len", "builtin") == nil || r.Omitted["builtin"] != 0 {
		t.Errorf("%+v %v", r.Edges, r.Omitted)
	}
	// the frontier: F's call with 5 candidates and the one with none; the one with 2 is followed
	r, _ = e.Callees("F", graphq.Options{})
	if r.FrontierTotal != 2 || len(r.Frontier) != 2 {
		t.Fatalf("%+v", r.Frontier)
	}
	f := r.Frontier[0]
	if f.At != "a.go:30:2" || f.Kind != "interface" || f.Candidates != 5 || f.Callee != "J.N" || f.From != "F" {
		t.Errorf("%+v", f)
	}
	if r.Frontier[1].Candidates != 0 {
		t.Errorf("%+v", r.Frontier[1])
	}
	if hasEdge(&r.Graph, "F", "X.M", "interface") == nil || len(r.Edges) != 2 {
		t.Errorf("%+v", r.Edges)
	}
	if len(r.Hints) == 0 {
		t.Errorf("a frontier says so in the hints")
	}
	// the hub is a frontier too
	r, _ = e.Callees("Hub", graphq.Options{})
	if r.FrontierTotal != 1 || r.Frontier[0].Candidates != 25 || len(r.Edges) != 0 {
		t.Errorf("%+v", r)
	}
	// the depth, and the count of what is beyond it
	r, _ = e.Callees("main", graphq.Options{})
	if r.Truncated.BeyondDepth == 0 {
		t.Errorf("%+v", r.Truncated)
	}
}

func TestPositionsAndSpecifications(t *testing.T) {
	e := engine(t)
	// file:line:col is the call; callers of it are the callers of the function that holds it
	r, err := e.Callers("a.go:14:2", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Start[0] != "A" || hasEdge(&r.Graph, "main", "A", "") == nil || r.CallSite == nil || r.CallSite.At != "a.go:14:2" {
		t.Errorf("%+v", r)
	}
	// callees of it are the functions it reaches
	c, err := e.Callees("a.go:12:2", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(&c.Graph, "main", "X.M", "interface") == nil || hasEdge(&c.Graph, "main", "Y.M", "interface") == nil || c.Start[0] != "main" {
		t.Errorf("%+v", c.Edges)
	}
	// an absolute path
	if r, err = e.Callers("/proj/a.go:14:2", graphq.Options{}); err != nil || r.Start[0] != "A" {
		t.Errorf("%v", err)
	}
	// a line with one call (the builtin one is left out of the choice and counted)
	if r, err = e.Callers("a.go:11", graphq.Options{}); err == nil {
		t.Errorf("a line with only a builtin: %+v", r)
	} else if !strings.Contains(err.Error(), "builtin") {
		t.Error(err)
	}
	if r, err = e.Callers("a.go:11:2", graphq.Options{IncludeBuiltin: true}); err != nil || r.Start[0] != "main" {
		t.Errorf("%v", err)
	}
	// a line with several calls: no error, the choice is the answer
	r, err = e.Callers("a.go:20", graphq.Options{})
	if err != nil || len(r.Ambiguous) != 2 || len(r.Edges) != 0 {
		t.Fatalf("%v %+v", err, r)
	}
	if amb(r.Ambiguous[0]).At != "a.go:20:5" || amb(r.Ambiguous[1]).At != "a.go:20:15" || amb(r.Ambiguous[0]).In != "TwoOnALine" {
		t.Errorf("%+v", r.Ambiguous)
	}
	if len(r.Hints) == 0 {
		t.Error("the choice has a hint")
	}
	// a column inside a call takes the call that has begun
	if r, err = e.Callers("a.go:20:9", graphq.Options{}); err != nil || r.CallSite.At != "a.go:20:5" {
		t.Errorf("%v %+v", err, r)
	}
	// the site
	if r, err = e.Callers("site:s1:aaa", graphq.Options{}); err != nil || r.Start[0] != "main" || r.CallSite.At != "a.go:10:2" {
		t.Errorf("%v %+v", err, r)
	}
	if r, err = e.Callers("s1:aaa", graphq.Options{}); err != nil || r.Start[0] != "main" {
		t.Errorf("%v", err)
	}
	// the end of a symbol, the package left out, when only one has it
	if _, err = e.Callers("E", graphq.Options{}); err != nil {
		t.Errorf("%v", err)
	}
}

func TestUnknownTargetsSayWhatIsNear(t *testing.T) {
	e := engine(t)
	for _, c := range []struct{ spec, want string }{
		{"Hubb", "no function"},
		{"nofile.go:3:4", "no file"},
		{"site:s1:nope", "no site"},
		{"a.go:99", "no call at"},
		{"a.go:3", "begins last before"},
		{"", "empty"},
	} {
		_, err := e.Callers(c.spec, graphq.Options{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v", c.spec, err)
		}
	}
	// the same name in other words
	_, err := e.Callers("pkg/E", graphq.Options{})
	if err == nil || !strings.Contains(err.Error(), "near it") {
		t.Errorf("%v", err)
	}
}

func pathIDs(p graphq.Path) string {
	var s []string
	for _, st := range p.Steps {
		s = append(s, st.Func)
	}
	return strings.Join(s, ">")
}

func TestPathStaticFirst(t *testing.T) {
	e := engine(t)
	r, err := e.Path("main", "C", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Reachable || r.Certainty != "certain" || len(r.Paths) != 1 || pathIDs(r.Paths[0]) != "main>A>B>C" {
		t.Fatalf("%+v", r.Paths)
	}
	p := r.Paths[0]
	if p.Certainty != "certain" || p.Hops != 3 || p.Steps[0].At != "a.go:10:2" || p.Steps[0].Kind != "static" || p.Steps[3].At != "" {
		t.Errorf("%+v", p)
	}
	if len(p.Steps[0].Sites) != 1 || p.Steps[0].Sites[0] != "s1:aaa" {
		t.Errorf("site: %+v", p.Steps[0])
	}
	if r.Truncated.Count != 0 || !r.Exhaustive || strings.Join(r.Searched, ",") != "static" || r.HubSkippedTotal != 0 {
		t.Errorf("%+v", r)
	}
	// a static path wins over a dynamic one: main reaches D by the interface only
	// (a path through the dynamic calls is returned only when there is none by the static ones)
	r, _ = e.Path("main", "D", graphq.Options{})
	if !r.Reachable || r.Certainty != "uncertain" || pathIDs(r.Paths[0]) != "main>X.M>D" {
		t.Fatalf("%+v", r.Paths)
	}
	st := r.Paths[0].Steps[0]
	if st.Kind == "static" || st.Candidates == 0 || r.Paths[0].Certainty != "uncertain" {
		t.Errorf("a dynamic step has its kind and candidates: %+v", st)
	}
	// the function itself, and a path against the direction
	if r, _ = e.Path("A", "A", graphq.Options{}); !r.Reachable || len(r.Paths) < 1 || r.Paths[0].Hops != 0 {
		t.Errorf("%+v", r)
	}
	if r, _ = e.Path("D", "main", graphq.Options{}); r.Reachable || r.Certainty != "" || !r.Exhaustive || len(r.Paths) != 0 {
		t.Errorf("%+v", r)
	}
	// by a cycle: B to A, and A to B, the shortest
	if r, _ = e.Path("A", "A", graphq.Options{}); r.Paths[0].Hops != 0 {
		t.Errorf("%+v", r.Paths)
	}
	if r, _ = e.Path("B", "C", graphq.Options{}); pathIDs(r.Paths[0]) != "B>C" {
		t.Errorf("%+v", r.Paths)
	}
}

func TestPathKPathsWithoutDuplicates(t *testing.T) {
	// two ways from S to Z through the same functions' pair twice: M1 and M2, and a longer one by L
	fns := []fnDef{
		{"S", []call{{"p.go", 1, 1, "", "M1", nil}, {"p.go", 2, 1, "", "M1", nil}, {"p.go", 3, 1, "", "M2", nil}, {"p.go", 4, 1, "", "L", nil}}},
		{"M1", []call{{"p.go", 5, 1, "", "Z", nil}}},
		{"M2", []call{{"p.go", 6, 1, "", "Z", nil}}},
		{"L", []call{{"p.go", 7, 1, "", "L2", nil}}},
		{"L2", []call{{"p.go", 8, 1, "", "Z", nil}}},
		{"Z", nil},
	}
	e := build(t, fns)
	r, err := e.Path("S", "Z", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range r.Paths {
		got = append(got, pathIDs(p))
	}
	if strings.Join(got, " ") != "S>M1>Z S>M2>Z S>L>L2>Z" {
		t.Errorf("%v", got)
	}
	// the number of paths is bounded, and the cut is said
	r, _ = e.Path("S", "Z", graphq.Options{Limit: 2})
	if len(r.Paths) != 2 || r.Truncated.Count != 1 || !r.Truncated.AtLeast || r.Truncated.Limit != 2 || r.Truncated.Unit != "paths" {
		t.Errorf("%+v %d", r.Truncated, len(r.Paths))
	}
	// --limit 1: the shortest one
	r, _ = e.Path("S", "Z", graphq.Options{Limit: 1})
	if len(r.Paths) != 1 || pathIDs(r.Paths[0]) != "S>M1>Z" {
		t.Errorf("%+v", r.Paths)
	}
}

func TestPathFromAndToACall(t *testing.T) {
	e := engine(t)
	// from the call a.go:14:2 (A calls B): the path starts in A, through that call
	r, err := e.Path("a.go:14:2", "C", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if pathIDs(r.Paths[0]) != "A>B>C" || r.Paths[0].Steps[0].At != "a.go:14:2" {
		t.Errorf("%+v", r.Paths)
	}
	// to the call a.go:16:2 (B calls C): the path ends in B, with that call
	r, err = e.Path("main", "a.go:16:2", graphq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	last := r.Paths[0].Steps[len(r.Paths[0].Steps)-1]
	if pathIDs(r.Paths[0]) != "main>A>B" || last.At != "a.go:16:2" {
		t.Errorf("%+v", r.Paths)
	}
	// a line with two calls: the choice
	r, err = e.Path("a.go:20", "D", graphq.Options{})
	if err != nil || len(r.Ambiguous) != 2 || r.Reachable {
		t.Errorf("%v %+v", err, r)
	}
}

func TestEdgePredicateFiltersWhatIsFollowed(t *testing.T) {
	e := engine(t)
	// a filter on the edges (the stage 3 gives it a syntax): never through B
	pred := func(ed *graphdb.Edge) bool {
		f, err := e.FuncID(ed.To)
		return err == nil && f != "B"
	}
	r, _ := e.Path("main", "C", graphq.Options{Edge: pred})
	if r.Reachable {
		t.Errorf("%+v", r.Paths)
	}
	c, _ := e.Callees("A", graphq.Options{Edge: pred})
	if hasEdge(&c.Graph, "A", "B", "") != nil {
		t.Errorf("%+v", c.Edges)
	}
}

// Every answer has the fields the schema names, and the ones a reader needs to know what was cut.
func TestOutputSchema(t *testing.T) {
	e := engine(t)
	callers, _ := e.Callers("A", graphq.Options{})
	callees, _ := e.Callees("main", graphq.Options{})
	path, _ := e.Path("main", "C", graphq.Options{})
	nopath, _ := e.Path("D", "main", graphq.Options{})
	amb, _ := e.Callers("a.go:20", graphq.Options{})
	common := []string{"schema", "relation", "flows", "snapshot", "query", "truncated", "omitted", "filters", "hints"}
	for name, c := range map[string]struct {
		v    any
		want []string
	}{
		"callers": {callers, append([]string{"start", "nodes", "edges"}, common...)},
		"callees": {callees, append([]string{"start", "nodes", "edges", "frontier", "frontier_total"}, common...)},
		"path":    {path, append([]string{"reachable", "certainty", "paths", "searched", "exhaustive"}, common...)},
		"nopath":  {nopath, append([]string{"reachable", "paths", "searched", "exhaustive"}, common...)},
		"amb":     {amb, append([]string{"ambiguous"}, common...)},
	} {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		json.Unmarshal(b, &m)
		for _, k := range c.want {
			if _, ok := m[k]; !ok {
				t.Errorf("%s: no %q in %s", name, k, b)
			}
		}
		if string(m["schema"]) != `"rivus.graph/1"` || string(m["relation"]) != `"calls"` || string(m["flows"]) != `"not_checked"` || string(m["filters"]) != `[]` {
			t.Errorf("%s: %s %s %s", name, m["schema"], m["relation"], m["filters"])
		}
		var tr struct{ Count, Limit int }
		if json.Unmarshal(m["truncated"], &tr); tr.Limit == 0 {
			t.Errorf("%s: truncated %s", name, m["truncated"])
		}
		var om map[string]int
		json.Unmarshal(m["omitted"], &om)
		if _, ok := om["builtin"]; !ok {
			t.Errorf("%s: omitted %s", name, m["omitted"])
		}
	}
}

// Two calls at one place of one function (a desugared "a()?") are one call: no choice to make.
func TestCallsAtOnePlaceAreOne(t *testing.T) {
	e := engine(t)
	c, err := e.Callees("a.go:50:3", graphq.Options{})
	if err != nil || len(c.Ambiguous) != 0 || hasEdge(&c.Graph, "Dup", "D", "") == nil || hasEdge(&c.Graph, "Dup", "E", "") == nil {
		t.Errorf("%v %+v", err, c)
	}
	if r, err := e.Callers("a.go:50", graphq.Options{}); err != nil || len(r.Ambiguous) != 0 || r.Start[0] != "Dup" {
		t.Errorf("%v %+v", err, r)
	}
}
