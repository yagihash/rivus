package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRustRows(t *testing.T) {
	rows, err := LoadRustRows([]string{filepath.Join("testdata", "rust", "rows.yaml")}, "actix-web")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows.Probed {
		got = append(got, r.Kind+" "+r.Symbol)
	}
	want := []string{"type &actix_web::HttpRequest", "type actix_web::web::Json", "symbol actix_web::HttpRequest::path", "symbol actix_web::request::HttpRequest::uri"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("probed %q, want %q", got, want)
	}
	if rows.Other != 1 {
		t.Errorf("rows of other crates = %d, want 1", rows.Other)
	}
	if len(rows.Skipped) != 2 {
		t.Errorf("skipped = %q, want the field row and the generic one", rows.Skipped)
	}
	if len(rows.Versions) != 1 || rows.Versions[0].CheckedWith != "4.20.0" {
		t.Errorf("versions = %+v", rows.Versions)
	}
}

func TestRawIdentifierRow(t *testing.T) {
	if !plainPathRe.MatchString("octocrab::repos::GetContentBuilder::r#ref") {
		t.Error("a raw identifier is a plain path")
	}
	if Judge(CompileError{Code: "E0603", Msg: "module `file` is private"}) != Resolves {
		t.Error("a private module on the path resolves, it is not absent")
	}
}

func TestProbeLine(t *testing.T) {
	cases := []struct {
		row     RustRow
		variant int
		want    string
		ok      bool
	}{
		{RustRow{Kind: "symbol", Symbol: "actix_web::HttpRequest::path"}, 0, "pub fn p1<A, B, C>() { let f = actix_web::HttpRequest::path; std::hint::black_box(f); }", true},
		{RustRow{Kind: "symbol", Symbol: "actix_web::web::Query::into_inner"}, 2, "pub fn p1<A, B, C>() { let f = actix_web::web::Query::<A, B>::into_inner; std::hint::black_box(f); }", true},
		{RustRow{Kind: "symbol", Symbol: "actix_web::web::block"}, 1, "", false},
		{RustRow{Kind: "type", Symbol: "&actix_web::HttpRequest"}, 0, "pub fn t1<A, B, C>(_x: &actix_web::HttpRequest) {}", true},
		{RustRow{Kind: "type", Symbol: "actix_web::web::Json"}, 1, "pub fn t1<A, B, C>(_x: actix_web::web::Json<A>) {}", true},
	}
	for _, c := range cases {
		got, ok := ProbeLine(1, c.row, c.variant)
		if got != c.want || ok != c.ok {
			t.Errorf("ProbeLine(%+v, %d) = %q, %v; want %q, %v", c.row, c.variant, got, ok, c.want, c.ok)
		}
	}
}

func TestCompileErrors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "rust", "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	errs := ParseCompileErrors(string(data))
	if len(errs) != 4 {
		t.Fatalf("errors = %+v, want 4 (the warning and the Cargo.toml location are not probes)", errs)
	}
	want := []struct {
		line int
		v    Verdict
	}{{3, Absent}, {5, Resolves}, {6, Resolves}, {7, NextVariant}}
	for i, w := range want {
		if errs[i].Line != w.line || Judge(errs[i]) != w.v {
			t.Errorf("error %d = %+v (verdict %v), want line %d verdict %v", i, errs[i], Judge(errs[i]), w.line, w.v)
		}
	}
}

// the canned IR: rows against what the extractor wrote.
func TestMatchRows(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "rust", "probe-ir.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := ReadIR(data, "rivus_spelling_probe", "actix_web")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Defs) != 3 {
		t.Errorf("definition ids = %v, want the 3 of the crate", facts.Defs)
	}
	cases := []struct {
		row   RustRow
		probe string
		want  Status
		wrote string
	}{
		{RustRow{Kind: "symbol", Symbol: "actix_web::HttpRequest::path"}, "p0", Found, ""},
		{RustRow{Kind: "symbol", Symbol: "actix_web::request::HttpRequest::uri"}, "p1", Found, ""},
		{RustRow{Kind: "symbol", Symbol: "actix_web::HttpRequest::uri"}, "p1", Spelled, "actix_web::request::HttpRequest::uri"},
		{RustRow{Kind: "symbol", Symbol: "actix_web::web::Query::into_inner"}, "p2", Found, ""},
		{RustRow{Kind: "type", Symbol: "&actix_web::HttpRequest"}, "t3", Found, ""},
		{RustRow{Kind: "type", Symbol: "actix_web::web::Query"}, "t4", Found, ""},
		{RustRow{Kind: "type", Symbol: "actix_web::web::Json"}, "t5", Spelled, "actix_web::web::types::json::Json"},
		{RustRow{Kind: "symbol", Symbol: "actix_web::HttpRequest::gone"}, "p9", NotFound, ""},
	}
	for _, c := range cases {
		got, wrote := MatchProbe(c.row, facts.Probe[c.probe])
		if got != c.want || wrote != c.wrote {
			t.Errorf("%s: %s, %q; want %s, %q", c.row.Symbol, got, wrote, c.want, c.wrote)
		}
	}
	// the closest candidate of a row that the extractor spells otherwise: the definition path with the same last two segments
	if c := Closest(facts.Defs, "actix_web::HttpRequest::uri"); len(c) != 1 || c[0] != "actix_web::request::HttpRequest::uri" {
		t.Errorf("closest = %v", c)
	}
}

func TestStripGenerics(t *testing.T) {
	for in, want := range map[string]string{
		"actix_web::web::Query<A>":                 "actix_web::web::Query",
		"&actix_web::web::Query<Vec<A>, B>::get":   "&actix_web::web::Query::get",
		"std::hint::black_box::<fn(&A) -> Vec<B>>": "std::hint::black_box",
		"a::b": "a::b",
	} {
		if got := StripGenerics(in); got != want {
			t.Errorf("StripGenerics(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProbeManifest(t *testing.T) {
	got := ProbeManifest("actix-web", "4.20.0", "macros, cookies", true)
	for _, w := range []string{`actix-web = { version = "=4.20.0", default-features = false, features = ["macros", "cookies"] }`, "[workspace]"} {
		if !strings.Contains(got, w) {
			t.Errorf("manifest lacks %q:\n%s", w, got)
		}
	}
}

func TestSkipReason(t *testing.T) {
	if r := SkipReason(""); r == "" {
		t.Error("no extractor must be a skip")
	}
}
