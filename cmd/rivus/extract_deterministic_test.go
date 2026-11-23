package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/accept"
	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// The IR of debug extract is the same each time it is made: the closures that bind a method ("$bound": go/types.Sizes has them,
// one for each place that takes the method as a value) are several functions of one name, and the candidates of their call were
// all of the implementations in one run and none in the next. Several runs (the order of the load changes too), the same IR.
func TestExtractIsTheSameEachTime(t *testing.T) {
	if testing.Short() {
		t.Skip("reads go/types")
	}
	dir := filepath.Join(runtime.GOROOT(), "src")
	extract := func() *ir.Program {
		p, err := gossa.Load(dir, []string{"go/types"}, extractLoadOptions(false))
		if err != nil {
			t.Fatal(err)
		}
		out, err := goir.Extract(p, goir.Options{All: true})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	base := extract()
	for i, seed := range []string{"", "a", "b", "c", "d", "e"} {
		t.Setenv("RIVUS_INTERNAL_LOAD_ORDER", seed)
		if r := accept.Compare(base, extract()); !r.Empty() {
			t.Fatalf("run %d (load order %q) differs:\\n%s", i, seed, r.Summary(10))
		}
	}
}
