package report

import (
	"fmt"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

func TestFunctionsSkippedOfCountsTheProjectAndTheDependenciesApart(t *testing.T) {
	if FunctionsSkippedOf(nil) != nil {
		t.Fatal("nothing skipped: no entry")
	}
	var in []ir.FnSkipped
	for i := 0; i < 5; i++ {
		in = append(in, ir.FnSkipped{Unit: "dep_a", Func: fmt.Sprintf("dep_a::f%d", i), Dependency: true, Why: "async closure"})
	}
	in = append(in, ir.FnSkipped{Unit: "dep_a", Func: "dep_a::g", Dependency: true, Why: "another"})
	in = append(in, ir.FnSkipped{Unit: "mine", Func: "mine::h", Why: "async closure"})
	for i := 0; i < 12; i++ {
		in = append(in, ir.FnSkipped{Unit: fmt.Sprintf("dep_%02d", i), Func: "x", Dependency: true, Why: "w"})
	}
	got := FunctionsSkippedOf(in)
	if got.Functions != 19 || got.Project != 1 || got.Dependencies != 18 {
		t.Errorf("counts: %+v", got)
	}
	if len(got.Crates) != maxSkippedCrates {
		t.Errorf("at most %d crates: %d", maxSkippedCrates, len(got.Crates))
	}
	if c := got.Crates[0]; c.Crate != "mine" || c.Dependency {
		t.Errorf("the project's crate first: %+v", c)
	}
	a := got.Crates[1]
	if a.Crate != "dep_a" || a.Functions != 6 || a.Why != "async closure" || a.WhyOther != 1 || len(a.Examples) != maxSkippedExamples {
		t.Errorf("dep_a: %+v", a)
	}
}
