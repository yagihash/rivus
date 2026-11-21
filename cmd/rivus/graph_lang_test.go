package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphdb"
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
			skipOrFail(t, "RIVUS_DART")
		}
		ensureDartSamplePub(t, dart)
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
			skipOrFail(t, "RIVUS_RUST")
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
			// the references: a function named as a value (a tear-off of Dart, a function pointer of Rust) is in the
			// refs table, as many as the IR has of a function value (or a closure that captured something) in an instruction
			want := 0
			for _, f := range prog.Funcs {
				for _, b := range f.Blocks {
					for _, in := range b.Instrs {
						if in.Op == ir.OpMakeClosure && len(in.Args) > 0 {
							want++
						}
					}
				}
			}
			if c.Refs == 0 || c.Refs < want/2 {
				t.Errorf("the file has %d references, the IR has %d closures that captured a value, and function values besides", c.Refs, want)
			}
			t.Logf("%s: %d references (%d closures that captured a value)", s.lang, c.Refs, want)
			if len(info.Graph.Languages) != 1 || info.Graph.Languages[0] != s.lang {
				t.Errorf("languages: %v", info.Graph.Languages)
			}
			// callers and path read the same file: a function that has a caller, and the way from it
			d, err := graphdb.Open(db)
			if err != nil {
				t.Fatal(err)
			}
			callee, err := d.QueryForTest(`SELECT f.fid, 0, 0 FROM edges e JOIN funcs f ON f.id = e.to_func WHERE e.kind = 'static' AND f.has_body = 1 AND e.from_func != e.to_func ORDER BY f.fid LIMIT 1`)
			if err != nil {
				t.Fatal(err)
			}
			caller, err := d.QueryForTest(`SELECT f.fid, 0, 0 FROM edges e JOIN funcs f ON f.id = e.from_func JOIN funcs t ON t.id = e.to_func WHERE t.fid = '` + strings.ReplaceAll(callee.File, "'", "''") + `' AND e.from_func != e.to_func ORDER BY f.fid LIMIT 1`)
			d.Close()
			if err != nil {
				t.Fatal(err)
			}
			cq := graphQuery(t, dir, "callers", "-graph", db, callee.File)
			found := false
			for _, e := range cq.Edges {
				found = found || e.From == caller.File
			}
			if !found {
				t.Errorf("callers of %s: %+v", callee.File, cq.Edges)
			}
			pq := graphQuery(t, dir, "path", "-graph", db, "--from", caller.File, "--to", callee.File)
			if !pq.Reachable || len(pq.Paths) == 0 || pq.Paths[0].Steps[0].Func != caller.File {
				t.Errorf("path %s -> %s: %+v", caller.File, callee.File, pq)
			}
			// from the entry points to a function that one of them calls (neither sample has a main, so the
			// entries are the exported functions; Dart's variable initializers are entries that do not start a program)
			start := `exported = 1`
			d2, err := graphdb.Open(db)
			if err != nil {
				t.Fatal(err)
			}
			reached, err := d2.QueryForTest(`SELECT t.fid, 0, 0 FROM edges e JOIN funcs f ON f.id = e.from_func JOIN funcs t ON t.id = e.to_func WHERE f.` + start + ` AND f.dep = 0 AND e.kind = 'static' AND t.has_body = 1 AND t.id != f.id ORDER BY t.fid LIMIT 1`)
			d2.Close()
			if err != nil {
				t.Fatalf("no function that an entry calls: %v", err)
			}
			en := graphStage3(t, dir, "path", "-graph", db, "--from-entry", "--to", reached.File)
			wantSource := "exported"
			if !en.Reachable || en.Entry == nil || en.Entry.Source != wantSource || en.Entry.Count == 0 || len(en.Paths) == 0 {
				t.Errorf("from the entry to %s: %+v", reached.File, en)
			}
			// the filters, and a short name of the language's own form
			if f := graphStage3(t, dir, "path", "-graph", db, "--from", caller.File, "--to", callee.File, "--avoid", caller.File); f.Reachable || len(f.Filters) != 1 {
				t.Errorf("avoid the start: %+v", f)
			}
			short := callee.File[strings.LastIndexAny(callee.File, "#:")+1:]
			if sh := graphStage3(t, dir, "callers", "-graph", db, short); len(sh.Ambiguous) == 0 && !strings.Contains(sh.Query.Resolved, "function") {
				t.Errorf("%s: %+v", short, sh)
			}
			// impls and dispatchers, from the interface method (a trait of Rust, the implicit interface of a Dart class)
			// of a call in the code itself, and back from the implementation it reaches
			d3, err := graphdb.Open(db)
			if err != nil {
				t.Fatal(err)
			}
			im, err := d3.QueryForTest(`SELECT c.callee, 0, 0 FROM callsites c JOIN funcs f ON f.id = c.func_id JOIN edges e ON e.callsite_id = c.id JOIN funcs t ON t.id = e.to_func
				WHERE c.kind = 'interface' AND f.dep = 0 AND t.has_body = 1 AND t.dep = 0 AND c.callee IS NOT NULL ORDER BY c.callee, t.fid LIMIT 1`)
			var impl string
			if err == nil {
				var r graphdb.Position
				r, err = d3.QueryForTest(`SELECT t.fid, 0, 0 FROM callsites c JOIN funcs f ON f.id = c.func_id JOIN edges e ON e.callsite_id = c.id JOIN funcs t ON t.id = e.to_func
					WHERE c.kind = 'interface' AND f.dep = 0 AND t.has_body = 1 AND t.dep = 0 AND c.callee = '` + strings.ReplaceAll(im.File, "'", "''") + `' ORDER BY t.fid LIMIT 1`)
				impl = r.File
			}
			d3.Close()
			if err != nil {
				t.Fatalf("no interface method that has an implementation: %v", err)
			}
			ii := graphImpls(t, dir, "impls", "-graph", db, im.File)
			if ii.Method != im.File || len(ii.Impls) == 0 || ii.CallSites == 0 || !strings.Contains(ii.Basis, "not every type") {
				t.Errorf("impls %s: %+v", im.File, ii)
			}
			listed := false
			for _, x := range ii.Impls {
				listed = listed || x.ID == impl
			}
			if !listed {
				t.Errorf("impls %s does not list %s: %+v", im.File, impl, ii.Impls)
			}
			dd := graphDispatch(t, dir, "dispatchers", "-graph", db, impl)
			if dd.CallSitesTotal == 0 || len(dd.CallSites) == 0 || dd.CallSites[0].At == "" || dd.CallSites[0].In == "" {
				t.Fatalf("dispatchers %s: %+v", impl, dd)
			}
			seen := false
			for _, x := range dd.CallSites {
				seen = seen || x.Callee == im.File
			}
			if !seen {
				t.Errorf("dispatchers %s has no call through %s: %+v", impl, im.File, dd.CallSites)
			}
			// a call by its position, for the language's own file names
			if len(cq.Edges) > 0 {
				if at := graphQuery(t, dir, "callees", "-graph", db, cq.Edges[0].At); len(at.Start) == 0 {
					t.Errorf("callees %s: %+v", cq.Edges[0].At, at)
				}
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
			env := []string{"RIVUS_" + strings.ToUpper(c.lang) + "_EXTRACTOR=" + ext, "RIVUS_UNSAFE_HOST_EXEC=1" /* a script of the test, not a project that is read */}
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
