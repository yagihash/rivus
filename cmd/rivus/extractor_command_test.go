package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The value of an extractor command is split like a shell does for quotes, a path with spaces is one word, and a value
// written the old way (split at white space) means what it did (#200).
func TestSplitCommand(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"rivus-rust", []string{"rivus-rust"}},
		{"  dart   run\tfoo  ", []string{"dart", "run", "foo"}},
		{"dart run bin/x.dart --flag=1", []string{"dart", "run", "bin/x.dart", "--flag=1"}},
		{`"/a b/Application Support/rivus-rust" --x`, []string{"/a b/Application Support/rivus-rust", "--x"}},
		{`'/a b/rivus-rust' -v 'two words'`, []string{"/a b/rivus-rust", "-v", "two words"}},
		{`/a\ b/rivus-rust --x`, []string{"/a b/rivus-rust", "--x"}},
		{`"a \"q\" b" c`, []string{`a "q" b`, "c"}},
		{`pre"fix ed"post`, []string{"prefix edpost"}},
		{`'' x`, []string{"", "x"}},
		{`C:\tmp\x y`, []string{`C:\tmp\x`, "y"}},
		{"", nil},
		{"   ", nil},
	} {
		got, err := splitCommand(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
	for _, bad := range []string{`'abc`, `"abc`, `x "a\"`} {
		if _, err := splitCommand(bad); err == nil {
			t.Errorf("%q: an open quote is not an error", bad)
		}
	}
}

// A path with spaces in it, named without quotes, is one word when the file is there.
func TestSplitCommandExistingPathWithSpaces(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "Application Support", "rivus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "rivus-rust")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := splitCommand(exe)
	if err != nil || !reflect.DeepEqual(got, []string{exe}) {
		t.Fatalf("got %q, %v", got, err)
	}
	// with an argument it is split as before, unless the path is quoted
	got, _ = splitCommand(exe + " --x")
	if len(got) < 3 {
		t.Errorf("an unquoted path with an argument was kept whole: %q", got)
	}
	got, _ = splitCommand(`"` + exe + `" --x`)
	if !reflect.DeepEqual(got, []string{exe, "--x"}) {
		t.Errorf("quoted: %q", got)
	}
}

// -rust-extractor and RIVUS_RUST_EXTRACTOR (and the Dart ones) go through the same split, with a fake executable.
func TestExtractorCommandWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Application Support")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "fake-extractor")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RIVUS_RUST_DATA_DIR", t.TempDir())
	t.Setenv("RIVUS_RUST_EXTRACTOR", "")
	t.Setenv("RIVUS_DART_EXTRACTOR", "")
	for _, c := range []struct {
		name string
		o    analyzeOpts
		env  string
		want []string
	}{
		{"flag, bare path", analyzeOpts{rustExtractor: exe}, "", []string{exe}},
		{"flag, quoted with argument", analyzeOpts{rustExtractor: `"` + exe + `" --a b`}, "", []string{exe, "--a", "b"}},
		{"env, bare path", analyzeOpts{}, exe, []string{exe}},
		{"env, single quotes", analyzeOpts{}, `'` + exe + `' -x`, []string{exe, "-x"}},
	} {
		if c.env != "" {
			t.Setenv("RIVUS_RUST_EXTRACTOR", c.env)
		}
		o := c.o
		got, err := o.extractorCommand(langRust)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
		t.Setenv("RIVUS_RUST_EXTRACTOR", "")
	}
	// the old form still works: words at spaces, the first is found on PATH
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	o := analyzeOpts{rustExtractor: "fake-extractor --a  --b"}
	got, err := o.extractorCommand(langRust)
	if err != nil || !reflect.DeepEqual(got, []string{"fake-extractor", "--a", "--b"}) {
		t.Errorf("old form: %q, %v", got, err)
	}
	o = analyzeOpts{dartExtractor: `"` + exe + `" run`}
	if got, err := o.extractorCommand(langDart); err != nil || !reflect.DeepEqual(got, []string{exe, "run"}) {
		t.Errorf("dart: %q, %v", got, err)
	}
	o = analyzeOpts{rustExtractor: `"unclosed`}
	if _, err := o.extractorCommand(langRust); err == nil || !strings.Contains(err.Error(), "not closed") {
		t.Errorf("open quote: %v", err)
	}
}
