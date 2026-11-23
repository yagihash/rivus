package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irspill"
)

// irspillExtract is what "rivus debug extract -all" reads: the IR of the code and of its dependencies.
func irspillExtract(t testing.TB, dir string, patterns ...string) *ir.Program {
	t.Helper()
	prog, err := gossa.Load(dir, patterns, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	out, err := goir.Extract(prog, goir.Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// irspillCheck puts every function of the program through the record and through a store, and holds what comes
// back to what went in.
func irspillCheck(t *testing.T, name string, p *ir.Program) {
	t.Helper()
	if len(p.Funcs) == 0 {
		t.Fatalf("%s: no functions", name)
	}
	s := irspill.New(0, os.Stderr)
	defer s.Close()
	var recBytes, instrs int
	for _, f := range p.Funcs {
		rec := irspill.Encode(nil, f)
		recBytes += len(rec)
		for _, b := range f.Blocks {
			instrs += len(b.Instrs)
		}
		g, n, err := irspill.Decode(rec)
		if err != nil || n != len(rec) {
			t.Fatalf("%s: %s: %v", name, f.ID, err)
		}
		if d := irspill.Diff(f, g); d != "" {
			t.Fatalf("%s: %s: differs at %s", name, f.ID, d)
		}
		s.Put(f)
	}
	for _, f := range p.Funcs {
		g, err := s.Get(f.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d := irspill.Diff(f, g); d != "" {
			t.Fatalf("%s: %s from the store: differs at %s", name, f.ID, d)
		}
	}
	t.Logf("%s: %d functions, %d instructions, %d bytes in records (%.1f a function, %.1f an instruction)", name, len(p.Funcs), instrs, recBytes, float64(recBytes)/float64(len(p.Funcs)), float64(recBytes)/float64(max(instrs, 1)))
}

func irspillRoot(t testing.TB, rel ...string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join(append([]string{"..", ".."}, rel...)...))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestIRSpillRoundTripPrograms: decode(encode(f)) is f, for every function of the programs of the repository: the
// test data, the curated project, the zoo (with the dependencies they read), and the synthetic modules.
func TestIRSpillRoundTripPrograms(t *testing.T) {
	t.Run("testdata", func(t *testing.T) {
		irspillCheck(t, "testdata/go", irspillExtract(t, irspillRoot(t, "testdata", "go"), "./..."))
	})
	t.Run("curated", func(t *testing.T) {
		irspillCheck(t, "bench/go-curated", irspillExtract(t, irspillRoot(t, "bench", "go-curated"), "./..."))
	})
	t.Run("zoo", func(t *testing.T) {
		irspillCheck(t, "bench/zoo-go", irspillExtract(t, irspillRoot(t, "bench", "zoo-go"), "./..."))
	})
	t.Run("synthetic", func(t *testing.T) {
		gen := filepath.Join(t.TempDir(), "gen")
		build := exec.Command("go", "build", "-o", gen, "./bench/go-scale/gen")
		build.Dir = irspillRoot(t)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("building the generator: %v\n%s", err, out)
		}
		dir := t.TempDir()
		if out, err := exec.Command(gen, "-shapes", "all", "-out", dir).CombinedOutput(); err != nil {
			t.Fatalf("gen: %v\n%s", err, out)
		}
		irspillCheck(t, "synthetic all", irspillExtract(t, dir, "./..."))
	})
}
