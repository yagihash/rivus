package main

import (
	"bytes"
	"database/sql"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupNames(t *testing.T) {
	fids := []string{
		"m/p.F", "(m/p.T).M", "(*m/p.T).M", "(*m/p.G[T]).Get", "m/p.Map[T]", "m/p.F$1", "m/p.F$1$1", "m/p.F$2",
		"m/p.init#1", "m/p.init#2", "m/p.init#2$1", "(m/p.T).M$1", "m/p.U", "(*m/p.U).P",
	}
	x := NewFuncIndex(fids)
	for name, want := range map[string]string{
		"m/p.F":                  "m/p.F",
		"m/p.T.M":                "(m/p.T).M",
		"m/p.(*T).M":             "(*m/p.T).M",
		"m/p.(*U).P":             "(*m/p.U).P",
		"m/p.(*G[...]).Get":      "(*m/p.G[T]).Get",
		"m/p.Map[...]":           "m/p.Map[T]",
		"m/p.F.func1":            "m/p.F$1",
		"m/p.F.func1.1":          "m/p.F$1$1", // not inlined
		"m/p.F.func1.func1":      "m/p.F$1$1", // inlined
		"m/p.F.func2":            "m/p.F$2",
		"m/p.init.0":             "m/p.init#1",
		"m/p.init.1.func1":       "m/p.init#2$1",
		"m/p.T.M.func1":          "(m/p.T).M$1",
		"m/p.Missing":            "",
		"other/q.F":              "",
		"m/p.T.Missing":          "",
		"m/p.(*T).Missing.func1": "",
	} {
		if got := x.Lookup(name); got != want {
			t.Errorf("Lookup(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestInstrumentSource(t *testing.T) {
	src := []byte("package p // doc\n\nimport \"fmt\"\n\nfunc F() {\n\tg := func() { fmt.Println() }\n\tg()\n}\n\ntype T struct{}\n\nfunc (T) M() {}\n\nfunc decl()\n")
	out, n, err := InstrumentSource("p.go", src, "m/rvobs")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("%d functions, want 3 (F, the literal, M; decl has no body)", n)
	}
	if got, want := strings.Count(string(out), "\n"), strings.Count(string(src), "\n"); got != want {
		t.Errorf("the number of lines changed: %d, was %d", got, want)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "p.go", out, 0); err != nil {
		t.Fatalf("the instrumented source does not parse: %v\n%s", err, out)
	}
	if got := strings.Count(string(out), "rvobs.Enter()"); got != 3 {
		t.Errorf("%d calls of Enter, want 3", got)
	}
	// what was inserted is all on the lines that were there: taking it out gives the original
	back := strings.ReplaceAll(string(out), enterText, "")
	back = strings.Replace(back, "; import rvobs \"m/rvobs\"; ", "", 1)
	if strings.TrimSpace(back) != strings.TrimSpace(string(src)) {
		t.Errorf("the original is not what is left:\n%s", back)
	}
}

func makeGraph(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "g.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT)`,
		`CREATE TABLE funcs (id INTEGER PRIMARY KEY, fid TEXT)`,
		`CREATE TABLE callsites (id INTEGER PRIMARY KEY, func_id INT, file_id INT, line INT, kind TEXT, ncand INT, callee TEXT)`,
		`CREATE TABLE edges (callsite_id INT, to_func INT)`,
		`INSERT INTO files VALUES (1, 'cells/a/a.go')`,
		`INSERT INTO funcs VALUES (1,'m/a.main'),(2,'m/a.f'),(3,'m/a.g'),(4,'(m/a.T).M$bound'),(5,'(m/a.T).M'),(6,'m/a.h'),(7,'m/a.rng')`,
		// main:10 calls f (static); main:11 calls a value with two candidates f and g; main:12 calls the bound method;
		// main:13 calls h through nothing (no edge); rng has a call without a line
		`INSERT INTO callsites VALUES (1,1,1,10,'static',1,''),(2,1,1,11,'value',2,''),(3,1,1,12,'value',1,''),(4,1,1,13,'value',1,''),(5,4,1,0,'static',1,''),(6,7,1,0,'value',1,'')`,
		`INSERT INTO edges VALUES (1,2),(2,2),(2,3),(3,4),(5,5)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestCompareClasses(t *testing.T) {
	g, err := OpenGraph(makeGraph(t))
	if err != nil {
		t.Fatal(err)
	}
	e := func(from, to string, line int) Edge {
		return Edge{FromFID: from, ToFID: to, From: from, To: to, File: "cells/a/a.go", Line: line, Count: 1}
	}
	out := Edge{From: "x", To: "y", Outside: "harness"}
	o := Observed{Lang: "go", Edges: []Edge{
		e("m/a.main", "m/a.f", 10),                                   // ok
		e("m/a.main", "m/a.g", 11),                                   // ok, a value call with two candidates
		e("m/a.main", "(*m/a.T).M", 12),                              // ok through the relay of a method value
		e("m/a.main", "m/a.h", 13),                                   // no_edge
		e("m/a.main", "m/a.f", 99),                                   // no_callsite
		e("m/a.rng", "m/a.f", 5),                                     // outside: the call has no line
		{From: "m/a.zz", To: "m/a.f", File: "cells/a/a.go", Line: 1}, // id_unmapped
		out,
	}}
	r := Compare(o, g)
	want := []string{ClassOK, ClassOK, ClassOK, ClassNoEdge, ClassNoCallsite, ClassOutside, ClassIDUnmapped, ClassOutside}
	for i, w := range want {
		if r.Class[i] != w {
			t.Errorf("edge %d: %s, want %s", i, r.Class[i], w)
		}
	}
	if r.Note[5] != "no_position" {
		t.Errorf("note %q", r.Note[5])
	}
	if len(r.Over) != 1 || r.Over[0].Cands != 2 || r.Over[0].Observed != 1 {
		t.Errorf("over-approximation rows: %+v (only the call site with 2 candidates, one seen)", r.Over)
	}
	var b bytes.Buffer
	r.Print(&b, true)
	if !strings.Contains(b.String(), "no_edge") || !strings.Contains(b.String(), "2 candidates, 1 seen called") {
		t.Errorf("table:\n%s", b.String())
	}
}

const tinyMod = "module tiny\n\ngo 1.22\n"

const tinyCell = `package a

import "strings"

type S interface{ Do() string }
type A struct{}
type B struct{}

func (A) Do() string  { return "a" }
func (*B) Do() string { return "b" }

func run(s S) string { return s.Do() }

func Run() string {
	f := func() string { return run(A{}) }
	h := (&B{}).Do
	return strings.ToUpper(f() + run(&B{}) + h())
}
`

const tinyMain = `package main

import (
	"os"

	"tiny/cells/a"
)

func main() {
	got := a.Run()
	os.WriteFile(os.Args[2], []byte(got), 0o644)
}
`

// TestRunTiny records a small module (a call through an interface with two implementations, a closure, a method value),
// compares it with the graph that the rivus of this tree makes, and checks that every observed call is in the graph.
func TestRunTiny(t *testing.T) {
	if testing.Short() {
		t.Skip("builds rivus and runs it")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{"go.mod": tinyMod, "cells/a/a.go": tinyCell, "cmd/observe/main.go": tinyMain} {
		p := filepath.Join(dir, "zoo", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rivus := filepath.Join(dir, "rivus")
	build := exec.Command("go", "build", "-o", rivus, "./cmd/rivus")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build rivus: %v\n%s", err, out)
	}
	var b bytes.Buffer
	args := []string{"-zoo", filepath.Join(dir, "zoo"), "-rivus", rivus, "-tags", "", "-hook", "a.Run(", "-run-args", "-out " + filepath.Join(dir, "got.txt"), "-json", filepath.Join(dir, "obs.json"), "-v"}
	if err := cmdRun(args, &b); err != nil {
		t.Fatalf("%v\n%s", err, b.String())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "got.txt")); string(got) != "ABB" {
		t.Errorf("the instrumented copy changed what the code computes: %q", got)
	}
	out := b.String()
	for _, want := range []string{"no_edge      0", "no_callsite  0", "id_unmapped  0", "2 candidates, 2 seen called"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "  ok           0") {
		t.Errorf("nothing matched:\n%s", out)
	}
}
