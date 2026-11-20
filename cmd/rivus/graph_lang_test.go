package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The graph file is made from the neutral IR, so it is the same for every language: the IR that the Dart and
// the Rust extractors write for their samples gives a file whose numbers are those of the IR.
func TestGraphFromTheIROfOtherLanguages(t *testing.T) {
	type sample struct {
		name, lang string
		stream     func(t *testing.T) []byte
		rules      string
	}
	extractorDart := func(t *testing.T) []byte {
		dart := os.Getenv("RIVUS_DART")
		if dart == "" {
			t.Skip("RIVUS_DART is not set")
		}
		extractor, _ := filepath.Abs("../../extractor/dart")
		words := strings.Fields(dart)
		cmd := exec.Command(words[0], append(words[1:], "run", "bin/rivus_dart.dart", "test/sample")...)
		cmd.Dir = extractor
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("the extractor failed: %v", err)
		}
		return out
	}
	extractorRust := func(t *testing.T) []byte {
		bin := os.Getenv("RIVUS_RUST")
		if bin == "" {
			t.Skip("RIVUS_RUST is not set")
		}
		bin, _ = filepath.Abs(bin)
		root, _ := filepath.Abs("../../extractor/rust/test/sample")
		out, err := exec.Command(bin, root).Output()
		if err != nil {
			t.Fatalf("the extractor failed: %v", err)
		}
		return out
	}
	dartRules, _ := filepath.Abs("../../extractor/dart/test/sample/rules.yaml")
	rustRules, _ := filepath.Abs("../../extractor/rust/test/sample/rules.yaml")
	for _, s := range []sample{
		{"dart", "dart", extractorDart, dartRules},
		{"rust", "rust", extractorRust, rustRules},
	} {
		t.Run(s.name, func(t *testing.T) {
			stream := s.stream(t)
			dir := t.TempDir()
			irFile := filepath.Join(dir, "p.jsonl")
			os.WriteFile(irFile, stream, 0o644)
			db := filepath.Join(dir, "g.db")
			runIn(t, binary, dir, nil, "index", "-ir", irFile, "-rules", s.rules, "-no-project-rules", "-graph", db)

			prog, err := ir.Read(bytes.NewReader(stream))
			if err != nil {
				t.Fatal(err)
			}
			prog.PruneDeps()
			calls := 0
			for _, f := range prog.Funcs {
				for _, b := range f.Blocks {
					for _, in := range b.Instrs {
						if in.Op == ir.OpCall {
							calls++
						}
					}
				}
			}
			info := graphInfoOf(t, binary, dir, nil, "-graph", db)
			c := info.Graph.Counts
			if c.FuncsWithBody != len(prog.Funcs) || c.CallSites != calls {
				t.Errorf("the file has %d functions and %d calls, the IR has %d and %d", c.FuncsWithBody, c.CallSites, len(prog.Funcs), calls)
			}
			if c.Edges == 0 || c.Sites == 0 {
				t.Errorf("%+v", c)
			}
			if len(info.Graph.Languages) != 1 || info.Graph.Languages[0] != s.lang {
				t.Errorf("languages: %v", info.Graph.Languages)
			}
			// the sites are the sites of "rivus sites"
			sites := runIn(t, binary, dir, nil, "sites", "-ir", irFile, "-rules", s.rules, "-no-project-rules")
			if n := strings.Count(string(sites), `"id": "s1:`); n != c.Sites {
				t.Errorf("the file has %d sites, rivus sites lists %d", c.Sites, n)
			}
			// the key is the IR stream: -C says it is to be held against it
			if f := graphInfoOf(t, binary, dir, nil, "-graph", db, "-C", dir, "-ir", irFile, "-rules", s.rules, "-no-project-rules"); f.Snapshot.Fresh == nil || !*f.Snapshot.Fresh {
				t.Errorf("fresh: %v", f.Snapshot.Reasons)
			}
			os.WriteFile(irFile, append(stream, '\n'), 0o644)
			if f := graphInfoOf(t, binary, dir, nil, "-graph", db, "-C", dir, "-ir", irFile, "-rules", s.rules, "-no-project-rules"); *f.Snapshot.Fresh {
				t.Errorf("the stream changed")
			}
		})
	}
}

