package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

func writeIR(t *testing.T, dir, name string, exported bool) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fn := func(id string, exported bool) *ir.Func {
		return &ir.Func{ID: ir.FuncID(id), Symbol: id, Exported: exported, Blocks: []ir.Block{{ID: 0, Term: ir.Term{Kind: ir.TermReturn}}}, Roots: []ir.BlockID{0}}
	}
	prog := &ir.Program{Funcs: []*ir.Func{fn("a.F", exported), fn("a.G", false)}}
	if err := ir.Write(f, prog); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIRSameAndDifferent(t *testing.T) {
	d := t.TempDir()
	a, b, c := writeIR(t, d, "a", true), writeIR(t, d, "b", true), writeIR(t, d, "c", false)
	if out, same, err := compareIR(a, b); err != nil || !same || !strings.HasPrefix(out, "IR: same (2 functions)") {
		t.Fatalf("same IR: %q %v %v", out, same, err)
	}
	out, same, err := compareIR(a, c)
	if err != nil || same || !strings.Contains(out, "IR: DIFFERENT") || !strings.Contains(out, "a.F") {
		t.Fatalf("different IR: %q %v %v", out, same, err)
	}
}

func writeLines(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSummariesSameAndDifferent(t *testing.T) {
	d := t.TempDir()
	a := writeLines(t, d, "a", "f\telems=1\tsteps=2\th1", "g\telems=0\tsteps=0\th2")
	b := writeLines(t, d, "b", "g\telems=0\tsteps=0\th2", "f\telems=1\tsteps=2\th1") // the order of the lines does not count
	c := writeLines(t, d, "c", "f\telems=1\tsteps=2\thX", "h\telems=0\tsteps=0\th3")
	if out, same, err := compareLines("summaries", "functions", a, b, summaryKey); err != nil || !same || !strings.HasPrefix(out, "summaries: same (2 functions)") {
		t.Fatalf("same: %q %v %v", out, same, err)
	}
	out, same, err := compareLines("summaries", "functions", a, c, summaryKey)
	if err != nil || same {
		t.Fatalf("different: %q %v %v", out, same, err)
	}
	for _, want := range []string{"only in the first: 1, only in the second: 1, with a different summary: 1", "different: f", "only in the first: g", "only in the second: h"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is not in %q", want, out)
		}
	}
}

func TestMultisetDiff(t *testing.T) {
	onlyA, onlyB := multisetDiff([]string{"a", "b", "b", "c"}, []string{"a", "b", "d"})
	if strings.Join(onlyA, ",") != "b,c" || strings.Join(onlyB, ",") != "d" {
		t.Fatalf("%v %v", onlyA, onlyB)
	}
}
