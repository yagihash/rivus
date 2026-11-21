package main

import (
	"strings"
	"testing"
)

func TestLoadRows(t *testing.T) {
	rows, err := LoadRows()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]int{}
	for _, r := range rows {
		by[r.Lang]++
		if r.Symbol == "" {
			t.Errorf("a row without a key: %+v", r)
		}
	}
	for _, l := range langs {
		if by[l] == 0 {
			t.Errorf("no rows for %s", l)
		}
	}
	mi := NewModelIndex(rows)
	// a known pure row, a known sink, a known propagator that writes to its receiver
	if s := mi.ShapeOf("go", "strings.Join"); s != ShapeValue {
		t.Errorf("strings.Join is %q", s)
	}
	if !mi.HasModel("go", "os/exec.Command") {
		t.Errorf("os/exec.Command has no row")
	}
	if mi.HasModel("go", "no/such.Func") {
		t.Errorf("a row for a function that is not there")
	}
}

func TestScanIR(t *testing.T) {
	ir := `{"schema_version":"rivus.ir/0"}
{"kind":"func","func":{"id":"p.F","symbol":"p.F","file":"a.go","blocks":[{"instrs":[{"op":"call","callee":{"kind":"static","func":"p.G","symbol":"p.G"}},{"op":"call","callee":{"kind":"static","func":"strings.Join","symbol":"strings.Join"}},{"op":"call","callee":{"kind":"static","symbol":"builtin.len"}},{"op":"call","callee":{"kind":"dynamic","symbol":"(io.Writer).Write","candidates":["x.W.Write"]}}]}]}}
{"kind":"func","func":{"id":"p.G","symbol":"p.G","file":"a_test.go","blocks":[{"instrs":[{"op":"call","callee":{"kind":"static","func":"strings.Join","symbol":"strings.Join"}}]}]}}
`
	c, err := ScanIR(strings.NewReader(ir), "t", "go", true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Funcs != 2 {
		t.Errorf("funcs = %d", c.Funcs)
	}
	if g := c.Calls["p.G"]; g == nil || g.Own != 1 || g.Lib != 0 {
		t.Errorf("p.G = %+v: a function that the IR has is not a library call", g)
	}
	if j := c.Calls["strings.Join"]; j == nil || j.Lib != 2 || j.LibTest != 1 || j.Test != 1 {
		t.Errorf("strings.Join = %+v", j)
	}
	if c.Calls["builtin.len"] != nil {
		t.Errorf("a builtin is not a call to a library")
	}
	if w := c.Calls["(io.Writer).Write"]; w == nil || w.Lib != 1 {
		t.Errorf("(io.Writer).Write = %+v: a dynamic call whose candidate is not in the IR is a library call", w)
	}
	if !c.FuncSyms["p.F"] {
		t.Errorf("the symbols of the functions are not kept")
	}
}

func TestTailAndNames(t *testing.T) {
	cases := []struct{ lang, sym, tail string }{
		{"go", "(*strings.Builder).WriteString", "(strings.Builder).WriteString"},
		{"dart", "dart:core#String.trim", "String.trim"},
		{"dart", "package:path/path.dart#join", "join"},
		{"rust", "tonic::request::Request::into_inner", "Request::into_inner"},
		{"rust", "std::fs::read_to_string", "fs::read_to_string"},
	}
	for _, c := range cases {
		if got := tailOf(c.lang, c.sym); got != c.tail {
			t.Errorf("tailOf(%s, %s) = %q, want %q", c.lang, c.sym, got, c.tail)
		}
	}
	if !IsStd("go", "(*net/http.Request).FormValue") || IsStd("go", "github.com/x/y.F") || !IsStd("go", "strings.Join") {
		t.Errorf("IsStd of Go")
	}
	if !IsStd("dart", "dart:core#String.trim") || IsStd("dart", "package:path/path.dart#join") {
		t.Errorf("IsStd of Dart")
	}
	if !IsStd("rust", "std::fs::read") || IsStd("rust", "tonic::Request::into_inner") {
		t.Errorf("IsStd of Rust")
	}
	if !IsCallbackName("dart", "dart:async#Future.then") || IsCallbackName("dart", "dart:core#String.trim") {
		t.Errorf("IsCallbackName of Dart")
	}
	if !IsWriterName("rust", "std::vec::Vec::push") || IsWriterName("rust", "std::vec::Vec::len") {
		t.Errorf("IsWriterName of Rust")
	}
}

func TestCheckRows(t *testing.T) {
	rows := []Row{
		{Lang: "rust", Symbol: "axum::extract::Json::new", Call: true, Shape: ShapeValue},
		{Lang: "rust", Symbol: "axum::Json::new", Call: true, Shape: ShapeValue},
		{Lang: "rust", Symbol: "std::fs::read", Call: true, Shape: ShapeValue},
		{Lang: "rust", Symbol: "a type", Call: false},
	}
	c := &Corpus{Name: "c", Lang: "rust", Real: true,
		Calls:    map[string]*CallCount{"axum::Json::new": {Lib: 1}},
		FuncSyms: map[string]bool{}}
	res := CheckRows("rust", rows, []*Corpus{c}, nil)
	want := []string{DeadSuspect, "", DeadUnused, DeadNoCall}
	for i, r := range res {
		if r.Dead != want[i] {
			t.Errorf("%s: %q, want %q", r.Symbol, r.Dead, want[i])
		}
	}
	if len(res[0].Spellings) != 1 || res[0].Spellings[0] != "axum::Json::new" {
		t.Errorf("spellings = %v", res[0].Spellings)
	}
}

func TestCheckRowsMember(t *testing.T) {
	rows := []Row{
		{Lang: "dart", Symbol: "dart:core#List.join", Call: true, Shape: ShapeValue},
		{Lang: "dart", Symbol: "dart:core#List.get", Call: true, Shape: ShapeValue}, // a common name: no signal
	}
	c := &Corpus{Name: "c", Lang: "dart", Real: true,
		Calls:    map[string]*CallCount{"dart:core#Iterable.join": {Lib: 1}, "dart:core#Iterable.get": {Lib: 1}},
		FuncSyms: map[string]bool{}}
	res := CheckRows("dart", rows, []*Corpus{c}, nil)
	if res[0].Dead != DeadMember || len(res[0].Spellings) != 1 {
		t.Errorf("List.join: %+v", res[0])
	}
	if res[1].Dead != DeadUnused {
		t.Errorf("List.get: %q", res[1].Dead)
	}
	supp := &Corpus{Name: "s", Lang: "dart", Supp: true, Calls: map[string]*CallCount{"dart:core#List.join": {Lib: 1}}, FuncSyms: map[string]bool{}}
	if r := CheckRows("dart", rows[:1], []*Corpus{supp}, nil); r[0].HitAll {
		t.Errorf("a supplement counts for the hit of a row")
	}
}

// The types of the standard library: slow, because go/packages loads it.
func TestGoUniverse(t *testing.T) {
	if testing.Short() {
		t.Skip("loads the standard library")
	}
	gu, err := LoadGoUniverse(".", nil)
	if err != nil {
		t.Fatal(err)
	}
	shape := func(sym string) string {
		fn, st := gu.Resolve(sym)
		if st != "found" {
			t.Fatalf("%s: %s", sym, st)
		}
		s, weak := FuncShape(fn)
		if weak {
			s += "?"
		}
		return s
	}
	for sym, want := range map[string]string{
		"strings.Join":                   ShapeValue,
		"encoding/json.Unmarshal":        ShapeWrite, // the second argument is an any
		"(*strings.Builder).WriteString": ShapeWrite, // a pointer receiver
		"sort.Slice":                     ShapeCall,
		"io.ReadFull":                    ShapeWrite + "?",
		"(sync.Locker).Lock":             "", // carries nothing
	} {
		if got := shape(sym); got != want {
			t.Errorf("%s: shape %q, want %q", sym, got, want)
		}
	}
	if _, st := gu.Resolve("strings.NoSuchFunction"); st != "missing" {
		t.Errorf("a name that is not there: %s", st)
	}
	if _, st := gu.Resolve("example.com/nowhere.F"); st != "unloaded" {
		t.Errorf("a package that is not loaded: %s", st)
	}
	if n := len(gu.Enumerate()); n < 1000 {
		t.Errorf("only %d functions listed", n)
	}
}
