package goir

import (
	"runtime"
	"strings"
	"testing"
)

// The name of a type that shadows a type alias (`Name·N.M`) is the same in every run, whatever the order the
// files were read in (ADR-118 (ii)): it does not come from token.Pos.
func TestShadowNameIsTheSameInEveryRun(t *testing.T) {
	names := func(procs int) []string {
		old := runtime.GOMAXPROCS(procs)
		defer runtime.GOMAXPROCS(old)
		var out []string
		for _, f := range extract(t, "./cases/typealias").Funcs {
			for _, l := range f.Locals {
				if strings.Contains(string(l.Type), "·") {
					out = append(out, string(f.ID)+" "+string(l.Type))
				}
			}
		}
		return out
	}
	first := names(1)
	if len(first) == 0 {
		t.Fatal("no type that shadows an alias was found in the testdata")
	}
	for _, procs := range []int{8, 1, 8} {
		got := names(procs)
		if strings.Join(got, "\n") != strings.Join(first, "\n") {
			t.Fatalf("GOMAXPROCS=%d: the names differ:\n%v\nand\n%v", procs, first, got)
		}
	}
}
