package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The alloc of the environment of a closure says the function of the closure (Instr.Func): the name of the closure that the
// extractor lowers, so it is the ID of a function of the same stream. A closure that captures nothing and one that captured
// a value are told so, and the two closures that a macro writes at one place have two IDs (the type of their environments is one:
// it names the place, and cannot tell them apart).
func TestRustEnvAllocSaysItsClosure(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, _ = filepath.Abs(bin)
	root, _ := filepath.Abs("../../extractor/rust/test/closure_id")
	stream, err := exec.Command(bin, "--no-deps", root).Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v", err)
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	have := map[ir.FuncID]bool{}
	for _, f := range prog.Funcs {
		have[f.ID] = true
	}
	envs := func(id string) (funcs []ir.FuncID, types map[ir.TypeID]bool) {
		types = map[ir.TypeID]bool{}
		for _, f := range prog.Funcs {
			if string(f.ID) != id {
				continue
			}
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op == ir.OpAlloc && ir.IsEnvType(string(in.Type)) {
						funcs = append(funcs, in.Func)
						types[in.Type] = true
					}
				}
			}
		}
		return
	}
	for _, id := range []string{"closure_id::nothing_captured", "closure_id::captured", "closure_id::macro_twice"} {
		funcs, _ := envs(id)
		want := 1
		if id == "closure_id::macro_twice" {
			want = 2
		}
		if len(funcs) != want {
			t.Errorf("%s: %d allocs of an environment, want %d", id, len(funcs), want)
			continue
		}
		for _, fn := range funcs {
			if fn == "" || !have[fn] || !ir.IsClosureID(string(fn)) {
				t.Errorf("%s: an alloc of an environment says the function %q, which the stream has: %v", id, fn, have[fn])
			}
		}
	}
	funcs, types := envs("closure_id::macro_twice")
	if len(funcs) == 2 && funcs[0] == funcs[1] {
		t.Errorf("the two closures of the macro have one ID: %q", funcs[0])
	}
	if len(types) != 1 {
		t.Errorf("the two closures of the macro have %d types of environment, want one (the place): %v", len(types), types)
	}
}