// An extractor does not say which files it read, so for the languages that have one the whole tree is the key
// (without the places tools put what they make), with the lock files, the files the IR names outside the
// tree, and the versions of the tools.
func TestGraphKeyOfLanguagesWhoseExtractorDoesNotSayWhatItRead(t *testing.T) {
	outside := t.TempDir() // a package cache
	for _, c := range []struct {
		lang, marker string
		lock         string
		hidden       string // a directory that is read
		built        string // a directory that holds what a build made
	}{
		{"dart", "pubspec.yaml", "pubspec.lock", filepath.Join("lib", ".generated", "g.dart"), filepath.Join("build", "out.txt")},
		{"rust", "Cargo.toml", "Cargo.lock", filepath.Join("src", ".gen", "g.rs"), filepath.Join("target", "debug", "x")},
	} {
		t.Run(c.lang, func(t *testing.T) {
			proj := t.TempDir()
			write(t, filepath.Join(proj, c.marker), "name: app\n")
			write(t, filepath.Join(proj, c.lock), "# lock\n")
			write(t, filepath.Join(proj, "lib", "a.src"), "a\n")
			write(t, filepath.Join(proj, c.hidden), "g\n")
			write(t, filepath.Join(proj, c.built), "o\n")
			depFile := filepath.Join(outside, c.lang+"-dep.src")
			write(t, depFile, "dep\n")
			// the IR the fake extractor writes: a function of the project that calls one of a dependency
			prog := &ir.Program{Funcs: []*ir.Func{
				{ID: ir.FuncID(map[string]string{"dart": "package:app/a.dart#main", "rust": "app::main"}[c.lang]), Entry: true, File: "lib/a.src", Pos: ir.Pos{File: "lib/a.src", Line: 1, Col: 1},
					Blocks: []ir.Block{{ID: 0, Term: ir.Term{Kind: ir.TermReturn}}}},
				{ID: ir.FuncID(map[string]string{"dart": "package:dep/d.dart#f", "rust": "dep::f"}[c.lang]), Dep: true, File: depFile, Pos: ir.Pos{File: depFile, Line: 1, Col: 1},
					Blocks: []ir.Block{{ID: 0, Term: ir.Term{Kind: ir.TermReturn}}}},
			}}
			var buf bytes.Buffer
			if err := ir.Write(&buf, prog); err != nil {
				t.Fatal(err)
			}
			stream := filepath.Join(t.TempDir(), "stream.jsonl")
			os.WriteFile(stream, buf.Bytes(), 0o644)
			ext := filepath.Join(t.TempDir(), "fake-extractor")
			script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo fake 1; exit 0; fi\ncat " + stream + "\n"
			write(t, ext, script)
			os.Chmod(ext, 0o755)
			env := []string{"RIVUS_" + strings.ToUpper(c.lang) + "_EXTRACTOR=" + ext}
			db := filepath.Join(t.TempDir(), "g.db")
			args := []string{"-lang", c.lang, "-graph", db, "-C", proj, "-no-project-rules"}
			runIn(t, binary, proj, env, append([]string{"index"}, args...)...)
			check := func() infoOut {
				return graphInfoOf(t, binary, proj, env, "-lang", c.lang, "-graph", db, "-C", proj, "-no-project-rules")
			}

			if info := check(); info.Snapshot.Fresh == nil || !*info.Snapshot.Fresh {
				t.Fatalf("fresh: %v", info.Snapshot.Reasons)
			}
			if info := check(); info.Graph.Counts.FuncsWithBody != 1 /* the dependency that nothing reaches is dropped */ {
				t.Errorf("%+v", info.Graph.Counts)
			}
			// what a build made is not an input
			write(t, filepath.Join(proj, c.built), "changed\n")
			if info := check(); !*info.Snapshot.Fresh {
				t.Errorf("the output of a build is left out: %v", info.Snapshot.Reasons)
			}
			for _, ch := range []struct {
				name string
				do   func() func()
			}{
				{"a source file of the project", edit(t, filepath.Join(proj, "lib", "a.src"), func(s string) string { return s + "x" })},
				{"a hidden directory that is read", edit(t, filepath.Join(proj, c.hidden), func(s string) string { return s + "x" })},
				{"the lock file", edit(t, filepath.Join(proj, c.lock), func(s string) string { return s + "x" })},
				{"a new file", create(t, filepath.Join(proj, "lib", "b.src"), "b")},
				{"a file of a dependency outside the tree, named by the IR", edit(t, depFile, func(s string) string { return s + "x" })},
				{"the extractor", edit(t, ext, func(s string) string { return s + "# x\n" })},
			} {
				undo := ch.do()
				if info := check(); *info.Snapshot.Fresh {
					t.Errorf("%s: not stale", ch.name)
				}
				undo()
			}
			if c.lang == "dart" {
				// the packages are where package_config.json says
				write(t, filepath.Join(proj, ".dart_tool", "package_config.json"), "{}")
				if info := check(); *info.Snapshot.Fresh {
					t.Errorf("package_config.json appeared: not stale")
				}
			}
		})
	}
}
