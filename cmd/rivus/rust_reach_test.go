package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// A program of Rust that starts at main reaches what a web framework would run: the impls of a trait that
// the crate re-exports at its root (main names it by the root path, the impls by where it is defined), a
// function item given to a function that does not call it (also made a function pointer), and a closure
// that captured a value (the alloc of its environment, not a make_closure). sg.Program.Reachable and the
// graph (--from-entry) say the same (ADR-051, ADR-055).
func TestRustReachesWhatTheFrameworkRuns(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, _ = filepath.Abs(bin)
	root, _ := filepath.Abs("../../extractor/rust/test/reach")
	cmd := exec.Command(bin, "--no-deps", root)
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v", err)
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	g, err := sg.Build(ir.MemSource(prog))
	if err != nil {
		t.Fatal(err)
	}
	reach := g.Reachable()
	want := map[string]bool{
		"<reach::modal::Web as reach::modal::Server>::init": true,
		"reach::handler":         true,
		"reach::pointer_handler": true,
		"reach::inner_handler":   true,
		"reach::unreferenced":    false,
	}
	for id, w := range want {
		f := g.ByID[id]
		if f == nil {
			t.Fatalf("no function %s", id)
		}
		if reach[f] != w {
			t.Errorf("Reachable has %s: %v, want %v", id, reach[f], w)
		}
	}

	dir := t.TempDir()
	irFile := filepath.Join(dir, "p.jsonl")
	os.WriteFile(irFile, stream, 0o644)
	db := filepath.Join(dir, "g.db")
	rules, _ := filepath.Abs("../../extractor/rust/test/sample/rules.yaml")
	runIn(t, binary, dir, nil, "index", "-ir", irFile, "-rules", rules, "-no-project-rules", "-graph", db)
	for id, w := range want {
		q := graphStage3(t, dir, "path", "-graph", db, "--from-entry", "--to", id)
		if q.Reachable != w {
			t.Errorf("the graph reaches %s from the entry: %v, want %v", id, q.Reachable, w)
		}
	}
}
